//go:build !wasip1

package llmbridge

import (
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
