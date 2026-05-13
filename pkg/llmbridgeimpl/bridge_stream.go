package llmbridgeimpl

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"picotera/pkg/llmbridge"

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
type StreamBridge interface {
	Pump(ctx context.Context, w io.Writer) error
	Close() error
}

type streamBridge struct {
	stream streams.Stream[*httpclient.StreamEvent]
}

func OpenStream(ctx context.Context, src, upstream llmbridge.Format, upstreamBody io.ReadCloser, upstreamCT string, profile llmbridge.OutboundProfile) (StreamBridge, error) {
	if src == llmbridge.FormatUnknown || upstream == llmbridge.FormatUnknown {
		_ = upstreamBody.Close()
		return nil, fmt.Errorf("llmbridge: bridge stream with unknown format (src=%s upstream=%s)", src, upstream)
	}
	if src == upstream {
		_ = upstreamBody.Close()
		return nil, fmt.Errorf("llmbridge: open stream called for identity formats")
	}

	in, err := inboundFor(src)
	if err != nil {
		_ = upstreamBody.Close()
		return nil, err
	}
	out, err := outboundFor(upstream, profile)
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
	return &streamBridge{stream: clientEvents}, nil
}

func BridgeStream(ctx context.Context, src, upstream llmbridge.Format, upstreamBody io.ReadCloser, upstreamCT string, profile llmbridge.OutboundProfile) (io.ReadCloser, error) {
	if src == llmbridge.FormatUnknown || upstream == llmbridge.FormatUnknown {
		_ = upstreamBody.Close()
		return nil, fmt.Errorf("llmbridge: bridge stream with unknown format (src=%s upstream=%s)", src, upstream)
	}
	if src == upstream {
		return upstreamBody, nil
	}
	stream, err := OpenStream(ctx, src, upstream, upstreamBody, upstreamCT, profile)
	if err != nil {
		return nil, err
	}
	pr, pw := io.Pipe()
	go func() {
		err := stream.Pump(ctx, pw)
		_ = stream.Close()
		_ = pw.CloseWithError(err)
	}()
	return pr, nil
}

func (s *streamBridge) Pump(ctx context.Context, w io.Writer) error {
	for s.stream.Next() {
		ev := s.stream.Current()
		if ev == nil {
			continue
		}
		buf := encodeSSEEvent(ev)
		if _, err := w.Write(buf); err != nil {
			return err
		}
	}
	if err := s.stream.Err(); err != nil && !errors.Is(err, io.EOF) {
		return err
	}
	return nil
}

func (s *streamBridge) Close() error {
	return s.stream.Close()
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
