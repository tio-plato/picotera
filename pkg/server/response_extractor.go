package server

import (
	"io"
	"strings"
	"time"

	"github.com/tidwall/gjson"
)

// ResponseMetrics holds extracted TTFT and token usage from a provider response.
type ResponseMetrics struct {
	TTFTMs             *int64
	InputTokens        *int64
	OutputTokens       *int64
	CacheReadTokens    *int64
	CacheWriteTokens   *int64
	CacheWrite1HTokens *int64
}

// ResponseExtractor wraps an io.Reader and inspects bytes as they flow through,
// extracting TTFT and token usage from SSE or JSON provider responses.
type ResponseExtractor struct {
	inner        io.Reader
	mode         string // "sse" or "json"
	startTime    time.Time
	metrics      ResponseMetrics
	ttftRecorded bool

	// SSE: line buffer for reassembling events across Read() boundaries
	lineBuf []byte

	// JSON: accumulate full body for post-stream parsing
	jsonBuf []byte
}

// NewResponseExtractor creates a new extractor. contentType is the upstream
// response Content-Type header. startTime is when the upstream request was sent.
func NewResponseExtractor(inner io.Reader, contentType string, startTime time.Time) *ResponseExtractor {
	mode := "json"
	if strings.Contains(strings.ToLower(contentType), "text/event-stream") {
		mode = "sse"
	}
	return &ResponseExtractor{
		inner:     inner,
		mode:      mode,
		startTime: startTime,
	}
}

// Metrics returns the extracted metrics. Call after the Read loop finishes.
func (e *ResponseExtractor) Metrics() ResponseMetrics {
	return e.metrics
}

// Read implements io.Reader. Bytes are forwarded to the caller unchanged.
// SSE bytes are also fed into the line buffer for event parsing.
// JSON bytes are accumulated for post-stream extraction.
func (e *ResponseExtractor) Read(p []byte) (int, error) {
	n, err := e.inner.Read(p)
	if n > 0 {
		chunk := p[:n]
		switch e.mode {
		case "sse":
			e.lineBuf = append(e.lineBuf, chunk...)
			e.processSSEBuffer()
		case "json":
			e.jsonBuf = append(e.jsonBuf, chunk...)
		}
	}
	if err == io.EOF && e.mode == "json" && len(e.jsonBuf) > 0 {
		e.extractJSONMetrics()
	}
	return n, err
}

// processSSEBuffer scans the line buffer for complete SSE events (delimited by \n\n),
// processes each event, and removes processed bytes from the buffer.
func (e *ResponseExtractor) processSSEBuffer() {
	for {
		idx := bytesIndex(e.lineBuf, "\n\n")
		if idx == -1 {
			break
		}
		eventBytes := e.lineBuf[:idx]
		e.lineBuf = e.lineBuf[idx+2:]
		e.processSSEEvent(eventBytes)
	}
}

// processSSEEvent parses a single SSE event and extracts metrics.
func (e *ResponseExtractor) processSSEEvent(eventBytes []byte) {
	// Extract data: lines and concatenate (per SSE spec, multi-line data is joined with \n)
	var dataPayloads []string
	lines := strings.Split(string(eventBytes), "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "data: ") {
			dataPayloads = append(dataPayloads, strings.TrimPrefix(line, "data: "))
		} else if strings.HasPrefix(line, "data:") {
			dataPayloads = append(dataPayloads, strings.TrimPrefix(line, "data:"))
		}
	}
	if len(dataPayloads) == 0 {
		return
	}
	payload := strings.Join(dataPayloads, "\n")

	// Skip [DONE] sentinel
	if payload == "[DONE]" {
		return
	}

	// Try OpenAI Chat Completions format
	e.extractOpenAISSE(payload)
	// Try OpenAI Responses format
	e.extractOpenAIResponsesSSE(payload)
	// Try Anthropic format
	e.extractAnthropicSSE(payload)
}

