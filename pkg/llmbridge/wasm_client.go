//go:build !wasip1

package llmbridge

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"sync"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/api"
	"github.com/tetratelabs/wazero/imports/wasi_snapshot_preview1"
)

const wasmABIVersion = 1

const (
	wasmRuntimeInterpreter = "interpreter"
	wasmRuntimeCompiler    = "compiler"
)

const (
	streamStatusOK uint32 = iota
	streamStatusEOF
	streamStatusError
)

const wasmStdioLimit = 64 * 1024

type wasmBridge struct {
	wasmBytes []byte
	runtime   wazero.RuntimeConfig
	cache     wazero.CompilationCache
	slots     chan *moduleSlot
}

type moduleSlot struct {
	runtime  wazero.Runtime
	compiled wazero.CompiledModule
	module   api.Module
	mu       sync.Mutex
	stdio    *wasmStdioBuffer

	upstream io.ReadCloser
	writer   *io.PipeWriter
}

func newWASMBridge(ctx context.Context, cfg Config) (Bridge, error) {
	if cfg.WASMPath == "" {
		return disabledBridge{}, nil
	}
	if cfg.PoolSize <= 0 {
		return nil, fmt.Errorf("llmbridge: wasm pool size must be positive")
	}
	cacheDir := cfg.CacheDir
	if cacheDir == "" {
		cacheDir = DefaultCacheDir(cfg.WASMPath)
	}
	runtimeConfig, cache, err := wasmRuntimeConfig(cfg.RuntimeMode, cacheDir)
	if err != nil {
		return nil, err
	}
	if cache == nil && cfg.CacheDir != "" {
		return nil, fmt.Errorf("llmbridge: wasm cache dir requires compiler runtime")
	}
	wasmBytes, err := os.ReadFile(cfg.WASMPath)
	if err != nil {
		if cache != nil {
			_ = cache.Close(ctx)
		}
		return nil, fmt.Errorf("llmbridge: read wasm module: %w", err)
	}
	b := &wasmBridge{
		wasmBytes: wasmBytes,
		runtime:   runtimeConfig,
		cache:     cache,
		slots:     make(chan *moduleSlot, cfg.PoolSize),
	}
	for i := 0; i < cfg.PoolSize; i++ {
		slot, err := b.instantiateSlot(ctx)
		if err != nil {
			_ = b.Close(ctx)
			return nil, err
		}
		b.slots <- slot
	}
	return b, nil
}

func (b *wasmBridge) instantiateSlot(ctx context.Context) (*moduleSlot, error) {
	rt := wazero.NewRuntimeWithConfig(ctx, b.runtime)
	slot := &moduleSlot{runtime: rt, stdio: &wasmStdioBuffer{}}
	if _, err := wasi_snapshot_preview1.Instantiate(ctx, rt); err != nil {
		_ = rt.Close(ctx)
		return nil, fmt.Errorf("llmbridge: instantiate wasi: %w", err)
	}
	compiled, err := rt.CompileModule(ctx, b.wasmBytes)
	if err != nil {
		_ = rt.Close(ctx)
		return nil, fmt.Errorf("llmbridge: compile wasm module: %w", err)
	}
	slot.compiled = compiled
	hostBuilder := rt.NewHostModuleBuilder("picotera_llmbridge_host")
	hostBuilder.NewFunctionBuilder().WithGoModuleFunction(api.GoModuleFunc(func(ctx context.Context, mod api.Module, stack []uint64) {
		ptr := uint32(stack[0])
		capacity := uint32(stack[1])
		stack[0] = slot.hostStreamRead(mod, ptr, capacity)
	}), []api.ValueType{api.ValueTypeI32, api.ValueTypeI32}, []api.ValueType{api.ValueTypeI64}).Export("llmbridge_stream_read")
	hostBuilder.NewFunctionBuilder().WithGoModuleFunction(api.GoModuleFunc(func(ctx context.Context, mod api.Module, stack []uint64) {
		ptr := uint32(stack[0])
		n := uint32(stack[1])
		stack[0] = uint64(slot.hostStreamWrite(mod, ptr, n))
	}), []api.ValueType{api.ValueTypeI32, api.ValueTypeI32}, []api.ValueType{api.ValueTypeI32}).Export("llmbridge_stream_write")
	hostModule, err := hostBuilder.Instantiate(ctx)
	if err != nil {
		return nil, fmt.Errorf("llmbridge: instantiate host imports: %w", err)
	}
	defer func() {
		if slot.module == nil {
			_ = hostModule.Close(ctx)
		}
	}()
	mod, err := rt.InstantiateModule(ctx, compiled, wazero.NewModuleConfig().
		WithName("llmbridge").
		WithStdout(slot.stdio).
		WithStderr(slot.stdio))
	if err != nil {
		return nil, fmt.Errorf("llmbridge: instantiate wasm module: %w", err)
	}
	slot.module = mod
	if initFn := mod.ExportedFunction("_initialize"); initFn != nil {
		if _, err := initFn.Call(ctx); err != nil {
			_ = mod.Close(ctx)
			return nil, fmt.Errorf("llmbridge: initialize wasm module: %w", err)
		}
	}
	versionFn := mod.ExportedFunction("llmbridge_abi_version")
	if versionFn == nil {
		_ = mod.Close(ctx)
		return nil, fmt.Errorf("llmbridge: wasm module missing llmbridge_abi_version")
	}
	version, err := versionFn.Call(ctx)
	if err != nil {
		_ = mod.Close(ctx)
		return nil, fmt.Errorf("llmbridge: read wasm ABI version: %w", err)
	}
	if len(version) != 1 || uint32(version[0]) != wasmABIVersion {
		_ = mod.Close(ctx)
		got := uint32(0)
		if len(version) == 1 {
			got = uint32(version[0])
		}
		return nil, fmt.Errorf("llmbridge: wasm ABI version mismatch: got %d want %d", got, wasmABIVersion)
	}
	return slot, nil
}

