package server

import (
	"io"
	"strings"
	"testing"
	"time"
)

// chunkReader delivers pre-defined chunks sequentially.
type chunkReader struct {
	chunks []string
	idx    int
}

func (r *chunkReader) Read(p []byte) (int, error) {
	if r.idx >= len(r.chunks) {
		return 0, io.EOF
	}
	n := copy(p, r.chunks[r.idx])
	r.idx++
	return n, nil
}

func TestResponseExtractor_SSE_ForwardsBytesUnchanged(t *testing.T) {
	sseData := "data: {\"id\":\"chatcmpl-1\"}\n\ndata: {\"id\":\"chatcmpl-2\"}\n\n"
	inner := strings.NewReader(sseData)
	extractor := NewResponseExtractor(inner, "text/event-stream", time.Now())

	got, err := io.ReadAll(extractor)
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	if string(got) != sseData {
		t.Errorf("bytes forwarded unchanged:\ngot:  %q\nwant: %q", string(got), sseData)
	}
}

func TestResponseExtractor_SSE_EventsAcrossReadCalls(t *testing.T) {
	parts := []string{
		"data: {\"id\":\"ch",
		"atcmpl-1\"}\n\ndata: [DONE]\n\n",
	}
	inner := &chunkReader{chunks: parts}
	extractor := NewResponseExtractor(inner, "text/event-stream", time.Now())

	got, err := io.ReadAll(extractor)
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	want := "data: {\"id\":\"chatcmpl-1\"}\n\ndata: [DONE]\n\n"
	if string(got) != want {
		t.Errorf("bytes forwarded unchanged:\ngot:  %q\nwant: %q", string(got), want)
	}
}

func TestResponseExtractor_JSON_ForwardsBytesUnchanged(t *testing.T) {
	jsonData := `{"id":"chatcmpl-1","usage":{"prompt_tokens":10,"completion_tokens":20}}`
	inner := strings.NewReader(jsonData)
	extractor := NewResponseExtractor(inner, "application/json", time.Now())

	got, err := io.ReadAll(extractor)
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	if string(got) != jsonData {
		t.Errorf("bytes forwarded unchanged:\ngot:  %q\nwant: %q", string(got), jsonData)
	}
}

func TestResponseExtractor_SSE_OpenAI_TTFT(t *testing.T) {
	start := time.Now().Add(-100 * time.Millisecond)
	events := []string{
		"data: {\"id\":\"chatcmpl-1\",\"choices\":[{\"delta\":{\"role\":\"assistant\"}}]}\n\n",
		"data: {\"id\":\"chatcmpl-1\",\"choices\":[{\"delta\":{\"content\":\"Hello\"}}]}\n\n",
		"data: {\"id\":\"chatcmpl-1\",\"choices\":[{\"delta\":{\"content\":\" world\"}}]}\n\n",
		"data: [DONE]\n\n",
	}
	inner := &chunkReader{chunks: []string{strings.Join(events, "")}}
	extractor := NewResponseExtractor(inner, "text/event-stream", start)

	_, _ = io.ReadAll(extractor)

	m := extractor.Metrics()
	if m.TTFTMs == nil {
		t.Fatal("expected TTFTMs to be set")
	}
	if *m.TTFTMs < 50 {
		t.Errorf("TTFTMs too low: got %d, expected >= 50", *m.TTFTMs)
	}
}

func TestResponseExtractor_SSE_OpenAI_ToolCallTTFT(t *testing.T) {
	start := time.Now()
	events := []string{
		"data: {\"id\":\"chatcmpl-1\",\"choices\":[{\"delta\":{\"tool_calls\":[{\"index\":0}]}}]}\n\n",
		"data: [DONE]\n\n",
	}
	inner := &chunkReader{chunks: []string{strings.Join(events, "")}}
	extractor := NewResponseExtractor(inner, "text/event-stream", start)

	_, _ = io.ReadAll(extractor)

	m := extractor.Metrics()
	if m.TTFTMs == nil {
		t.Fatal("expected TTFTMs to be set for tool_calls delta")
	}
}

