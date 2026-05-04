package llmbridge

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/tidwall/gjson"
)

func TestBridgeRequestIdentity(t *testing.T) {
	body := []byte(`{"model":"x","messages":[{"role":"user","content":"hi"}],"max_tokens":1}`)
	got, ct, err := BridgeRequest(context.Background(), FormatAnthropicMessages, FormatAnthropicMessages, body, http.Header{}, "/v1/messages")
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
	got, ct, err := BridgeRequest(context.Background(), FormatAnthropicMessages, FormatOpenAIChatCompletions, body, http.Header{"Content-Type": []string{"application/json"}}, "/v1/messages")
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
	got, _, err := BridgeRequest(context.Background(), FormatGeminiStreamGenerateContent, FormatOpenAIChatCompletions, body, http.Header{"Content-Type": []string{"application/json"}}, SyntheticGeminiPath(FormatGeminiStreamGenerateContent, model))
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
