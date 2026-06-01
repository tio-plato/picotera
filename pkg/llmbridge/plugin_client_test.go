package llmbridge

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/tidwall/gjson"
)

var (
	testPluginPath string
	testPluginErr  error
	testPluginOnce sync.Once
)

func buildTestPlugin(t *testing.T) string {
	t.Helper()
	testPluginOnce.Do(func() {
		dir, err := os.MkdirTemp("", "picotera-llmbridge-plugin-test-*")
		if err != nil {
			testPluginErr = err
			return
		}
		testPluginPath = filepath.Join(dir, "picotera-llmbridge-plugin")
		cmd := exec.Command("go", "build", "-o", testPluginPath, "../../cmd/picotera-llmbridge-plugin")
		cmd.Dir = "."
		out, err := cmd.CombinedOutput()
		if err != nil {
			testPluginErr = commandError{err: err, output: out}
		}
	})
	if testPluginErr != nil {
		t.Fatalf("build plugin: %v", testPluginErr)
	}
	return testPluginPath
}

func TestNewDisabledBridge(t *testing.T) {
	bridge, err := New(t.Context(), Config{})
	if err != nil {
		t.Fatalf("New disabled bridge: %v", err)
	}
	if bridge.Enabled() {
		t.Fatalf("disabled bridge reports enabled")
	}

	body := []byte(`{"ok":true}`)
	got, ct, err := bridge.BridgeRequest(t.Context(), FormatAnthropicMessages, FormatAnthropicMessages, body, http.Header{"Content-Type": []string{"application/json"}}, "/v1/messages", OutboundProfile{})
	if err != nil {
		t.Fatalf("identity BridgeRequest on disabled bridge: %v", err)
	}
	if string(got) != string(body) || ct != "application/json" {
		t.Fatalf("identity BridgeRequest = %q %q", got, ct)
	}

	_, _, err = bridge.BridgeRequest(t.Context(), FormatAnthropicMessages, FormatOpenAIChatCompletions, body, http.Header{}, "/v1/messages", OutboundProfile{})
	if err == nil || !strings.Contains(err.Error(), "plugin is not configured") {
		t.Fatalf("cross-format disabled error = %v", err)
	}
}

func TestNewPluginMissingPathFails(t *testing.T) {
	_, err := New(t.Context(), Config{
		PluginPath:         filepath.Join(t.TempDir(), "missing-plugin"),
		PluginStartTimeout: time.Second,
	})
	if err == nil {
		t.Fatalf("missing plugin path should fail")
	}
}

func TestPluginBridgeRequestAnthropicToOpenAIChat(t *testing.T) {
	bridge := newTestPluginBridge(t)
	body := []byte(`{"model":"claude-3-5-sonnet","messages":[{"role":"user","content":"ping"}],"max_tokens":16,"stream":true}`)
	got, ct, err := bridge.BridgeRequest(t.Context(), FormatAnthropicMessages, FormatOpenAIChatCompletions, body, http.Header{"Content-Type": []string{"application/json"}}, "/v1/messages", OutboundProfile{Type: "openai"})
	if err != nil {
		t.Fatalf("BridgeRequest: %v", err)
	}
	if !strings.Contains(ct, "application/json") {
		t.Fatalf("content-type = %q", ct)
	}
	if model := gjson.GetBytes(got, "model").Str; model != "claude-3-5-sonnet" {
		t.Fatalf("model = %q body=%s", model, got)
	}
	if text := gjson.GetBytes(got, "messages.0.content").String(); !strings.Contains(text, "ping") {
		t.Fatalf("message content lost: %s", got)
	}
}

func TestPluginBridgeNonStreamOpenAIChatToAnthropic(t *testing.T) {
	bridge := newTestPluginBridge(t)
	body := []byte(`{"id":"chatcmpl_1","object":"chat.completion","created":1700000000,"model":"gpt","choices":[{"index":0,"message":{"role":"assistant","content":"pong"},"finish_reason":"stop"}],"usage":{"prompt_tokens":1,"completion_tokens":2,"total_tokens":3}}`)
	got, ct, err := bridge.BridgeNonStream(t.Context(), FormatAnthropicMessages, FormatOpenAIChatCompletions, body, http.Header{"Content-Type": []string{"application/json"}}, OutboundProfile{Type: "openai"})
	if err != nil {
		t.Fatalf("BridgeNonStream: %v", err)
	}
	if !strings.Contains(ct, "application/json") {
		t.Fatalf("content-type = %q", ct)
	}
	if responseType := gjson.GetBytes(got, "type").Str; responseType != "message" {
		t.Fatalf("type = %q body=%s", responseType, got)
	}
	if text := gjson.GetBytes(got, "content.0.text").Str; text != "pong" {
		t.Fatalf("text = %q body=%s", text, got)
	}
}

