package server

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"time"

	"picotera/pkg/contract"

	"github.com/rs/xid"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

const webSearchMaxRounds = 10

// webSearchContext tracks the per-request state needed to emulate
// Anthropic-native web search using Exa when the chosen upstream does not
// natively support web search. It is created by the unified gateway after the
// outbound rewrite fires and consumed by both the non-stream and SSE response
// transformers.
type webSearchContext struct {
	active              bool
	apiKeyToken         string
	metaID              string
	parentSpanID        string
	metaCreatedAt       time.Time
	originalRequestBody []byte // pre-rewrite client body for sub-call construction
}

// hasWebSearchTool reports whether the Anthropic Messages body declares an
// Anthropic server-side web search tool. Recognizes both versions.
func hasWebSearchTool(body []byte) bool {
	tools := gjson.GetBytes(body, "tools")
	if !tools.IsArray() {
		return false
	}
	found := false
	tools.ForEach(func(_, val gjson.Result) bool {
		t := val.Get("type").Str
		if t == "web_search_20250305" || t == "web_search_20260209" {
			found = true
			return false
		}
		return true
	})
	return found
}

// webSearchFunctionToolJSON is the function-tool replacement injected into the
// outbound `tools` array. The schema mirrors Exa's POST /search body so the
// LLM can populate them directly; only `query` is required.
var webSearchFunctionToolJSON = json.RawMessage(`{
  "name": "web_search",
  "description": "Search the web for current information. Returns relevant snippets from web pages.",
  "input_schema": {
    "type": "object",
    "properties": {
      "query": {"type": "string", "description": "The search query"},
      "numResults": {"type": "integer", "description": "Number of results to return (default 10, max 25)"},
      "category": {"type": "string", "enum": ["company", "research paper", "news", "personal site", "financial report", "people"], "description": "Optional category filter"},
      "includeDomains": {"type": "array", "items": {"type": "string"}, "description": "Restrict results to these domains"},
      "excludeDomains": {"type": "array", "items": {"type": "string"}, "description": "Exclude results from these domains"},
      "startPublishedDate": {"type": "string", "description": "ISO 8601 date; only results published after this"},
      "endPublishedDate": {"type": "string", "description": "ISO 8601 date; only results published before this"}
    },
    "required": ["query"]
  }
}`)

// rewriteWebSearchTools replaces every Anthropic web-search server tool entry
// in the `tools` array with a function-tool equivalent that exposes Exa's
// parameters as the input schema. Other tools pass through unchanged.
func rewriteWebSearchTools(body []byte) ([]byte, error) {
	tools := gjson.GetBytes(body, "tools")
	if !tools.IsArray() {
		return body, nil
	}
	out := make([]json.RawMessage, 0)
	tools.ForEach(func(_, val gjson.Result) bool {
		t := val.Get("type").Str
		if t == "web_search_20250305" || t == "web_search_20260209" {
			out = append(out, webSearchFunctionToolJSON)
		} else {
			out = append(out, json.RawMessage(val.Raw))
		}
		return true
	})
	encoded, err := json.Marshal(out)
	if err != nil {
		return nil, fmt.Errorf("rewriteWebSearchTools: marshal tools: %w", err)
	}
	return sjson.SetRawBytes(body, "tools", encoded)
}