func wasmRuntimeConfig(mode string, cacheDir ...string) (wazero.RuntimeConfig, wazero.CompilationCache, error) {
	switch mode {
	case wasmRuntimeInterpreter:
		return wazero.NewRuntimeConfigInterpreter().WithDebugInfoEnabled(true), nil, nil
	case "", wasmRuntimeCompiler:
		cache, err := newWASMCompilationCache(cacheDir...)
		if err != nil {
			return nil, nil, err
		}
		return wazero.NewRuntimeConfigCompiler().
			WithDebugInfoEnabled(true).
			WithCompilationCache(cache), cache, nil
	default:
		return nil, nil, fmt.Errorf("llmbridge: unsupported wasm runtime %q", mode)
	}
}

func newWASMCompilationCache(cacheDir ...string) (wazero.CompilationCache, error) {
	if len(cacheDir) == 0 || cacheDir[0] == "" {
		return wazero.NewCompilationCache(), nil
	}
	cache, err := wazero.NewCompilationCacheWithDir(cacheDir[0])
	if err != nil {
		return nil, fmt.Errorf("llmbridge: create wasm compilation cache: %w", err)
	}
	return cache, nil
}

func DefaultCacheDir(wasmPath string) string {
	if wasmPath == "" {
		return ""
	}
	return filepath.Clean(wasmPath) + ".cache"
}

func Precompile(ctx context.Context, cfg Config) error {
	if cfg.WASMPath == "" {
		return fmt.Errorf("llmbridge: wasm path is required")
	}
	if cfg.RuntimeMode == wasmRuntimeInterpreter {
		return fmt.Errorf("llmbridge: precompile requires compiler runtime")
	}
	cacheDir := cfg.CacheDir
	if cacheDir == "" {
		cacheDir = DefaultCacheDir(cfg.WASMPath)
	}
	runtimeConfig, cache, err := wasmRuntimeConfig(cfg.RuntimeMode, cacheDir)
	if err != nil {
		return err
	}
	defer cache.Close(ctx)
	wasmBytes, err := os.ReadFile(cfg.WASMPath)
	if err != nil {
		return fmt.Errorf("llmbridge: read wasm module: %w", err)
	}
	rt := wazero.NewRuntimeWithConfig(ctx, runtimeConfig)
	defer rt.Close(ctx)
	compiled, err := rt.CompileModule(ctx, wasmBytes)
	if err != nil {
		return fmt.Errorf("llmbridge: compile wasm module: %w", err)
	}
	return compiled.Close(ctx)
}

func (b *wasmBridge) Enabled() bool {
	return true
}

func (b *wasmBridge) Close(ctx context.Context) error {
	var firstErr error
	for {
		select {
		case slot := <-b.slots:
			if err := slot.runtime.Close(ctx); err != nil && firstErr == nil {
				firstErr = err
			}
		default:
			if b.cache != nil {
				if err := b.cache.Close(ctx); err != nil && firstErr == nil {
					firstErr = err
				}
			}
			return firstErr
		}
	}
}

