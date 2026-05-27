//go:build !wasip1

package llmbridge

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWASMRuntimeConfigModes(t *testing.T) {
	cases := []string{"", wasmRuntimeInterpreter, wasmRuntimeCompiler}
	for _, mode := range cases {
		t.Run("mode="+mode, func(t *testing.T) {
			cfg, cache, err := wasmRuntimeConfig(mode)
			if err != nil {
				t.Fatalf("wasmRuntimeConfig(%q): %v", mode, err)
			}
			if cfg == nil {
				t.Fatalf("wasmRuntimeConfig(%q) returned nil config", mode)
			}
			if mode == "" || mode == wasmRuntimeCompiler {
				if cache == nil {
					t.Fatalf("compiler mode returned nil compilation cache")
				}
			} else if cache != nil {
				t.Fatalf("mode %q returned unexpected compilation cache", mode)
			}
		})
	}
}

func TestWASMRuntimeConfigRejectsUnknownMode(t *testing.T) {
	_, _, err := wasmRuntimeConfig("Compiler")
	if err == nil {
		t.Fatalf("unknown runtime mode should fail")
	}
	if !strings.Contains(err.Error(), "unsupported wasm runtime") {
		t.Fatalf("err = %v, want unsupported wasm runtime", err)
	}
}

func TestWASMRuntimeConfigUsesDiskCache(t *testing.T) {
	cacheDir := filepath.Join(t.TempDir(), "llmbridge.wasm.cache")
	_, cache, err := wasmRuntimeConfig(wasmRuntimeCompiler, cacheDir)
	if err != nil {
		t.Fatalf("wasmRuntimeConfig with disk cache: %v", err)
	}
	if cache == nil {
		t.Fatalf("compiler mode returned nil compilation cache")
	}
	t.Cleanup(func() { _ = cache.Close(t.Context()) })
	entries, err := os.ReadDir(cacheDir)
	if err != nil {
		t.Fatalf("read cache dir: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("cache dir entries = %d, want wazero version dir", len(entries))
	}
	if !strings.HasPrefix(entries[0].Name(), "wazero-") {
		t.Fatalf("cache dir entry = %q, want wazero-*", entries[0].Name())
	}
}

func TestDefaultCacheDir(t *testing.T) {
	if got := DefaultCacheDir("/tmp/llmbridge.wasm"); got != "/tmp/llmbridge.wasm.cache" {
		t.Fatalf("DefaultCacheDir = %q", got)
	}
	if got := DefaultCacheDir(""); got != "" {
		t.Fatalf("DefaultCacheDir empty = %q", got)
	}
}

func TestWASMStdioBuffer(t *testing.T) {
	buf := &wasmStdioBuffer{}
	if _, err := buf.Write([]byte("panic: bad\n")); err != nil {
		t.Fatalf("Write: %v", err)
	}
	if got := buf.Snapshot(); got != "panic: bad" {
		t.Fatalf("Snapshot = %q", got)
	}
	buf.Reset()
	if got := buf.Snapshot(); got != "" {
		t.Fatalf("Snapshot after reset = %q", got)
	}
	if _, err := buf.Write([]byte(strings.Repeat("x", wasmStdioLimit+7))); err != nil {
		t.Fatalf("Write large: %v", err)
	}
	got := buf.Snapshot()
	if !strings.Contains(got, "truncated 7 bytes") {
		t.Fatalf("Snapshot missing truncation marker: %q", got)
	}
}

func TestWASMDiagnosticsIncludesCapturedOutput(t *testing.T) {
	slot := &moduleSlot{stdio: &wasmStdioBuffer{}}
	if _, err := slot.stdio.Write([]byte("panic: runtime error: out of memory\n")); err != nil {
		t.Fatalf("Write: %v", err)
	}
	err := slot.withDiagnostics(errors.New("llmbridge: call llmbridge_bridge_request: wasm error: unreachable"))
	if err == nil {
		t.Fatalf("withDiagnostics returned nil")
	}
	got := err.Error()
	if !strings.Contains(got, "wasm stdout/stderr:") || !strings.Contains(got, "panic: runtime error: out of memory") {
		t.Fatalf("diagnostic error missing wasm stdio: %s", got)
	}
}

func TestWASMBridgeRequest(t *testing.T) {
	wasmPath := os.Getenv("PICOTERA_LLMBRIDGE_TEST_WASM")
	if wasmPath == "" {
		t.Skip("PICOTERA_LLMBRIDGE_TEST_WASM is not set")
	}

	bridge, err := New(t.Context(), Config{
		PoolSize:    1,
		WASMPath:    wasmPath,
		RuntimeMode: wasmRuntimeInterpreter,
	})
	if err != nil {
		t.Fatalf("New wasm bridge: %v", err)
	}
	t.Cleanup(func() { _ = bridge.Close(context.Background()) })

	body := []byte(`{"model":"claude","messages":[{"role":"user","content":"ping"}],"max_tokens":16}`)
	got, ct, err := bridge.BridgeRequest(t.Context(), FormatAnthropicMessages, FormatOpenAIChatCompletions, body, http.Header{"Content-Type": []string{"application/json"}}, "/v1/messages", OutboundProfile{Type: "openai"})
	if err != nil {
		t.Fatalf("BridgeRequest: %v", err)
	}
	if ct != "application/json" {
		t.Fatalf("content type = %q, want application/json", ct)
	}
	var out struct {
		Model    string `json:"model"`
		Messages []struct {
			Role    string `json:"role"`
			Content any    `json:"content"`
		} `json:"messages"`
	}
	if err := json.Unmarshal(got, &out); err != nil {
		t.Fatalf("decode output: %v\n%s", err, got)
	}
	if out.Model != "claude" || len(out.Messages) != 1 || out.Messages[0].Role != "user" {
		t.Fatalf("unexpected bridge output: %s", got)
	}
}