// rewriteWebSearchHistory walks the messages array and converts assistant-side
// Anthropic web-search blocks into the equivalent function-tool flow the
// upstream will accept:
//
//   - server_tool_use(web_search)   → tool_use (ID prefix srvtoolu_ → toolu_)
//   - web_search_tool_result       → split into a new user message carrying a
//     single tool_result block; the encoded text is the
//     human-readable Exa highlight summary.
//   - text blocks that follow a web_search_tool_result get split into a new
//     assistant message; any `web_search_result_location` citations are
//     stripped because the upstream does not recognize them.
//
// Messages that do not need splitting are passed through verbatim.
func rewriteWebSearchHistory(body []byte) ([]byte, error) {
	msgs := gjson.GetBytes(body, "messages")
	if !msgs.IsArray() {
		return body, nil
	}
	out := make([]json.RawMessage, 0, len(msgs.Array()))
	var convertErr error
	msgs.ForEach(func(_, msg gjson.Result) bool {
		role := msg.Get("role").Str
		if role != "assistant" {
			out = append(out, json.RawMessage(msg.Raw))
			return true
		}
		content := msg.Get("content")
		if !content.IsArray() {
			out = append(out, json.RawMessage(msg.Raw))
			return true
		}
		converted, err := convertAssistantMessage(msg, content)
		if err != nil {
			convertErr = err
			return false
		}
		out = append(out, converted...)
		return true
	})
	if convertErr != nil {
		return nil, convertErr
	}
	encoded, err := json.Marshal(out)
	if err != nil {
		return nil, fmt.Errorf("rewriteWebSearchHistory: marshal messages: %w", err)
	}
	return sjson.SetRawBytes(body, "messages", encoded)
}

// convertAssistantMessage walks one assistant message's content blocks and
// returns 1+ messages (assistant / user / assistant / …) in the correct order
// once web-search blocks are expanded.
func convertAssistantMessage(msg, content gjson.Result) ([]json.RawMessage, error) {
	currentAssistant := make([]json.RawMessage, 0)
	out := make([]json.RawMessage, 0, 1)

	flushAssistant := func() {
		if len(currentAssistant) == 0 {
			return
		}
		blob, err := encodeAssistantWithContent(msg, currentAssistant)
		if err != nil {
			return
		}
		out = append(out, blob)
		currentAssistant = make([]json.RawMessage, 0)
	}

	var loopErr error
	content.ForEach(func(_, block gjson.Result) bool {
		blockType := block.Get("type").Str
		switch blockType {
		case "server_tool_use":
			if block.Get("name").Str == "web_search" {
				converted, err := convertServerToolUseToToolUse(block)
				if err != nil {
					loopErr = err
					return false
				}
				currentAssistant = append(currentAssistant, converted)
				return true
			}
			currentAssistant = append(currentAssistant, json.RawMessage(block.Raw))
		case "web_search_tool_result":
			toolUseID := mapWebSearchToolUseID(block.Get("tool_use_id").Str)
			text := renderWebSearchResultsAsText(block.Get("content"))
			toolResult, err := encodeUserToolResult(toolUseID, text)
			if err != nil {
				loopErr = err
				return false
			}
			flushAssistant()
			out = append(out, toolResult)
		case "text":
			currentAssistant = append(currentAssistant, stripWebSearchCitations(block))
		default:
			currentAssistant = append(currentAssistant, json.RawMessage(block.Raw))
		}
		return true
	})
	if loopErr != nil {
		return nil, loopErr
	}
	flushAssistant()
	if len(out) == 0 {
		out = append(out, json.RawMessage(msg.Raw))
	}
	return out, nil
}

// encodeAssistantWithContent rebuilds an assistant message preserving every
// non-content field from the original (so things like custom metadata stay
// intact) and swapping in the converted content slice.
func encodeAssistantWithContent(msg gjson.Result, blocks []json.RawMessage) (json.RawMessage, error) {
	fields := make(map[string]json.RawMessage)
	msg.ForEach(func(key, val gjson.Result) bool {
		if key.Str == "content" {
			return true
		}
		fields[key.Str] = json.RawMessage(val.Raw)
		return true
	})
	if _, ok := fields["role"]; !ok {
		fields["role"] = json.RawMessage(`"assistant"`)
	}
	contentBytes, err := json.Marshal(blocks)
	if err != nil {
		return nil, fmt.Errorf("encodeAssistantWithContent: marshal content: %w", err)
	}
	fields["content"] = contentBytes
	return json.Marshal(fields)
}

func encodeUserToolResult(toolUseID, text string) (json.RawMessage, error) {
	msg := map[string]any{
		"role": "user",
		"content": []map[string]any{
			{
				"type":        "tool_result",
				"tool_use_id": toolUseID,
				"content":     text,
			},
		},
	}
	return json.Marshal(msg)
}

