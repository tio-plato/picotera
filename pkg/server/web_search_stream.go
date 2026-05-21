package server

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"strings"

	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

// webSearchSSETransformer wraps an Anthropic Messages SSE stream and inlines
// web_search emulation: tool_use(web_search) blocks are buffered, the
// accumulated input JSON is dispatched to Exa, and a synthesized
// server_tool_use + web_search_tool_result pair is emitted to the client.
//
// content_block indices on every passthrough event after an expansion are
// shifted by +1 per expansion to keep the client-facing index space
// monotonically increasing.
//
// stop_reason on message_delta is rewritten from "tool_use" to "pause_turn"
// when every tool_use seen during this turn was a web_search call.
type webSearchSSETransformer struct {
	upstream io.ReadCloser
	pr       *io.PipeReader
	pw       *io.PipeWriter
	ctx      context.Context
	wsCtx    *webSearchContext
	h        *gatewayHandler

	upstreamBuf []byte

	state sseState

	bufUpstreamIdx     int64
	bufServerToolUseID string
	bufInputJSON       bytes.Buffer

	idxOffset      int64
	webSearchCalls int
	otherToolCalls int
}

type sseState int

const (
	ssePassthrough sseState = iota
	sseBuffering
)

func newWebSearchSSETransformer(ctx context.Context, upstream io.ReadCloser, wsCtx *webSearchContext, h *gatewayHandler) io.ReadCloser {
	pr, pw := io.Pipe()
	t := &webSearchSSETransformer{
		upstream: upstream,
		pr:       pr,
		pw:       pw,
		ctx:      ctx,
		wsCtx:    wsCtx,
		h:        h,
	}
	go t.run()
	return t
}

func (t *webSearchSSETransformer) Read(p []byte) (int, error) { return t.pr.Read(p) }

func (t *webSearchSSETransformer) Close() error {
	_ = t.pr.Close()
	return t.upstream.Close()
}

func (t *webSearchSSETransformer) run() {
	defer func() { _ = t.pw.Close() }()
	buf := make([]byte, 8*1024)
	for {
		n, err := t.upstream.Read(buf)
		if n > 0 {
			t.feedChunk(buf[:n])
		}
		if err != nil {
			return
		}
	}
}

func (t *webSearchSSETransformer) feedChunk(chunk []byte) {
	t.upstreamBuf = append(t.upstreamBuf, chunk...)
	for {
		idx := bytesIndex(t.upstreamBuf, "\n\n")
		if idx == -1 {
			return
		}
		evt := append([]byte(nil), t.upstreamBuf[:idx]...)
		t.upstreamBuf = t.upstreamBuf[idx+2:]
		t.handleEvent(evt)
	}
}

// handleEvent dispatches one fully-buffered SSE frame. The frame's raw bytes
// (`event: …\n data: …`) are split into the event name and the data payload;
// dispatch is keyed off the data payload's `type` (Anthropic puts the
// canonical event type there, which matches the SSE event: header for native
// streams but is the authoritative field).
func (t *webSearchSSETransformer) handleEvent(evt []byte) {
	eventName, dataPayload := parseSSEFrame(evt)
	if dataPayload == "" {
		t.writeRaw(evt)
		return
	}
	parsed := gjson.Parse(dataPayload)
	eventType := parsed.Get("type").Str
	if eventName == "" {
		eventName = eventType
	}

	switch t.state {
	case sseBuffering:
		switch eventType {
		case "content_block_delta":
			if parsed.Get("index").Int() != t.bufUpstreamIdx {
				// Out-of-order event for a different block — keep buffering
				// the current one and forward this one (after offset).
				t.writeAdjusted(eventName, parsed)
				return
			}
			if pj := parsed.Get("delta.partial_json"); pj.Exists() {
				t.bufInputJSON.WriteString(pj.Str)
			}
			return
		case "content_block_stop":
			if parsed.Get("index").Int() != t.bufUpstreamIdx {
				t.writeAdjusted(eventName, parsed)
				return
			}
			t.flushBufferedWebSearch()
			return
		default:
			t.writeAdjusted(eventName, parsed)
			return
		}
	case ssePassthrough:
		switch eventType {
		case "content_block_start":
			if t.isWebSearchToolUseStart(parsed) {
				t.beginBuffering(parsed)
				return
			}
			if parsed.Get("content_block.type").Str == "tool_use" {
				t.otherToolCalls++
			}
			t.writeAdjusted(eventName, parsed)
		case "message_delta":
			t.writeMessageDelta(eventName, parsed)
		default:
			t.writeAdjusted(eventName, parsed)
		}
	}
}

func (t *webSearchSSETransformer) isWebSearchToolUseStart(parsed gjson.Result) bool {
	if parsed.Get("type").Str != "content_block_start" {
		return false
	}
	cb := parsed.Get("content_block")
	return cb.Get("type").Str == "tool_use" && cb.Get("name").Str == "web_search"
}

func (t *webSearchSSETransformer) beginBuffering(parsed gjson.Result) {
	t.state = sseBuffering
	t.bufUpstreamIdx = parsed.Get("index").Int()
	t.bufInputJSON.Reset()
	t.bufServerToolUseID = mapToolUseIDToServer(parsed.Get("content_block.id").Str)
	if t.bufServerToolUseID == "" {
		t.bufServerToolUseID = generateToolUseID()
	}
}