func TestPluginBridgeIdentityStreamPassthrough(t *testing.T) {
	bridge := newTestPluginBridge(t)
	body := io.NopCloser(bytes.NewBufferString("data: one\n\n"))
	got, err := bridge.BridgeStream(t.Context(), FormatOpenAIChatCompletions, FormatOpenAIChatCompletions, body, "text/event-stream", OutboundProfile{})
	if err != nil {
		t.Fatalf("BridgeStream identity: %v", err)
	}
	out, err := io.ReadAll(got)
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	if string(out) != "data: one\n\n" {
		t.Fatalf("identity stream = %q", out)
	}
}

func TestPluginBridgeSelfHealsAfterCrash(t *testing.T) {
	bridge := newTestPluginBridge(t)
	pb, ok := bridge.(*pluginBridge)
	if !ok {
		t.Fatalf("bridge is %T, want *pluginBridge", bridge)
	}

	// Simulate the subprocess dying: kill it out from under the bridge.
	old := pb.client
	old.Kill()
	if !old.Exited() {
		t.Fatalf("killed client should report Exited")
	}

	body := []byte(`{"model":"claude-3-5-sonnet","messages":[{"role":"user","content":"ping"}],"max_tokens":16}`)
	got, _, err := bridge.BridgeRequest(t.Context(), FormatAnthropicMessages, FormatOpenAIChatCompletions, body, http.Header{"Content-Type": []string{"application/json"}}, "/v1/messages", OutboundProfile{Type: "openai"})
	if err != nil {
		t.Fatalf("BridgeRequest after crash should self-heal: %v", err)
	}
	if model := gjson.GetBytes(got, "model").Str; model != "claude-3-5-sonnet" {
		t.Fatalf("model = %q body=%s", model, got)
	}
	if pb.client == old {
		t.Fatalf("bridge should have restarted with a fresh client")
	}
}

func TestPluginBridgeStreamSelfHealsAfterCrash(t *testing.T) {
	bridge := newTestPluginBridge(t)
	pb, ok := bridge.(*pluginBridge)
	if !ok {
		t.Fatalf("bridge is %T, want *pluginBridge", bridge)
	}

	old := pb.client
	old.Kill()

	upstream := io.NopCloser(bytes.NewBufferString("data: [DONE]\n\n"))
	stream, err := bridge.BridgeStream(t.Context(), FormatAnthropicMessages, FormatOpenAIChatCompletions, upstream, "text/event-stream", OutboundProfile{Type: "openai"})
	if err != nil {
		t.Fatalf("BridgeStream after crash should self-heal: %v", err)
	}
	if _, err := io.ReadAll(stream); err != nil {
		t.Fatalf("ReadAll bridged stream: %v", err)
	}
	if err := stream.Close(); err != nil {
		t.Fatalf("Close bridged stream: %v", err)
	}
	if pb.client == old {
		t.Fatalf("bridge should have restarted with a fresh client")
	}
}

func TestProfileConfigJSONValidation(t *testing.T) {
	_, err := profileToProto(OutboundProfile{Type: "openai", Config: map[string]any{"bad": func() {}}})
	if err == nil || !strings.Contains(err.Error(), "encode outbound profile config") {
		t.Fatalf("profileToProto error = %v", err)
	}
	_, err = profileFromProto(&OutboundProfileMessage{Type: "openai", ConfigJson: []byte(`[]`)})
	if err == nil || !strings.Contains(err.Error(), "expected JSON object") {
		t.Fatalf("profileFromProto array error = %v", err)
	}
	_, err = profileFromProto(&OutboundProfileMessage{Type: "openai", ConfigJson: []byte(`{} {}`)})
	if err == nil || !strings.Contains(err.Error(), "multiple JSON values") {
		t.Fatalf("profileFromProto multi error = %v", err)
	}
}

func newTestPluginBridge(t *testing.T) Bridge {
	t.Helper()
	bridge, err := New(t.Context(), Config{
		PluginPath:         buildTestPlugin(t),
		PluginStartTimeout: 10 * time.Second,
	})
	if err != nil {
		t.Fatalf("New plugin bridge: %v", err)
	}
	t.Cleanup(func() { _ = bridge.Close(context.Background()) })
	return bridge
}

type commandError struct {
	err    error
	output []byte
}

func (e commandError) Error() string {
	return e.err.Error() + ": " + string(e.output)
}