func convertServerToolUseToToolUse(block gjson.Result) (json.RawMessage, error) {
	fields := make(map[string]json.RawMessage)
	block.ForEach(func(key, val gjson.Result) bool {
		switch key.Str {
		case "type":
			fields["type"] = json.RawMessage(`"tool_use"`)
		case "id":
			rawID := val.Str
			mapped, err := json.Marshal(mapServerToolUseID(rawID))
			if err != nil {
				return false
			}
			fields["id"] = mapped
		default:
			fields[key.Str] = json.RawMessage(val.Raw)
		}
		return true
	})
	if _, ok := fields["type"]; !ok {
		fields["type"] = json.RawMessage(`"tool_use"`)
	}
	return json.Marshal(fields)
}

func mapServerToolUseID(id string) string {
	if strings.HasPrefix(id, "srvtoolu_") {
		return "toolu_" + strings.TrimPrefix(id, "srvtoolu_")
	}
	return id
}

func mapWebSearchToolUseID(id string) string {
	return mapServerToolUseID(id)
}

// renderWebSearchResultsAsText produces the prose injected as tool_result
// content. Plain text format keeps the LLM-facing context compact while still
// surfacing the URL + title + highlight body for each Exa hit.
func renderWebSearchResultsAsText(content gjson.Result) string {
	if !content.IsArray() {
		return "No results."
	}
	var b strings.Builder
	b.WriteString("Search results:\n")
	idx := 0
	content.ForEach(func(_, item gjson.Result) bool {
		if item.Get("type").Str != "web_search_result" {
			return true
		}
		idx++
		title := item.Get("title").Str
		url := item.Get("url").Str
		body := item.Get("encrypted_content").Str
		fmt.Fprintf(&b, "\n%d. %s\n   URL: %s\n", idx, title, url)
		if body != "" {
			for _, line := range strings.Split(body, "\n") {
				if line == "" {
					continue
				}
				fmt.Fprintf(&b, "   %s\n", line)
			}
		}
		return true
	})
	if idx == 0 {
		return "No results."
	}
	return b.String()
}

// stripWebSearchCitations removes citations whose type is
// web_search_result_location (the upstream cannot validate them). If a text
// block carries no other citations, the field is dropped entirely.
func stripWebSearchCitations(block gjson.Result) json.RawMessage {
	citations := block.Get("citations")
	if !citations.Exists() || !citations.IsArray() {
		return json.RawMessage(block.Raw)
	}
	kept := make([]json.RawMessage, 0)
	citations.ForEach(func(_, c gjson.Result) bool {
		if c.Get("type").Str == "web_search_result_location" {
			return true
		}
		kept = append(kept, json.RawMessage(c.Raw))
		return true
	})
	fields := make(map[string]json.RawMessage)
	block.ForEach(func(key, val gjson.Result) bool {
		if key.Str == "citations" {
			return true
		}
		fields[key.Str] = json.RawMessage(val.Raw)
		return true
	})
	if len(kept) > 0 {
		encoded, err := json.Marshal(kept)
		if err == nil {
			fields["citations"] = encoded
		}
	}
	raw, err := json.Marshal(fields)
	if err != nil {
		return json.RawMessage(block.Raw)
	}
	return raw
}

// ExaSearchRequest mirrors the Exa /search payload fields we let the LLM
// populate (via the function-tool input schema), plus the fixed
// contents.highlights flag we always set.
type ExaSearchRequest struct {
	Query              string         `json:"query"`
	NumResults         int            `json:"numResults,omitempty"`
	Category           string         `json:"category,omitempty"`
	IncludeDomains     []string       `json:"includeDomains,omitempty"`
	ExcludeDomains     []string       `json:"excludeDomains,omitempty"`
	StartPublishedDate string         `json:"startPublishedDate,omitempty"`
	EndPublishedDate   string         `json:"endPublishedDate,omitempty"`
	Contents           ExaContentsReq `json:"contents"`
}

type ExaContentsReq struct {
	Highlights bool `json:"highlights"`
}

// ExaSearchResponse only decodes the fields the gateway uses to build the
// Anthropic web_search_tool_result block; everything else is ignored.
type ExaSearchResponse struct {
	Results []ExaResult `json:"results"`
}