func TestResponseExtractor_SSE_OpenAI_Usage(t *testing.T) {
	events := []string{
		"data: {\"id\":\"chatcmpl-1\",\"choices\":[{\"delta\":{\"content\":\"Hi\"}}]}\n\n",
		"data: {\"id\":\"chatcmpl-1\",\"choices\":[],\"usage\":{\"prompt_tokens\":100,\"completion_tokens\":50,\"prompt_tokens_details\":{\"cached_tokens\":30}}}\n\n",
		"data: [DONE]\n\n",
	}
	inner := &chunkReader{chunks: []string{strings.Join(events, "")}}
	extractor := NewResponseExtractor(inner, "text/event-stream", time.Now())

	_, _ = io.ReadAll(extractor)

	m := extractor.Metrics()
	if m.InputTokens == nil || *m.InputTokens != 100 {
		t.Errorf("InputTokens: got %v, want 100", m.InputTokens)
	}
	if m.OutputTokens == nil || *m.OutputTokens != 50 {
		t.Errorf("OutputTokens: got %v, want 50", m.OutputTokens)
	}
	if m.CacheReadTokens == nil || *m.CacheReadTokens != 30 {
		t.Errorf("CacheReadTokens: got %v, want 30", m.CacheReadTokens)
	}
}

func TestResponseExtractor_SSE_Anthropic_FullFlow(t *testing.T) {
	start := time.Now().Add(-80 * time.Millisecond)
	events := []string{
		"event: message_start\ndata: {\"type\":\"message_start\",\"message\":{\"usage\":{\"input_tokens\":200,\"cache_read_input_tokens\":50,\"cache_creation_input_tokens\":10}}}\n\n",
		"event: content_block_start\ndata: {\"type\":\"content_block_start\",\"content_block\":{\"type\":\"text\",\"text\":\"\"}}\n\n",
		"event: content_block_delta\ndata: {\"type\":\"content_block_delta\",\"delta\":{\"type\":\"text_delta\",\"text\":\"Hello\"}}\n\n",
		"event: message_delta\ndata: {\"type\":\"message_delta\",\"delta\":{\"stop_reason\":\"end_turn\"},\"usage\":{\"output_tokens\":25}}\n\n",
	}
	inner := &chunkReader{chunks: []string{strings.Join(events, "")}}
	extractor := NewResponseExtractor(inner, "text/event-stream", start)

	_, _ = io.ReadAll(extractor)

	m := extractor.Metrics()
	if m.TTFTMs == nil {
		t.Fatal("expected TTFTMs to be set")
	}
	if m.InputTokens == nil || *m.InputTokens != 200 {
		t.Errorf("InputTokens: got %v, want 200", m.InputTokens)
	}
	if m.OutputTokens == nil || *m.OutputTokens != 25 {
		t.Errorf("OutputTokens: got %v, want 25", m.OutputTokens)
	}
	if m.CacheReadTokens == nil || *m.CacheReadTokens != 50 {
		t.Errorf("CacheReadTokens: got %v, want 50", m.CacheReadTokens)
	}
	if m.CacheWriteTokens == nil || *m.CacheWriteTokens != 10 {
		t.Errorf("CacheWriteTokens: got %v, want 10", m.CacheWriteTokens)
	}
}

func TestResponseExtractor_SSE_Anthropic_ToolUseTTFT(t *testing.T) {
	events := []string{
		"event: content_block_start\ndata: {\"type\":\"content_block_start\",\"content_block\":{\"type\":\"tool_use\",\"id\":\"toolu_1\"}}\n\n",
	}
	inner := &chunkReader{chunks: []string{strings.Join(events, "")}}
	extractor := NewResponseExtractor(inner, "text/event-stream", time.Now())

	_, _ = io.ReadAll(extractor)

	m := extractor.Metrics()
	if m.TTFTMs == nil {
		t.Fatal("expected TTFTMs to be set for tool_use content_block_start")
	}
}

