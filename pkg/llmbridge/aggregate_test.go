package llmbridge

import (
	"context"
	"strings"
	"testing"

	"github.com/tidwall/gjson"
)

func TestStreamAggregationKind(t *testing.T) {
	cases := []struct {
		name        string
		format      Format
		contentType string
		want        AggregationKind
	}{
		{"openai chat sse", FormatOpenAIChatCompletions, "text/event-stream; charset=utf-8", StreamAggregationSSE},
		{"openai chat json", FormatOpenAIChatCompletions, "application/json", StreamAggregationNone},
		{"openai responses unsupported", FormatOpenAIResponses, "application/x-ndjson", StreamAggregationUnsupported},
		{"anthropic sse", FormatAnthropicMessages, "text/event-stream", StreamAggregationSSE},
		{"gemini sse", FormatGeminiStreamGenerateContent, "text/event-stream", StreamAggregationSSE},
		{"gemini jsonl", FormatGeminiStreamGenerateContent, "application/jsonl", StreamAggregationJSONL},
		{"gemini ndjson", FormatGeminiStreamGenerateContent, "application/x-ndjson", StreamAggregationJSONL},
		{"gemini stream json", FormatGeminiStreamGenerateContent, "application/json", StreamAggregationJSONL},
		{"gemini nonstream", FormatGeminiGenerateContent, "application/json", StreamAggregationNone},
		{"unknown", FormatUnknown, "text/event-stream", StreamAggregationNone},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			if got := StreamAggregationKind(tt.format, tt.contentType); got != tt.want {
				t.Fatalf("StreamAggregationKind(%s, %q) = %v, want %v", tt.format, tt.contentType, got, tt.want)
			}
		})
	}
}

func TestAggregateStreamOpenAIChatSSE(t *testing.T) {
	body := sse(
		`{"id":"chatcmpl-123","object":"chat.completion.chunk","created":1700000000,"model":"gpt-test","choices":[{"index":0,"delta":{"role":"assistant","content":"Hello ","tool_calls":[{"index":0,"id":"call_1","type":"function","function":{"name":"lookup","arguments":"{\"q\""}}]}}]}`,
		`{"id":"chatcmpl-123","object":"chat.completion.chunk","created":1700000000,"model":"gpt-test","choices":[{"index":0,"delta":{"content":"world","reasoning_content":"think","tool_calls":[{"index":0,"function":{"arguments":":\"x\"}"}}]},"finish_reason":"tool_calls"}],"usage":{"prompt_tokens":7,"completion_tokens":3,"total_tokens":10}}`,
		`[DONE]`,
	)

	got, err := AggregateStream(context.Background(), FormatOpenAIChatCompletions, "text/event-stream", []byte(body), mustProfile(t, FormatOpenAIChatCompletions))
	if err != nil {
		t.Fatal(err)
	}
	if object := gjson.GetBytes(got, "object").String(); object != "chat.completion" {
		t.Fatalf("object = %q, body=%s", object, got)
	}
	if content := gjson.GetBytes(got, "choices.0.message.content").String(); content != "Hello world" {
		t.Fatalf("content = %q, body=%s", content, got)
	}
	if reasoning := gjson.GetBytes(got, "choices.0.message.reasoning_content").String(); reasoning != "think" {
		t.Fatalf("reasoning_content = %q, body=%s", reasoning, got)
	}
	if args := gjson.GetBytes(got, "choices.0.message.tool_calls.0.function.arguments").String(); args != `{"q":"x"}` {
		t.Fatalf("tool arguments = %q, body=%s", args, got)
	}
	if total := gjson.GetBytes(got, "usage.total_tokens").Int(); total != 10 {
		t.Fatalf("total_tokens = %d, body=%s", total, got)
	}
}

