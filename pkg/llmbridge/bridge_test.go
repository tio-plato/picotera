package llmbridge

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/tidwall/gjson"
)

func mustProfile(t *testing.T, f Format) OutboundProfile {
	t.Helper()
	p, err := DefaultOutboundProfileForFormat(f)
	if err != nil {
		t.Fatalf("DefaultOutboundProfileForFormat(%s): %v", f, err)
	}
	return p
}

func TestBridgeRequestIdentity(t *testing.T) {
	body := []byte(`{"model":"x","messages":[{"role":"user","content":"hi"}],"max_tokens":1}`)
	got, ct, err := BridgeRequest(context.Background(), FormatAnthropicMessages, FormatAnthropicMessages, body, http.Header{}, "/v1/messages", mustProfile(t, FormatAnthropicMessages))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(body) {
		t.Errorf("identity bridge mutated body: %q", got)
	}
	if ct == "" {
		t.Error("identity bridge returned empty content-type")
	}
}

// TestBridgeRequestAnthropicToOpenAIChat checks the cross-format conversion
// produces a body that names the model and carries the user message text.
// Exhaustive shape verification lives in axonhub's own test suite; here we
// just confirm we wired the transformers together correctly.
func TestBridgeRequestAnthropicToOpenAIChat(t *testing.T) {
	body := []byte(`{"model":"claude-3-5-sonnet","messages":[{"role":"user","content":"ping"}],"max_tokens":16,"stream":true}`)
	got, ct, err := BridgeRequest(context.Background(), FormatAnthropicMessages, FormatOpenAIChatCompletions, body, http.Header{"Content-Type": []string{"application/json"}}, "/v1/messages", mustProfile(t, FormatOpenAIChatCompletions))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(ct, "application/json") {
		t.Errorf("unexpected content-type: %q", ct)
	}
	if m := gjson.GetBytes(got, "model").Str; m != "claude-3-5-sonnet" {
		t.Errorf("model not preserved: %q", m)
	}
	if s := gjson.GetBytes(got, "stream").Bool(); !s {
		t.Errorf("stream flag not preserved: %v", s)
	}
	if msg := gjson.GetBytes(got, "messages.0.content").Str; !strings.Contains(msg, "ping") {
		// OpenAI chat content can be a string OR a structured array, so
		// stringify the whole thing and look for the text.
		raw := gjson.GetBytes(got, "messages.0.content").String()
		if !strings.Contains(raw, "ping") {
			t.Errorf("user message text lost: %q", raw)
		}
	}
}

// TestBridgeRequestGeminiSourceUsesPath verifies that when the source is a
// Gemini route, parseSourceRequest uses the synthesized path to pick model
// and stream — the body itself has neither.
func TestBridgeRequestGeminiSourceUsesPath(t *testing.T) {
	body := []byte(`{"contents":[{"role":"user","parts":[{"text":"hi"}]}]}`)
	model := "gemini-2.5-pro"
	got, _, err := BridgeRequest(context.Background(), FormatGeminiStreamGenerateContent, FormatOpenAIChatCompletions, body, http.Header{"Content-Type": []string{"application/json"}}, SyntheticGeminiPath(FormatGeminiStreamGenerateContent, model), mustProfile(t, FormatOpenAIChatCompletions))
	if err != nil {
		t.Fatal(err)
	}
	if m := gjson.GetBytes(got, "model").Str; m != model {
		t.Errorf("model from path not propagated: %q", m)
	}
	if s := gjson.GetBytes(got, "stream").Bool(); !s {
		t.Errorf("stream flag from streamGenerateContent path not propagated")
	}
}

func TestBridgeRequestOpenRouterProfileUsesReasoningField(t *testing.T) {
	body := []byte(`{"model":"gpt","messages":[{"role":"assistant","content":"answer","reasoning_content":"thinking"},{"role":"user","content":"next"}]}`)
	got, _, err := BridgeRequest(context.Background(), FormatOpenAIChatCompletions, FormatOpenAIChatCompletions, body, http.Header{}, "/v1/chat/completions", OutboundProfile{Type: "openrouter"})
	if err != nil {
		t.Fatal(err)
	}
	if reasoning := gjson.GetBytes(got, "messages.0.reasoning").Str; reasoning != "" {
		// Identity path should not transform matching source/upstream formats.
		t.Errorf("identity openrouter bridge transformed body unexpectedly: %q", reasoning)
	}

	got, _, err = BridgeRequest(context.Background(), FormatAnthropicMessages, FormatOpenAIChatCompletions, []byte(`{"model":"claude","messages":[{"role":"assistant","content":[{"type":"thinking","thinking":"thinking","signature":"sig"},{"type":"text","text":"answer"}]},{"role":"user","content":"next"}],"max_tokens":16}`), http.Header{"Content-Type": []string{"application/json"}}, "/v1/messages", OutboundProfile{Type: "openrouter"})
	if err != nil {
		t.Fatal(err)
	}
	if reasoning := gjson.GetBytes(got, "messages.0.reasoning").Str; reasoning != "thinking" {
		t.Errorf("openrouter reasoning = %q, want thinking; body=%s", reasoning, got)
	}
	if gjson.GetBytes(got, "messages.0.reasoning_content").Exists() {
		t.Errorf("openrouter body still has reasoning_content: %s", got)
	}
}

