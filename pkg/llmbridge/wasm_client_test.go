//go:build !wasip1

package llmbridge

import (
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