func (e *ResponseExtractor) extractOpenAISSE(payload string) {
	result := gjson.Parse(payload)

	// TTFT: first content or tool_calls delta
	if !e.ttftRecorded {
		content := result.Get("choices.0.delta.content")
		reasoning := result.Get("choices.0.delta.reasoning")
		reasoningContent := result.Get("choices.0.delta.reasoning_content")
		toolCalls := result.Get("choices.0.delta.tool_calls")
		if (content.Exists()) || (reasoning.Exists()) || (reasoningContent.Exists()) || toolCalls.Exists() {
			ttft := time.Since(e.startTime).Milliseconds()
			e.metrics.TTFTMs = &ttft
			e.ttftRecorded = true
		}
	}

	// Usage
	usage := result.Get("usage")
	if usage.Exists() {
		e.setOpenAIInputTokens(usage)
		if v := usage.Get("completion_tokens"); v.Exists() {
			val := v.Int()
			e.metrics.OutputTokens = &val
		}
		if v := usage.Get("prompt_tokens_details.cached_tokens"); v.Exists() {
			val := v.Int()
			e.metrics.CacheReadTokens = &val
		}
	}
}

func (e *ResponseExtractor) extractOpenAIResponsesSSE(payload string) {
	result := gjson.Parse(payload)
	eventType := result.Get("type").String()

	// Only process OpenAI Responses API events
	if !strings.HasPrefix(eventType, "response.") {
		return
	}

	// TTFT: first output text or function call delta
	if !e.ttftRecorded {
		if eventType == "response.output_text.delta" || eventType == "response.function_call_arguments.delta" || eventType == "response.output_item.added" {
			ttft := time.Since(e.startTime).Milliseconds()
			e.metrics.TTFTMs = &ttft
			e.ttftRecorded = true
		}
	}

	// Usage from response.completed
	if eventType == "response.completed" {
		usage := result.Get("response.usage")
		if usage.Exists() {
			e.setOpenAIInputTokens(usage)
			if v := usage.Get("output_tokens"); v.Exists() {
				val := v.Int()
				e.metrics.OutputTokens = &val
			}
			if v := usage.Get("input_tokens_details.cached_tokens"); v.Exists() {
				val := v.Int()
				e.metrics.CacheReadTokens = &val
			}
		}
	}
}

func (e *ResponseExtractor) extractAnthropicSSE(payload string) {
	result := gjson.Parse(payload)
	eventType := result.Get("type").String()

	// TTFT: first content_block_delta with text_delta, or content_block_start with tool_use
	if !e.ttftRecorded {
		if eventType == "content_block_delta" || eventType == "content_block_start" {
			ttft := time.Since(e.startTime).Milliseconds()
			e.metrics.TTFTMs = &ttft
			e.ttftRecorded = true
		}
	}

	// Usage from message_start (input tokens, cache tokens)
	if eventType == "message_start" {
		msgUsage := result.Get("message.usage")
		if v := msgUsage.Get("input_tokens"); v.Exists() {
			val := v.Int()
			e.metrics.InputTokens = &val
		}
		if v := msgUsage.Get("cache_read_input_tokens"); v.Exists() {
			val := v.Int()
			e.metrics.CacheReadTokens = &val
		}
		e.extractAnthropicCacheCreation(msgUsage)
	}

	// Usage from message_delta (output tokens, cache tokens)
	if eventType == "message_delta" {
		if v := result.Get("usage.output_tokens"); v.Exists() {
			val := v.Int()
			e.metrics.OutputTokens = &val
		}
		if v := result.Get("usage.cache_read_input_tokens"); v.Exists() {
			val := v.Int()
			e.metrics.CacheReadTokens = &val
		}
		e.extractAnthropicCacheCreation(result.Get("usage"))
	}
}