func (b *wasmBridge) BridgeRequest(ctx context.Context, src, dst Format, body []byte, headers http.Header, pendingURL string, profile OutboundProfile) ([]byte, string, error) {
	if src == FormatUnknown || dst == FormatUnknown {
		return nil, "", fmt.Errorf("llmbridge: bridge with unknown format (src=%s dst=%s)", src, dst)
	}
	if src == dst {
		return body, contentTypeOrDefault(headers), nil
	}
	var resp operationResponse
	if err := b.call(ctx, "llmbridge_bridge_request", bridgeRequestEnvelope{
		Src: src, Dst: dst, Body: body, Headers: map[string][]string(headers), PendingURL: pendingURL, Profile: normalizedProfile(profile),
	}, &resp); err != nil {
		return nil, "", err
	}
	return resp.Body, resp.ContentType, nil
}

func (b *wasmBridge) BridgeNonStream(ctx context.Context, src, upstream Format, upstreamBody []byte, upstreamHeaders http.Header, profile OutboundProfile) ([]byte, string, error) {
	if src == FormatUnknown || upstream == FormatUnknown {
		return nil, "", fmt.Errorf("llmbridge: bridge non-stream with unknown format (src=%s upstream=%s)", src, upstream)
	}
	if src == upstream {
		return upstreamBody, contentTypeOrDefault(upstreamHeaders), nil
	}
	var resp operationResponse
	if err := b.call(ctx, "llmbridge_bridge_non_stream", bridgeNonStreamEnvelope{
		Src: src, Upstream: upstream, Body: upstreamBody, Headers: map[string][]string(upstreamHeaders), Profile: normalizedProfile(profile),
	}, &resp); err != nil {
		return nil, "", err
	}
	return resp.Body, resp.ContentType, nil
}

func (b *wasmBridge) AggregateStream(ctx context.Context, format Format, contentType string, body []byte, profile OutboundProfile) ([]byte, error) {
	var resp operationResponse
	if err := b.call(ctx, "llmbridge_aggregate_stream", aggregateStreamEnvelope{
		Format: format, ContentType: contentType, Body: body, Profile: normalizedProfile(profile),
	}, &resp); err != nil {
		return nil, err
	}
	return resp.Body, nil
}

func (b *wasmBridge) BridgeStream(ctx context.Context, src, upstream Format, upstreamBody io.ReadCloser, upstreamCT string, profile OutboundProfile) (io.ReadCloser, error) {
	if src == FormatUnknown || upstream == FormatUnknown {
		_ = upstreamBody.Close()
		return nil, fmt.Errorf("llmbridge: bridge stream with unknown format (src=%s upstream=%s)", src, upstream)
	}
	if src == upstream {
		return upstreamBody, nil
	}
	slot, err := b.checkout(ctx)
	if err != nil {
		_ = upstreamBody.Close()
		return nil, err
	}
	pr, pw := io.Pipe()
	slot.upstream = upstreamBody
	slot.writer = pw
	var resp operationResponse
	if err := slot.call(ctx, "llmbridge_bridge_stream_open", bridgeStreamOpenEnvelope{
		Src: src, Upstream: upstream, ContentType: upstreamCT, Profile: normalizedProfile(profile),
	}, &resp); err != nil {
		slot.upstream = nil
		slot.writer = nil
		b.release(slot)
		_ = upstreamBody.Close()
		_ = pw.Close()
		_ = pr.Close()
		return nil, err
	}
	go func() {
		var pumpResp operationResponse
		err := slot.call(context.Background(), "llmbridge_bridge_stream_pump", nil, &pumpResp, uint64(resp.StreamID))
		var closeResp operationResponse
		if closeErr := slot.call(context.Background(), "llmbridge_bridge_stream_close", nil, &closeResp, uint64(resp.StreamID)); err == nil {
			err = closeErr
		}
		_ = upstreamBody.Close()
		_ = pw.CloseWithError(err)
		slot.upstream = nil
		slot.writer = nil
		b.release(slot)
	}()
	return &wasmStreamReadCloser{ReadCloser: pr, upstream: upstreamBody, writer: pw}, nil
}