type ExaResult struct {
	URL           string   `json:"url"`
	Title         string   `json:"title"`
	Highlights    []string `json:"highlights"`
	PublishedDate string   `json:"publishedDate"`
}

// callExa routes a search through the path-based gateway so the request goes
// through full provider resolution, JS hooks, retry, and logging. The Exa
// upstream is reached by `httptest.NewRequest` against the first endpoint with
// endpoint_type == exaSearch. Errors here surface to the caller and are
// rendered as a web_search_tool_result_error block.
func (h *gatewayHandler) callExa(ctx context.Context, toolInput json.RawMessage, wsCtx *webSearchContext) (*ExaSearchResponse, error) {
	if wsCtx == nil || !wsCtx.active {
		return nil, errors.New("callExa: web search context inactive")
	}
	ep, err := h.queries.GetFirstEndpointByType(ctx, contract.EndpointType_ExaSearch)
	if err != nil {
		return nil, fmt.Errorf("callExa: no exaSearch endpoint configured: %w", err)
	}

	var exaReq ExaSearchRequest
	if len(toolInput) > 0 {
		if err := json.Unmarshal(toolInput, &exaReq); err != nil {
			return nil, fmt.Errorf("callExa: decode tool input: %w", err)
		}
	}
	if exaReq.Query == "" {
		return nil, errors.New("callExa: query is empty")
	}
	exaReq.Contents = ExaContentsReq{Highlights: true}

	bodyBytes, err := json.Marshal(exaReq)
	if err != nil {
		return nil, fmt.Errorf("callExa: marshal request: %w", err)
	}

	subReq := httptest.NewRequestWithContext(ctx, http.MethodPost, ep.Path, bytes.NewReader(bodyBytes))
	subReq.Header.Set("Content-Type", "application/json")
	if wsCtx.apiKeyToken != "" {
		subReq.Header.Set("Authorization", "Bearer "+wsCtx.apiKeyToken)
	}
	if wsCtx.parentSpanID != "" {
		subReq.Header.Set("X-Session-Affinity", wsCtx.parentSpanID)
	}

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, subReq)
	resp := rec.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("callExa: upstream status %d: %s", resp.StatusCode, string(respBytes))
	}
	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("callExa: read response: %w", err)
	}
	var out ExaSearchResponse
	if err := json.Unmarshal(respBytes, &out); err != nil {
		return nil, fmt.Errorf("callExa: decode response: %w", err)
	}
	return &out, nil
}

// buildWebSearchToolResult constructs an Anthropic web_search_tool_result
// block from an Exa response. Each Exa hit maps to one web_search_result with
// highlights joined and stored plaintext in encrypted_content.
func buildWebSearchToolResult(toolUseID string, exaResp *ExaSearchResponse) json.RawMessage {
	results := make([]map[string]any, 0, len(exaResp.Results))
	for _, r := range exaResp.Results {
		entry := map[string]any{
			"type":              "web_search_result",
			"url":               r.URL,
			"title":             r.Title,
			"encrypted_content": strings.Join(r.Highlights, "\n\n"),
		}
		if r.PublishedDate != "" {
			if pageAge := formatPageAge(r.PublishedDate); pageAge != "" {
				entry["page_age"] = pageAge
			}
		}
		results = append(results, entry)
	}
	block := map[string]any{
		"type":        "web_search_tool_result",
		"tool_use_id": toolUseID,
		"content":     results,
	}
	out, _ := json.Marshal(block)
	return out
}

func buildWebSearchToolResultError(toolUseID, errorCode string) json.RawMessage {
	block := map[string]any{
		"type":        "web_search_tool_result",
		"tool_use_id": toolUseID,
		"content": map[string]any{
			"type":       "web_search_tool_result_error",
			"error_code": errorCode,
		},
	}
	out, _ := json.Marshal(block)
	return out
}

// formatPageAge tries common Exa date formats and renders the date as
// "January 2, 2006". Unrecognized inputs return empty string so the caller
// drops the field.
func formatPageAge(s string) string {
	for _, layout := range []string{
		time.RFC3339,
		"2006-01-02T15:04:05Z",
		"2006-01-02",
	} {
		if t, err := time.Parse(layout, s); err == nil {
			return t.Format("January 2, 2006")
		}
	}
	return ""
}