func TestResponseExtractor_JSON_OpenAI(t *testing.T) {
	jsonData := `{"id":"chatcmpl-1","choices":[{"message":{"role":"assistant","content":"Hi"}}],"usage":{"prompt_tokens":150,"completion_tokens":75,"prompt_tokens_details":{"cached_tokens":40}}}`
	inner := strings.NewReader(jsonData)
	extractor := NewResponseExtractor(inner, "application/json", time.Now())

	_, _ = io.ReadAll(extractor)

	m := extractor.Metrics()
	if m.InputTokens == nil || *m.InputTokens != 150 {
		t.Errorf("InputTokens: got %v, want 150", m.InputTokens)
	}
	if m.OutputTokens == nil || *m.OutputTokens != 75 {
		t.Errorf("OutputTokens: got %v, want 75", m.OutputTokens)
	}
	if m.CacheReadTokens == nil || *m.CacheReadTokens != 40 {
		t.Errorf("CacheReadTokens: got %v, want 40", m.CacheReadTokens)
	}
	if m.TTFTMs != nil {
		t.Errorf("TTFTMs should be nil for non-streaming JSON, got %v", m.TTFTMs)
	}
}

func TestResponseExtractor_JSON_Anthropic(t *testing.T) {
	jsonData := `{"id":"msg_1","type":"message","content":[{"type":"text","text":"Hi"}],"usage":{"input_tokens":300,"output_tokens":100,"cache_read_input_tokens":60,"cache_creation_input_tokens":15}}`
	inner := strings.NewReader(jsonData)
	extractor := NewResponseExtractor(inner, "application/json", time.Now())

	_, _ = io.ReadAll(extractor)

	m := extractor.Metrics()
	if m.InputTokens == nil || *m.InputTokens != 300 {
		t.Errorf("InputTokens: got %v, want 300", m.InputTokens)
	}
	if m.OutputTokens == nil || *m.OutputTokens != 100 {
		t.Errorf("OutputTokens: got %v, want 100", m.OutputTokens)
	}
	if m.CacheReadTokens == nil || *m.CacheReadTokens != 60 {
		t.Errorf("CacheReadTokens: got %v, want 60", m.CacheReadTokens)
	}
	if m.CacheWriteTokens == nil || *m.CacheWriteTokens != 15 {
		t.Errorf("CacheWriteTokens: got %v, want 15", m.CacheWriteTokens)
	}
}

func TestResponseExtractor_JSON_UnrecognizedFormat(t *testing.T) {
	jsonData := `{"some":"random","data":true}`
	inner := strings.NewReader(jsonData)
	extractor := NewResponseExtractor(inner, "application/json", time.Now())

	_, _ = io.ReadAll(extractor)

	m := extractor.Metrics()
	if m.InputTokens != nil || m.OutputTokens != nil || m.CacheReadTokens != nil || m.CacheWriteTokens != nil || m.TTFTMs != nil {
		t.Errorf("expected all nil metrics for unrecognized JSON, got %+v", m)
	}
}

func TestResponseExtractor_SSE_DONESentinel(t *testing.T) {
	events := []string{
		"data: {\"id\":\"chatcmpl-1\",\"choices\":[{\"delta\":{\"content\":\"Hi\"}}]}\n\n",
		"data: [DONE]\n\n",
	}
	inner := &chunkReader{chunks: []string{strings.Join(events, "")}}
	extractor := NewResponseExtractor(inner, "text/event-stream", time.Now())

	got, err := io.ReadAll(extractor)
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}

	want := strings.Join(events, "")
	if string(got) != want {
		t.Errorf("bytes forwarded unchanged with [DONE]:\ngot:  %q\nwant: %q", string(got), want)
	}

	// Should have TTFT from the first event but no usage
	m := extractor.Metrics()
	if m.TTFTMs == nil {
		t.Fatal("expected TTFTMs to be set from first content event")
	}
	if m.InputTokens != nil {
		t.Errorf("InputTokens should be nil when no usage event, got %v", m.InputTokens)
	}
}

func TestResponseExtractor_SSE_MultiLineData(t *testing.T) {
	// SSE spec: multiple data: lines in one event are joined with \n
	events := "data: {\"id\":\"1\",\ndata: \"choices\":[]}\n\n"
	inner := strings.NewReader(events)
	extractor := NewResponseExtractor(inner, "text/event-stream", time.Now())

	_, _ = io.ReadAll(extractor)

	// Should parse the concatenated payload {"id":"1",\n"choices":[]}
	// gjson can handle this
	m := extractor.Metrics()
	// No usage expected, just verify no panic/crash
	if m.InputTokens != nil {
		t.Errorf("InputTokens should be nil, got %v", m.InputTokens)
	}
}