func TestBridgeRequestDeepSeekProfileThinking(t *testing.T) {
	body := []byte(`{"model":"gpt","messages":[{"role":"user","content":"ping"}],"reasoning_effort":"none"}`)
	got, _, err := BridgeRequest(context.Background(), FormatOpenAIChatCompletions, FormatAnthropicMessages, body, http.Header{"Content-Type": []string{"application/json"}}, "/v1/chat/completions", OutboundProfile{Type: "deepseek"})
	if err == nil {
		t.Fatalf("deepseek profile with incompatible upstream format succeeded: %s", got)
	}

	got, _, err = BridgeRequest(context.Background(), FormatAnthropicMessages, FormatOpenAIChatCompletions, []byte(`{"model":"claude","messages":[{"role":"user","content":"ping"}],"max_tokens":16}`), http.Header{"Content-Type": []string{"application/json"}}, "/v1/messages", OutboundProfile{Type: "deepseek"})
	if err != nil {
		t.Fatal(err)
	}
	if thinkingType := gjson.GetBytes(got, "thinking.type").Str; thinkingType != "enabled" {
		t.Errorf("deepseek thinking.type = %q, want enabled; body=%s", thinkingType, got)
	}
}

func TestBridgeRequestFireworksProfileBuildsBody(t *testing.T) {
	body := []byte(`{"model":"claude","messages":[{"role":"user","content":"ping"}],"max_tokens":16}`)
	got, _, err := BridgeRequest(context.Background(), FormatAnthropicMessages, FormatOpenAIChatCompletions, body, http.Header{"Content-Type": []string{"application/json"}}, "/v1/messages", OutboundProfile{Type: "fireworks", Config: map[string]any{"base_url": "https://example.invalid/v1"}})
	if err != nil {
		t.Fatal(err)
	}
	if model := gjson.GetBytes(got, "model").Str; model != "claude" {
		t.Errorf("fireworks model = %q, want claude", model)
	}
}

func TestDefaultOutboundProfileForFormat(t *testing.T) {
	cases := []struct {
		format Format
		want   string
	}{
		{FormatAnthropicMessages, "anthropic"},
		{FormatOpenAIChatCompletions, "openai"},
		{FormatOpenAIResponses, "openaiResponses"},
		{FormatGeminiGenerateContent, "gemini"},
		{FormatGeminiStreamGenerateContent, "gemini"},
	}
	for _, tt := range cases {
		t.Run(tt.format.String(), func(t *testing.T) {
			got, err := DefaultOutboundProfileForFormat(tt.format)
			if err != nil {
				t.Fatalf("DefaultOutboundProfileForFormat: %v", err)
			}
			if got.Type != tt.want {
				t.Errorf("type = %q, want %q", got.Type, tt.want)
			}
			if len(got.Config) != 0 {
				t.Errorf("config = %+v, want empty", got.Config)
			}
		})
	}
	if _, err := DefaultOutboundProfileForFormat(FormatUnknown); err == nil {
		t.Fatalf("FormatUnknown should fail")
	}
}

func TestBridgeRequestProfileErrors(t *testing.T) {
	body := []byte(`{"model":"claude","messages":[{"role":"user","content":"ping"}],"max_tokens":16}`)
	headers := http.Header{"Content-Type": []string{"application/json"}}

	cases := []struct {
		name    string
		dst     Format
		profile OutboundProfile
		want    string
	}{
		{name: "incompatible", dst: FormatOpenAIResponses, profile: OutboundProfile{Type: "openrouter"}, want: "only compatible"},
		{name: "unknown config field", dst: FormatOpenAIChatCompletions, profile: OutboundProfile{Type: "fireworks", Config: map[string]any{"unknown": true}}, want: "unknown field"},
		{name: "unknown type", dst: FormatOpenAIChatCompletions, profile: OutboundProfile{Type: "madeup"}, want: "unsupported outbound type"},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			_, _, err := BridgeRequest(context.Background(), FormatAnthropicMessages, tt.dst, body, headers, "/v1/messages", tt.profile)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("err = %v, want containing %q", err, tt.want)
			}
		})
	}
}

func TestSyntheticGeminiPath(t *testing.T) {
	if p := SyntheticGeminiPath(FormatGeminiGenerateContent, "x"); !strings.HasSuffix(p, ":generateContent") {
		t.Errorf("non-stream path wrong: %q", p)
	}
	if p := SyntheticGeminiPath(FormatGeminiStreamGenerateContent, "x"); !strings.HasSuffix(p, ":streamGenerateContent") {
		t.Errorf("stream path wrong: %q", p)
	}
}

func TestEncodeSSEEvent(t *testing.T) {
	got := encodeSSEEvent(&httpclient.StreamEvent{Type: "message_start", Data: []byte(`{"hi":1}`)})
	want := "event: message_start\ndata: {\"hi\":1}\n\n"
	if string(got) != want {
		t.Errorf("encodeSSEEvent type case = %q, want %q", got, want)
	}

	got = encodeSSEEvent(&httpclient.StreamEvent{Data: []byte("[DONE]")})
	want = "data: [DONE]\n\n"
	if string(got) != want {
		t.Errorf("encodeSSEEvent done case = %q, want %q", got, want)
	}

	got = encodeSSEEvent(&httpclient.StreamEvent{Data: []byte("a\nb")})
	want = "data: a\ndata: b\n\n"
	if string(got) != want {
		t.Errorf("encodeSSEEvent multiline = %q, want %q", got, want)
	}
}