func (b *wasmBridge) call(ctx context.Context, export string, input any, output *operationResponse, params ...uint64) error {
	slot, err := b.checkout(ctx)
	if err != nil {
		return err
	}
	defer b.release(slot)
	return slot.call(ctx, export, input, output, params...)
}

func (b *wasmBridge) checkout(ctx context.Context) (*moduleSlot, error) {
	select {
	case slot := <-b.slots:
		slot.mu.Lock()
		return slot, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (b *wasmBridge) release(slot *moduleSlot) {
	slot.mu.Unlock()
	b.slots <- slot
}

func (s *moduleSlot) call(ctx context.Context, export string, input any, output *operationResponse, params ...uint64) error {
	if s.stdio != nil {
		s.stdio.Reset()
	}
	fn := s.module.ExportedFunction(export)
	if fn == nil {
		return fmt.Errorf("llmbridge: wasm module missing %s", export)
	}
	var ptr uint32
	if input != nil {
		raw, err := json.Marshal(input)
		if err != nil {
			return fmt.Errorf("llmbridge: encode wasm input: %w", err)
		}
		if len(raw) > math.MaxUint32 {
			return fmt.Errorf("llmbridge: wasm input exceeds uint32 length")
		}
		ptr, err = s.writeInput(ctx, export, raw)
		if err != nil {
			return s.withDiagnostics(err)
		}
		defer s.free(ctx, ptr)
		params = []uint64{uint64(ptr), uint64(len(raw))}
	}
	result, err := fn.Call(ctx, params...)
	if err != nil {
		return s.withDiagnostics(fmt.Errorf("llmbridge: call %s: %w", export, err))
	}
	if len(result) != 1 {
		return s.withDiagnostics(fmt.Errorf("llmbridge: call %s returned %d values", export, len(result)))
	}
	raw, err := s.readOutput(ctx, export, result[0])
	if err != nil {
		return s.withDiagnostics(err)
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(output); err != nil {
		return s.withDiagnostics(fmt.Errorf("llmbridge: decode wasm output from %s: %w", export, err))
	}
	var extra any
	if err := dec.Decode(&extra); err != io.EOF {
		if err == nil {
			return s.withDiagnostics(fmt.Errorf("llmbridge: decode wasm output from %s: multiple JSON values", export))
		}
		return s.withDiagnostics(fmt.Errorf("llmbridge: decode wasm output from %s: %w", export, err))
	}
	if !output.OK {
		if output.Error == "" {
			output.Error = "llmbridge: wasm operation failed"
		}
		return s.withDiagnostics(fmt.Errorf("%s", output.Error))
	}
	return nil
}

func (s *moduleSlot) writeInput(ctx context.Context, export string, raw []byte) (uint32, error) {
	alloc := s.module.ExportedFunction("llmbridge_alloc")
	if alloc == nil {
		return 0, fmt.Errorf("llmbridge: wasm module missing llmbridge_alloc")
	}
	result, err := alloc.Call(ctx, uint64(len(raw)))
	if err != nil {
		return 0, fmt.Errorf("llmbridge: allocate guest memory for %s input (%d bytes, memory=%d bytes): %w", export, len(raw), wasmMemorySize(s.module), err)
	}
	if len(result) != 1 {
		return 0, fmt.Errorf("llmbridge: allocate guest memory returned %d values", len(result))
	}
	ptr := uint32(result[0])
	if !s.module.Memory().Write(ptr, raw) {
		_ = s.free(ctx, ptr)
		return 0, fmt.Errorf("llmbridge: write %s input to guest memory out of range (ptr=%d len=%d memory=%d bytes)", export, ptr, len(raw), wasmMemorySize(s.module))
	}
	return ptr, nil
}

func (s *moduleSlot) readOutput(ctx context.Context, export string, packed uint64) ([]byte, error) {
	ptr := uint32(packed >> 32)
	n := uint32(packed)
	defer s.free(ctx, ptr)
	raw, ok := s.module.Memory().Read(ptr, n)
	if !ok {
		return nil, fmt.Errorf("llmbridge: read %s output from guest memory out of range (ptr=%d len=%d memory=%d bytes)", export, ptr, n, wasmMemorySize(s.module))
	}
	out := make([]byte, len(raw))
	copy(out, raw)
	return out, nil
}

func (s *moduleSlot) withDiagnostics(err error) error {
	if err == nil {
		return nil
	}
	if s.stdio == nil {
		return err
	}
	diagnostics := s.stdio.Snapshot()
	if diagnostics == "" {
		return err
	}
	return fmt.Errorf("%w\nwasm stdout/stderr:\n%s", err, diagnostics)
}

func (s *moduleSlot) free(ctx context.Context, ptr uint32) error {
	free := s.module.ExportedFunction("llmbridge_free")
	if free == nil {
		return fmt.Errorf("llmbridge: wasm module missing llmbridge_free")
	}
	_, err := free.Call(ctx, uint64(ptr))
	return err
}

func (s *moduleSlot) hostStreamRead(mod api.Module, ptr, capacity uint32) uint64 {
	if s.upstream == nil {
		return uint64(streamStatusError) << 32
	}
	buf, ok := mod.Memory().Read(ptr, capacity)
	if !ok {
		return uint64(streamStatusError) << 32
	}
	n, err := s.upstream.Read(buf)
	if err == nil {
		return uint64(n)
	}
	if err == io.EOF {
		return uint64(streamStatusEOF)<<32 | uint64(n)
	}
	if n > 0 {
		return uint64(n)
	}
	return uint64(streamStatusError) << 32
}

func (s *moduleSlot) hostStreamWrite(mod api.Module, ptr, n uint32) uint32 {
	if s.writer == nil {
		return streamStatusError
	}
	buf, ok := mod.Memory().Read(ptr, n)
	if !ok {
		return streamStatusError
	}
	if _, err := s.writer.Write(buf); err != nil {
		return streamStatusError
	}
	return streamStatusOK
}

type wasmStreamReadCloser struct {
	io.ReadCloser
	upstream io.Closer
	writer   io.Closer
}

func (r *wasmStreamReadCloser) Close() error {
	err := r.ReadCloser.Close()
	_ = r.upstream.Close()
	_ = r.writer.Close()
	return err
}

func normalizedProfile(profile OutboundProfile) OutboundProfile {
	if profile.Config == nil {
		profile.Config = map[string]any{}
	}
	return profile
}

func wasmMemorySize(mod api.Module) uint32 {
	mem := mod.Memory()
	if mem == nil {
		return 0
	}
	return mem.Size()
}

type wasmStdioBuffer struct {
	mu        sync.Mutex
	data      []byte
	truncated int
}

func (b *wasmStdioBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	available := wasmStdioLimit - len(b.data)
	if available > 0 {
		n := len(p)
		if n > available {
			n = available
		}
		b.data = append(b.data, p[:n]...)
		b.truncated += len(p) - n
	} else {
		b.truncated += len(p)
	}
	return len(p), nil
}

func (b *wasmStdioBuffer) Reset() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.data = b.data[:0]
	b.truncated = 0
}

func (b *wasmStdioBuffer) Snapshot() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	if len(b.data) == 0 && b.truncated == 0 {
		return ""
	}
	out := string(bytes.TrimRight(b.data, "\n"))
	if b.truncated > 0 {
		if out != "" {
			out += "\n"
		}
		out += fmt.Sprintf("... truncated %d bytes", b.truncated)
	}
	return out
}

type bridgeRequestEnvelope struct {
	Src        Format              `json:"src"`
	Dst        Format              `json:"dst"`
	Body       []byte              `json:"body"`
	Headers    map[string][]string `json:"headers"`
	PendingURL string              `json:"pendingURL"`
	Profile    OutboundProfile     `json:"profile"`
}

type bridgeNonStreamEnvelope struct {
	Src      Format              `json:"src"`
	Upstream Format              `json:"upstream"`
	Body     []byte              `json:"body"`
	Headers  map[string][]string `json:"headers"`
	Profile  OutboundProfile     `json:"profile"`
}

type bridgeStreamOpenEnvelope struct {
	Src         Format          `json:"src"`
	Upstream    Format          `json:"upstream"`
	ContentType string          `json:"contentType"`
	Profile     OutboundProfile `json:"profile"`
}

type aggregateStreamEnvelope struct {
	Format      Format          `json:"format"`
	ContentType string          `json:"contentType"`
	Body        []byte          `json:"body"`
	Profile     OutboundProfile `json:"profile"`
}

type operationResponse struct {
	OK          bool   `json:"ok"`
	Error       string `json:"error,omitempty"`
	Body        []byte `json:"body,omitempty"`
	ContentType string `json:"contentType,omitempty"`
	StreamID    uint32 `json:"streamID,omitempty"`
}
