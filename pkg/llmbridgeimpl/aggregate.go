package llmbridgeimpl

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"

	"picotera/pkg/llmbridge"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
)

func AggregateStream(ctx context.Context, format llmbridge.Format, contentType string, body []byte, profile llmbridge.OutboundProfile) ([]byte, error) {
	kind := llmbridge.StreamAggregationKind(format, contentType)
	var chunks []*httpclient.StreamEvent
	var err error
	switch kind {
	case llmbridge.StreamAggregationSSE:
		chunks, err = decodeSSEStream(ctx, body)
	case llmbridge.StreamAggregationJSONL:
		chunks, err = decodeJSONLStream(body)
	case llmbridge.StreamAggregationNone:
		return nil, fmt.Errorf("llmbridge: aggregate stream: %s with content type %q is not a stream response", format, normalizedContentType(contentType))
	case llmbridge.StreamAggregationUnsupported:
		return nil, fmt.Errorf("llmbridge: aggregate stream: unsupported stream content type %q for %s", normalizedContentType(contentType), format)
	default:
		return nil, fmt.Errorf("llmbridge: aggregate stream: unknown aggregation kind %d", kind)
	}
	if err != nil {
		return nil, err
	}
	if len(chunks) == 0 {
		return nil, fmt.Errorf("llmbridge: aggregate stream: empty stream chunks")
	}

	out, err := outboundFor(format, profile)
	if err != nil {
		return nil, err
	}
	req := &httpclient.Request{
		Method:      http.MethodPost,
		Headers:     http.Header{"Content-Type": []string{"application/json"}},
		RequestType: string(llm.RequestTypeChat),
	}
	resp, _, err := out.AggregateStreamChunks(ctx, req, chunks)
	if err != nil {
		return nil, fmt.Errorf("llmbridge: aggregate stream: %w", err)
	}
	if !json.Valid(resp) {
		return nil, fmt.Errorf("llmbridge: aggregate stream: transformer returned invalid JSON")
	}
	return resp, nil
}

func decodeSSEStream(ctx context.Context, body []byte) ([]*httpclient.StreamEvent, error) {
	stream := httpclient.NewDefaultSSEDecoder(ctx, io.NopCloser(bytes.NewReader(body)))
	defer stream.Close()
	var chunks []*httpclient.StreamEvent
	for stream.Next() {
		if ev := stream.Current(); ev != nil {
			chunks = append(chunks, ev)
		}
	}
	if err := stream.Err(); err != nil && !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("llmbridge: decode sse stream: %w", err)
	}
	return chunks, nil
}

func decodeJSONLStream(body []byte) ([]*httpclient.StreamEvent, error) {
	reader := bufio.NewReader(bytes.NewReader(body))
	var chunks []*httpclient.StreamEvent
	lineNo := 0
	for {
		line, err := reader.ReadBytes('\n')
		if len(line) > 0 {
			lineNo++
			line = bytes.TrimSuffix(line, []byte{'\n'})
			line = bytes.TrimSuffix(line, []byte{'\r'})
			if len(line) > 0 {
				trimmed := bytes.TrimSpace(line)
				if len(trimmed) == 0 || trimmed[0] != '{' {
					return nil, fmt.Errorf("llmbridge: decode jsonl stream line %d: expected JSON object", lineNo)
				}
				var obj map[string]json.RawMessage
				if err := json.Unmarshal(line, &obj); err != nil {
					return nil, fmt.Errorf("llmbridge: decode jsonl stream line %d: expected JSON object: %w", lineNo, err)
				}
				if obj == nil {
					return nil, fmt.Errorf("llmbridge: decode jsonl stream line %d: expected JSON object", lineNo)
				}
				chunks = append(chunks, &httpclient.StreamEvent{Data: line})
			}
		}
		if err == nil {
			continue
		}
		if errors.Is(err, io.EOF) {
			break
		}
		return nil, fmt.Errorf("llmbridge: decode jsonl stream: %w", err)
	}
	return chunks, nil
}