func TestAggregateStreamOpenAIResponsesSSE(t *testing.T) {
	body := sse(
		`{"type":"response.created","sequence_number":0,"response":{"id":"resp_123","object":"response","created_at":1700000000,"model":"gpt-4o","status":"in_progress","output":[]}}`,
		`{"type":"response.output_item.added","sequence_number":1,"output_index":0,"item":{"id":"msg_1","type":"message","status":"in_progress","role":"assistant"}}`,
		`{"type":"response.content_part.added","sequence_number":2,"item_id":"msg_1","output_index":0,"content_index":0,"part":{"type":"output_text","text":""}}`,
		`{"type":"response.output_text.delta","sequence_number":3,"item_id":"msg_1","output_index":0,"content_index":0,"delta":"Hello"}`,
		`{"type":"response.output_text.delta","sequence_number":4,"item_id":"msg_1","output_index":0,"content_index":0,"delta":" there"}`,
		`{"type":"response.output_item.added","sequence_number":5,"output_index":1,"item":{"id":"rs_1","type":"reasoning","status":"in_progress","summary":[]}}`,
		`{"type":"response.reasoning_summary_part.added","sequence_number":6,"item_id":"rs_1","output_index":1,"summary_index":0,"part":{"type":"summary_text","text":""}}`,
		`{"type":"response.reasoning_summary_text.delta","sequence_number":7,"item_id":"rs_1","output_index":1,"summary_index":0,"delta":"Considered."}`,
		`{"type":"response.completed","sequence_number":8,"response":{"id":"resp_123","object":"response","created_at":1700000000,"model":"gpt-4o","status":"completed","output":[],"usage":{"input_tokens":10,"output_tokens":5,"total_tokens":15}}}`,
	)

	got, err := AggregateStream(context.Background(), FormatOpenAIResponses, "text/event-stream", []byte(body), mustProfile(t, FormatOpenAIResponses))
	if err != nil {
		t.Fatal(err)
	}
	if object := gjson.GetBytes(got, "object").String(); object != "response" {
		t.Fatalf("object = %q, body=%s", object, got)
	}
	if text := gjson.GetBytes(got, "output.0.content.0.text").String(); text != "Hello there" {
		t.Fatalf("output text = %q, body=%s", text, got)
	}
	if reasoning := gjson.GetBytes(got, "output.1.summary.0.text").String(); reasoning != "Considered." {
		t.Fatalf("reasoning summary = %q, body=%s", reasoning, got)
	}
	if total := gjson.GetBytes(got, "usage.total_tokens").Int(); total != 15 {
		t.Fatalf("total_tokens = %d, body=%s", total, got)
	}
}

func TestAggregateStreamAnthropicSSE(t *testing.T) {
	body := sse(
		`{"type":"message_start","message":{"id":"msg_123","type":"message","role":"assistant","content":[],"model":"claude-test","usage":{"input_tokens":11}}}`,
		`{"type":"content_block_start","index":0,"content_block":{"type":"thinking","thinking":""}}`,
		`{"type":"content_block_delta","index":0,"delta":{"type":"thinking_delta","thinking":"think"}}`,
		`{"type":"content_block_start","index":1,"content_block":{"type":"text","text":""}}`,
		`{"type":"content_block_delta","index":1,"delta":{"type":"text_delta","text":"Hello"}}`,
		`{"type":"content_block_delta","index":1,"delta":{"type":"text_delta","text":"!"}}`,
		`{"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"output_tokens":4}}`,
	)

	got, err := AggregateStream(context.Background(), FormatAnthropicMessages, "text/event-stream", []byte(body), mustProfile(t, FormatAnthropicMessages))
	if err != nil {
		t.Fatal(err)
	}
	if typ := gjson.GetBytes(got, "type").String(); typ != "message" {
		t.Fatalf("type = %q, body=%s", typ, got)
	}
	if thinking := gjson.GetBytes(got, "content.0.thinking").String(); thinking != "think" {
		t.Fatalf("thinking = %q, body=%s", thinking, got)
	}
	if text := gjson.GetBytes(got, "content.1.text").String(); text != "Hello!" {
		t.Fatalf("text = %q, body=%s", text, got)
	}
	if output := gjson.GetBytes(got, "usage.output_tokens").Int(); output != 4 {
		t.Fatalf("output_tokens = %d, body=%s", output, got)
	}
}