// flushBufferedWebSearch emits the server_tool_use + web_search_tool_result
// pair that replaces the buffered upstream tool_use(web_search) block. The
// Exa call is synchronous and happens between the two start/stop pairs.
func (t *webSearchSSETransformer) flushBufferedWebSearch() {
	t.webSearchCalls++
	outIdxServer := t.bufUpstreamIdx + t.idxOffset
	outIdxResult := outIdxServer + 1

	inputJSON := strings.TrimSpace(t.bufInputJSON.String())
	if inputJSON == "" {
		inputJSON = "{}"
	}

	var inputObj map[string]any
	if err := json.Unmarshal([]byte(inputJSON), &inputObj); err != nil || inputObj == nil {
		inputObj = map[string]any{}
	}

	// 1) content_block_start for server_tool_use carrying the full input
	// inline. Streaming the input via input_json_delta is unsafe here because
	// downstream aggregators only accumulate partial_json for tool_use, not
	// server_tool_use, and would lose the payload.
	startBlock := map[string]any{
		"type":  "server_tool_use",
		"id":    t.bufServerToolUseID,
		"name":  "web_search",
		"input": inputObj,
	}
	t.writeEvent("content_block_start", map[string]any{
		"type":          "content_block_start",
		"index":         outIdxServer,
		"content_block": startBlock,
	})

	// 2) content_block_stop for server_tool_use.
	t.writeEvent("content_block_stop", map[string]any{
		"type":  "content_block_stop",
		"index": outIdxServer,
	})

	// 3) Exa call.
	var resultBlock map[string]any
	if exaResp, err := t.h.callExa(t.ctx, json.RawMessage(inputJSON), t.wsCtx); err != nil {
		resultBlock = rawMessageToMap(buildWebSearchToolResultError(t.bufServerToolUseID, "unavailable"))
	} else {
		resultBlock = rawMessageToMap(buildWebSearchToolResult(t.bufServerToolUseID, exaResp))
	}

	// 4) content_block_start for web_search_tool_result (carries full content).
	t.writeEvent("content_block_start", map[string]any{
		"type":          "content_block_start",
		"index":         outIdxResult,
		"content_block": resultBlock,
	})
	// 5) content_block_stop.
	t.writeEvent("content_block_stop", map[string]any{
		"type":  "content_block_stop",
		"index": outIdxResult,
	})

	t.idxOffset++
	t.state = ssePassthrough
}

// writeAdjusted emits the upstream event after applying the running index
// offset to any `index` field. Bytes-level reuse is impossible because the
// JSON payload changes shape; we re-marshal from the parsed gjson result.
func (t *webSearchSSETransformer) writeAdjusted(eventName string, parsed gjson.Result) {
	idxResult := parsed.Get("index")
	payload := parsed.Raw
	if idxResult.Exists() {
		newIdx := idxResult.Int() + t.idxOffset
		updated, err := setJSONField(payload, "index", newIdx)
		if err == nil {
			payload = updated
		}
	}
	t.writeRawEvent(eventName, []byte(payload))
}

// writeMessageDelta forwards message_delta, optionally rewriting stop_reason
// to "pause_turn" when every tool_use seen this turn was a web_search call.
func (t *webSearchSSETransformer) writeMessageDelta(eventName string, parsed gjson.Result) {
	payload := parsed.Raw
	stopReason := parsed.Get("delta.stop_reason").Str
	if stopReason == "tool_use" && t.webSearchCalls > 0 && t.otherToolCalls == 0 {
		updated, err := setJSONField(payload, "delta.stop_reason", "pause_turn")
		if err == nil {
			payload = updated
		}
	}
	t.writeRawEvent(eventName, []byte(payload))
}

// writeEvent encodes a freshly-built map and sends it as an SSE frame.
func (t *webSearchSSETransformer) writeEvent(eventName string, payload map[string]any) {
	body, err := json.Marshal(payload)
	if err != nil {
		return
	}
	t.writeRawEvent(eventName, body)
}

func (t *webSearchSSETransformer) writeRawEvent(eventName string, jsonPayload []byte) {
	if eventName == "" {
		eventName = gjson.GetBytes(jsonPayload, "type").Str
	}
	var b bytes.Buffer
	b.WriteString("event: ")
	b.WriteString(eventName)
	b.WriteString("\ndata: ")
	b.Write(jsonPayload)
	b.WriteString("\n\n")
	_, _ = t.pw.Write(b.Bytes())
}

func (t *webSearchSSETransformer) writeRaw(evt []byte) {
	_, _ = t.pw.Write(evt)
	_, _ = t.pw.Write([]byte("\n\n"))
}

// parseSSEFrame extracts the `event:` name and the concatenated `data:` body
// from a fully buffered SSE frame.
func parseSSEFrame(evt []byte) (string, string) {
	lines := strings.Split(string(evt), "\n")
	var eventName string
	dataLines := make([]string, 0, len(lines))
	for _, line := range lines {
		switch {
		case strings.HasPrefix(line, "event: "):
			eventName = strings.TrimPrefix(line, "event: ")
		case strings.HasPrefix(line, "event:"):
			eventName = strings.TrimPrefix(line, "event:")
		case strings.HasPrefix(line, "data: "):
			dataLines = append(dataLines, strings.TrimPrefix(line, "data: "))
		case strings.HasPrefix(line, "data:"):
			dataLines = append(dataLines, strings.TrimPrefix(line, "data:"))
		}
	}
	return eventName, strings.Join(dataLines, "\n")
}

// setJSONField rewrites a single JSON field by path while preserving the
// surrounding structure.
func setJSONField(raw string, path string, value any) (string, error) {
	out, err := sjson.SetBytes([]byte(raw), path, value)
	if err != nil {
		return "", err
	}
	return string(out), nil
}

// rawMessageToMap turns a json.RawMessage carrying a JSON object into a
// generic map for re-marshalling inside larger SSE payloads.
func rawMessageToMap(raw json.RawMessage) map[string]any {
	var m map[string]any
	_ = json.Unmarshal(raw, &m)
	return m
}
