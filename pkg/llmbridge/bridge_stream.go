package llmbridge

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/streams"
)

// BridgeStream wraps the upstream SSE byte stream and returns a reader that
// emits source-format SSE bytes. When src == upstream the call is identity:
// the upstream reader is returned as-is, byte-for-byte preserving the
// original wire format (important so streamSuccess in the gateway can keep
// passing raw bytes to the client and the response extractor).
//
// On bridge: upstream bytes → axonhub SSE decoder → upstream Outbound →
// llm.Response stream → source Inbound → axonhub StreamEvent stream → SSE
// bytes written into a pipe whose read end is returned.
//
// The returned reader is a ReadCloser; closing it stops the conversion
// goroutine and closes the underlying upstream reader.
func BridgeStream(ctx context.Context, src, upstream Format, upstreamBody io.ReadCloser, upstreamCT string) (io.ReadCloser, error) {
	if src == FormatUnknown || upstream == FormatUnknown {
		_ = upstreamBody.Close()
		return nil, fmt.Errorf("llmbridge: bridge stream with unknown format (src=%s upstream=%s)", src, upstream)
	}
	if src == upstream {
		return upstreamBody, nil
	}

	in, err := inboundFor(src)
	if err != nil {
		_ = upstreamBody.Close()
		return nil, err
	}
	out, err := outboundFor(upstream)
	if err != nil {
		_ = upstreamBody.Close()
		return nil, err
	}

	decoderFactory, ok := httpclient.GetDecoder(normalizedContentType(upstreamCT))
	if !ok {
		decoderFactory = httpclient.NewDefaultSSEDecoder
	}
	rawStream := decoderFactory(ctx, upstreamBody)

	// Feed the upstream Outbound a synthetic httpclient.Request that names
	// the stream type. axonhub's Outbound.TransformStream uses req.RequestType
	// only as a routing key for composite transformers (e.g. DeepSeek);
	// passing "chat" is correct for all four formats we support here.
	feedReq := &httpclient.Request{
		Method:      http.MethodPost,
		Headers:     http.Header{"Content-Type": []string{"application/json"}},
		RequestType: string(llm.RequestTypeChat),
	}
	llmStream, err := out.TransformStream(ctx, feedReq, rawStream)
	if err != nil {
		_ = upstreamBody.Close()
		return nil, fmt.Errorf("llmbridge: open %s stream: %w", upstream, err)
	}
	clientEvents, err := in.TransformStream(ctx, llmStream)
	if err != nil {
		_ = upstreamBody.Close()
		return nil, fmt.Errorf("llmbridge: open %s stream: %w", src, err)
	}

	pr, pw := io.Pipe()
	go func() {
		err := pumpEvents(clientEvents, pw)
		_ = clientEvents.Close()
		// closing the underlying upstream reader is the decoder's job;
		// guard against double-close by relying on its idempotent Close.
		_ = upstreamBody.Close()
		_ = pw.CloseWithError(err)
	}()
	return pr, nil
}

// pumpEvents drains a stream of *httpclient.StreamEvent into the writer in
// SSE wire format. Standard SSE: optional `event: <type>` line, then `data:
// <data>` lines, then a blank line terminator. We follow each event's Type
// (Anthropic populates it, OpenAI/Gemini leave it empty) and split multi-line
// payloads onto multiple `data:` lines per RFC 8895 conventions.
func pumpEvents(stream streams.Stream[*httpclient.StreamEvent], w io.Writer) error {
	for stream.Next() {
		ev := stream.Current()
		if ev == nil {
			continue
		}
		buf := encodeSSEEvent(ev)
		if _, err := w.Write(buf); err != nil {
			return err
		}
	}
	if err := stream.Err(); err != nil && !errors.Is(err, io.EOF) {
		return err
	}
	return nil
}

// encodeSSEEvent serializes a StreamEvent to SSE wire bytes. Multi-line Data
// is split into one `data:` line per source line so well-behaved consumers
// reassemble it correctly.
func encodeSSEEvent(ev *httpclient.StreamEvent) []byte {
	var buf bytes.Buffer
	if ev.Type != "" {
		buf.WriteString("event: ")
		buf.WriteString(ev.Type)
		buf.WriteByte('\n')
	}
	data := ev.Data
	if len(data) == 0 {
		buf.WriteString("data:\n")
	} else {
		for _, line := range bytes.Split(data, []byte{'\n'}) {
			buf.WriteString("data: ")
			buf.Write(line)
			buf.WriteByte('\n')
		}
	}
	buf.WriteString("\n")
	return buf.Bytes()
}

func normalizedContentType(ct string) string {
	if i := strings.IndexByte(ct, ';'); i >= 0 {
		ct = ct[:i]
	}
	return strings.ToLower(strings.TrimSpace(ct))
}

// teeReadCloser tees every byte read from the underlying ReadCloser into the
// supplied bytes.Buffer. It is used by the unified gateway handler to keep
// the upstream wire bytes for the upstream-view artifact while the bridged
// (source-format) bytes drain to the client and the meta-view artifact.
type teeReadCloser struct {
	src  io.ReadCloser
	tee  *bytes.Buffer
	mu   sync.Mutex
	done bool
}

// NewUpstreamTee returns a ReadCloser that mirrors src into tee on every
// successful Read. Closing the returned reader closes src (so the bridge's
// downstream consumer drains naturally).
func NewUpstreamTee(src io.ReadCloser, tee *bytes.Buffer) io.ReadCloser {
	return &teeReadCloser{src: src, tee: tee}
}

func (t *teeReadCloser) Read(p []byte) (int, error) {
	n, err := t.src.Read(p)
	if n > 0 && t.tee != nil {
		t.mu.Lock()
		t.tee.Write(p[:n])
		t.mu.Unlock()
	}
	return n, err
}

func (t *teeReadCloser) Close() error {
	t.mu.Lock()
	if t.done {
		t.mu.Unlock()
		return nil
	}
	t.done = true
	t.mu.Unlock()
	return t.src.Close()
}