func TestAggregateStreamGeminiSSEAndJSONL(t *testing.T) {
	line1 := `{"responseId":"resp-1","modelVersion":"gemini-test","candidates":[{"index":0,"content":{"role":"model","parts":[{"text":"Thinking","thought":true}]}}]}`
	line2 := `{"responseId":"resp-1","modelVersion":"gemini-test","candidates":[{"index":0,"content":{"role":"model","parts":[{"text":"Answer"},{"functionCall":{"id":"call-1","name":"lookup","args":{"q":"x"}}}]},"finishReason":"STOP"}],"usageMetadata":{"promptTokenCount":8,"candidatesTokenCount":3,"totalTokenCount":11}}`

	for _, tt := range []struct {
		name        string
		contentType string
		body        string
	}{
		{name: "sse", contentType: "text/event-stream", body: sse(line1, line2)},
		{name: "jsonl", contentType: "application/x-ndjson", body: line1 + "\n" + line2 + "\n"},
		{name: "json content type jsonl body", contentType: "application/json", body: line1 + "\n" + line2 + "\n"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got, err := AggregateStream(context.Background(), FormatGeminiStreamGenerateContent, tt.contentType, []byte(tt.body), mustProfile(t, FormatGeminiStreamGenerateContent))
			if err != nil {
				t.Fatal(err)
			}
			if text := gjson.GetBytes(got, "candidates.0.content.parts.#(thought==true).text").String(); text != "Thinking" {
				t.Fatalf("thinking text = %q, body=%s", text, got)
			}
			var answer string
			for _, part := range gjson.GetBytes(got, "candidates.0.content.parts").Array() {
				if part.Get("thought").Bool() {
					continue
				}
				if text := part.Get("text").String(); text != "" {
					answer += text
				}
			}
			if answer != "Answer" {
				t.Fatalf("answer text = %q, body=%s", answer, got)
			}
			if call := gjson.GetBytes(got, "candidates.0.content.parts.#.functionCall.name").Array(); len(call) != 1 || call[0].String() != "lookup" {
				t.Fatalf("function call not preserved, body=%s", got)
			}
			if total := gjson.GetBytes(got, "usageMetadata.totalTokenCount").Int(); total != 11 {
				t.Fatalf("totalTokenCount = %d, body=%s", total, got)
			}
		})
	}
}

func TestAggregateStreamErrors(t *testing.T) {
	if _, err := AggregateStream(context.Background(), FormatOpenAIChatCompletions, "application/json", []byte(`{}`), mustProfile(t, FormatOpenAIChatCompletions)); err == nil || !strings.Contains(err.Error(), "not a stream response") {
		t.Fatalf("non-stream error = %v, want not a stream response", err)
	}
	if _, err := AggregateStream(context.Background(), FormatOpenAIResponses, "application/x-ndjson", []byte(`{}`), mustProfile(t, FormatOpenAIResponses)); err == nil || !strings.Contains(err.Error(), "unsupported stream content type") {
		t.Fatalf("unsupported error = %v, want unsupported stream content type", err)
	}
	if _, err := AggregateStream(context.Background(), FormatGeminiStreamGenerateContent, "application/jsonl", []byte(`{"ok":true}`+"\n"+`[]`+"\n"), mustProfile(t, FormatGeminiStreamGenerateContent)); err == nil || !strings.Contains(err.Error(), "expected JSON object") {
		t.Fatalf("malformed jsonl error = %v, want expected JSON object", err)
	}
	if _, err := AggregateStream(context.Background(), FormatGeminiStreamGenerateContent, "application/jsonl", []byte(`null`+"\n"), mustProfile(t, FormatGeminiStreamGenerateContent)); err == nil || !strings.Contains(err.Error(), "expected JSON object") {
		t.Fatalf("null jsonl error = %v, want expected JSON object", err)
	}
	if _, err := AggregateStream(context.Background(), FormatAnthropicMessages, "text/event-stream", []byte(":"), mustProfile(t, FormatAnthropicMessages)); err == nil || !strings.Contains(err.Error(), "empty stream chunks") {
		t.Fatalf("empty sse error = %v, want empty stream chunks", err)
	}
}

func sse(data ...string) string {
	var b strings.Builder
	for _, item := range data {
		b.WriteString("data: ")
		b.WriteString(item)
		b.WriteString("\n\n")
	}
	return b.String()
}