// extractJSONMetrics parses the accumulated JSON body and extracts usage metrics.
func (e *ResponseExtractor) extractJSONMetrics() {
	result := gjson.ParseBytes(e.jsonBuf)

	// Try OpenAI Chat Completions format
	usage := result.Get("usage")
	e.setOpenAIInputTokens(usage)
	if v := result.Get("usage.completion_tokens"); v.Exists() {
		val := v.Int()
		e.metrics.OutputTokens = &val
	}
	if v := result.Get("usage.prompt_tokens_details.cached_tokens"); v.Exists() {
		val := v.Int()
		e.metrics.CacheReadTokens = &val
	}

	// Try OpenAI Responses format (only sets if Chat Completions didn't find fields)
	if e.metrics.InputTokens == nil {
		e.setOpenAIInputTokens(usage)
	}
	if e.metrics.OutputTokens == nil {
		if v := result.Get("usage.output_tokens"); v.Exists() {
			val := v.Int()
			e.metrics.OutputTokens = &val
		}
	}
	if e.metrics.CacheReadTokens == nil {
		if v := result.Get("usage.input_tokens_details.cached_tokens"); v.Exists() {
			val := v.Int()
			e.metrics.CacheReadTokens = &val
		}
	}

	// Try Anthropic format (only sets if above didn't find fields)
	if e.metrics.InputTokens == nil {
		if v := result.Get("usage.input_tokens"); v.Exists() {
			val := v.Int()
			e.metrics.InputTokens = &val
		}
	}
	if e.metrics.OutputTokens == nil {
		if v := result.Get("usage.output_tokens"); v.Exists() {
			val := v.Int()
			e.metrics.OutputTokens = &val
		}
	}
	if e.metrics.CacheReadTokens == nil {
		if v := result.Get("usage.cache_read_input_tokens"); v.Exists() {
			val := v.Int()
			e.metrics.CacheReadTokens = &val
		}
	}
	if e.metrics.CacheWriteTokens == nil {
		e.extractAnthropicCacheCreation(result.Get("usage"))
	}
}

func (e *ResponseExtractor) extractAnthropicCacheCreation(usage gjson.Result) {
	if !usage.Exists() {
		return
	}

	cacheCreation := usage.Get("cache_creation")
	ephemeral5m := cacheCreation.Get("ephemeral_5m_input_tokens")
	ephemeral1h := cacheCreation.Get("ephemeral_1h_input_tokens")
	if ephemeral5m.Exists() && ephemeral1h.Exists() {
		cacheWrite := ephemeral5m.Int()
		cacheWrite1h := ephemeral1h.Int()
		e.metrics.CacheWriteTokens = &cacheWrite
		e.metrics.CacheWrite1HTokens = &cacheWrite1h
		return
	}

	// Flat total only — don't clobber a previously-extracted breakdown.
	// Why: message_delta repeats cache_creation_input_tokens but often omits
	// the cache_creation breakdown that message_start already supplied.
	if e.metrics.CacheWriteTokens != nil {
		return
	}
	if v := usage.Get("cache_creation_input_tokens"); v.Exists() {
		val := v.Int()
		e.metrics.CacheWriteTokens = &val
	}
}

func (e *ResponseExtractor) setOpenAIInputTokens(usage gjson.Result) {
	if !usage.Exists() {
		return
	}

	total := usage.Get("prompt_tokens")
	cached := usage.Get("prompt_tokens_details.cached_tokens")
	if !total.Exists() {
		total = usage.Get("input_tokens")
		cached = usage.Get("input_tokens_details.cached_tokens")
	}
	if !total.Exists() {
		return
	}

	val := total.Int()
	if cached.Exists() {
		val -= cached.Int()
	}
	e.metrics.InputTokens = &val
}

// bytesIndex finds the index of sep in buf, or -1 if not found.
func bytesIndex(buf []byte, sep string) int {
	sepBytes := []byte(sep)
	for i := 0; i <= len(buf)-len(sepBytes); i++ {
		match := true
		for j := range sepBytes {
			if buf[i+j] != sepBytes[j] {
				match = false
				break
			}
		}
		if match {
			return i
		}
	}
	return -1
}