// mapToolUseID flips the prefix from toolu_ → srvtoolu_ when emitting an
// Anthropic-native server_tool_use back to the client.
func mapToolUseIDToServer(id string) string {
	if strings.HasPrefix(id, "toolu_") {
		return "srvtoolu_" + strings.TrimPrefix(id, "toolu_")
	}
	return id
}

// generateToolUseID returns a synthetic server_tool_use id used when the
// upstream tool_use lacked one (defensive — Anthropic always supplies one).
func generateToolUseID() string {
	return "srvtoolu_" + xid.New().String()
}

// transformWebSearchResponse rewrites a non-stream Anthropic Messages JSON
// response in-place: every tool_use(web_search) is converted to
// server_tool_use and followed by a web_search_tool_result block populated
// via Exa. stop_reason is downgraded to "pause_turn" only when every tool_use
// in the response is a web_search call.
func (h *gatewayHandler) transformWebSearchResponse(ctx context.Context, body []byte, wsCtx *webSearchContext) ([]byte, error) {
	content := gjson.GetBytes(body, "content")
	if !content.IsArray() {
		return body, nil
	}
	out := make([]json.RawMessage, 0, len(content.Array()))
	webSearchCalls := 0
	otherToolCalls := 0
	var loopErr error
	content.ForEach(func(_, block gjson.Result) bool {
		if block.Get("type").Str != "tool_use" {
			out = append(out, json.RawMessage(block.Raw))
			return true
		}
		if block.Get("name").Str != "web_search" {
			otherToolCalls++
			out = append(out, json.RawMessage(block.Raw))
			return true
		}
		webSearchCalls++
		toolUseID := block.Get("id").Str
		serverToolUseID := mapToolUseIDToServer(toolUseID)
		if serverToolUseID == "" {
			serverToolUseID = generateToolUseID()
		}
		serverToolUse, err := buildServerToolUseFromToolUse(block, serverToolUseID)
		if err != nil {
			loopErr = err
			return false
		}
		out = append(out, serverToolUse)

		toolInput := block.Get("input").Raw
		exaResp, err := h.callExa(ctx, json.RawMessage(toolInput), wsCtx)
		if err != nil {
			out = append(out, buildWebSearchToolResultError(serverToolUseID, "unavailable"))
			return true
		}
		out = append(out, buildWebSearchToolResult(serverToolUseID, exaResp))
		return true
	})
	if loopErr != nil {
		return nil, loopErr
	}

	encoded, err := json.Marshal(out)
	if err != nil {
		return nil, fmt.Errorf("transformWebSearchResponse: marshal content: %w", err)
	}
	body, err = sjson.SetRawBytes(body, "content", encoded)
	if err != nil {
		return nil, err
	}
	if webSearchCalls > 0 && otherToolCalls == 0 {
		stopReason := gjson.GetBytes(body, "stop_reason").Str
		if stopReason == "tool_use" {
			body, err = sjson.SetBytes(body, "stop_reason", "pause_turn")
			if err != nil {
				return nil, err
			}
		}
	}
	return body, nil
}

// buildServerToolUseFromToolUse converts an Anthropic tool_use block into the
// matching server_tool_use block. The whole input object is preserved so the
// client can replay the same Exa parameters in a follow-up request.
func buildServerToolUseFromToolUse(block gjson.Result, serverToolUseID string) (json.RawMessage, error) {
	fields := make(map[string]json.RawMessage)
	block.ForEach(func(key, val gjson.Result) bool {
		switch key.Str {
		case "type":
			fields["type"] = json.RawMessage(`"server_tool_use"`)
		case "id":
			encoded, err := json.Marshal(serverToolUseID)
			if err != nil {
				return false
			}
			fields["id"] = encoded
		default:
			fields[key.Str] = json.RawMessage(val.Raw)
		}
		return true
	})
	if _, ok := fields["type"]; !ok {
		fields["type"] = json.RawMessage(`"server_tool_use"`)
	}
	return json.Marshal(fields)
}
