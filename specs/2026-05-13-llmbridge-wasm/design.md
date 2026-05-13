# Design: llmbridge WASM Isolation

## Goal

`pkg/llmbridge` currently imports `github.com/looplj/axonhub/llm`, whose LLM transformer subtree is LGPL-3.0. The change moves all axonhub imports and transformation code into a separately built WebAssembly module. The Go server loads that module at startup and calls it through a narrow ABI.

The `picotera` Go binary keeps only Picotera-owned bridge types and a host-side adapter. It does not import `github.com/looplj/axonhub/llm` or any axonhub transformer package, and it never embeds `llmbridge.wasm`.

Two Docker/runtime variants exist:

- default runtime: contains only the shared `picotera` binary. It enables the bridge only when `PICOTERA_LLMBRIDGE_WASM_PATH` points to an external module supplied by the operator.
- LGPL runtime: reuses the same shared `picotera` binary layer, adds `llmbridge.wasm` as a separate file, and sets `PICOTERA_LLMBRIDGE_WASM_PATH` to that file. Operators can replace it by mounting another file and overriding the env var.

## Build Shape

Add a WASI reactor command under `cmd/llmbridge-wasm/`. It imports the isolated implementation package and exports fixed ABI functions with `//go:wasmexport`.

Build command:

```bash
GOOS=wasip1 GOARCH=wasm go build -buildmode=c-shared -o dist/llmbridge.wasm ./cmd/llmbridge-wasm
```

`-buildmode=c-shared` is required on `wasip1/wasm`; it produces a WASI reactor that remains alive after initialization and exposes callable `//go:wasmexport` functions. The host calls `_initialize` once before calling bridge exports.

The generated `llmbridge.wasm` is a separate artifact. It is never embedded in the Go binary. A recipient can replace the LGPL component by changing `PICOTERA_LLMBRIDGE_WASM_PATH` without rebuilding the Go binary.

## Package Layout

```
cmd/llmbridge-wasm/
  main.go                 -- WASI reactor entry point and exported ABI

pkg/llmbridge/
  types.go                -- Format, OutboundProfile, helper methods
  client.go               -- Bridge interface used by server code
  wasm_client.go          -- wazero runtime, module pool, ABI codec

pkg/llmbridgeimpl/
  bridge.go               -- current axonhub-backed BridgeRequest / BridgeNonStream / BridgeStream / AggregateStream implementation
  bridge_stream.go        -- stream setup/pump primitives used by WASM and tests
  llmbridge.go            -- current transformer selection and profile validation
  aggregate.go            -- current aggregation implementation
```

`pkg/llmbridgeimpl` is imported only by `cmd/llmbridge-wasm` and its own tests. The main binary imports `pkg/llmbridge`, which depends on `github.com/tetratelabs/wazero`, not axonhub.

Host runtime files in `pkg/llmbridge` use `//go:build !wasip1`. Shared type/helper files have no build tag. This lets `cmd/llmbridge-wasm` reuse the Picotera-owned `Format` and `OutboundProfile` definitions through `pkg/llmbridge` without pulling the host wazero client into the guest module.

## Host Interface

Define a host-side interface:

```go
type Bridge interface {
    Enabled() bool
    Close(ctx context.Context) error
    BridgeRequest(ctx context.Context, src, dst Format, body []byte, headers http.Header, pendingURL string, profile OutboundProfile) ([]byte, string, error)
    BridgeNonStream(ctx context.Context, src, upstream Format, upstreamBody []byte, upstreamHeaders http.Header, profile OutboundProfile) ([]byte, string, error)
    BridgeStream(ctx context.Context, src, upstream Format, upstreamBody io.ReadCloser, upstreamCT string, profile OutboundProfile) (io.ReadCloser, error)
    AggregateStream(ctx context.Context, format Format, contentType string, body []byte, profile OutboundProfile) ([]byte, error)
}
```

`llmbridge.New(ctx, config)` returns a WASM-backed implementation when an external wasm path is configured. When no module path is configured, it returns a disabled implementation. `Server` receives one instance during `NewServer` startup and stores it as `s.llmBridge`.

The disabled implementation is intentional behavior for the non-LGPL binary without an external module. `Enabled()` returns false, and conversion methods return `llmbridge: wasm module is not configured`. This is not a compatibility layer for old bridge behavior; it is the strict state for a binary that has no LGPL component available.

The package keeps Picotera-owned helpers in-process:

- `Format` constants, `String`, `IsStreaming`, `IsGemini`.
- `OutboundProfile` data shape.
- `DefaultOutboundProfileForFormat`.
- `SyntheticGeminiPath`.
- `StreamAggregationKind`, because this is simple content-type routing and does not require axonhub.
- `NewUpstreamTee`, because it is pure Go stream plumbing used by the response path.

All transformations that need axonhub run inside WASM:

- request conversion
- non-stream response conversion
- stream response conversion
- stream aggregation
- outbound profile validation inside conversion and aggregation calls

## WASM ABI

The ABI uses one request JSON envelope per call. The host allocates input bytes in guest memory, writes the envelope, calls an exported function, reads an output envelope from guest memory, then frees both allocations.

Exported functions:

```go
//go:wasmexport llmbridge_abi_version
func llmbridgeABIVersion() uint32

//go:wasmexport llmbridge_alloc
func llmbridgeAlloc(n uint32) uint32

//go:wasmexport llmbridge_free
func llmbridgeFree(ptr uint32)

//go:wasmexport llmbridge_bridge_request
func llmbridgeBridgeRequest(ptr, len uint32) uint64

//go:wasmexport llmbridge_bridge_non_stream
func llmbridgeBridgeNonStream(ptr, len uint32) uint64

//go:wasmexport llmbridge_bridge_stream_open
func llmbridgeBridgeStreamOpen(ptr, len uint32) uint64

//go:wasmexport llmbridge_bridge_stream_pump
func llmbridgeBridgeStreamPump(streamID uint32) uint64

//go:wasmexport llmbridge_bridge_stream_close
func llmbridgeBridgeStreamClose(streamID uint32) uint64

//go:wasmexport llmbridge_aggregate_stream
func llmbridgeAggregateStream(ptr, len uint32) uint64
```

Every function returning data packs `(ptr, len)` into a `uint64` as `uint64(ptr)<<32 | uint64(len)`. The returned pointer is owned by the host and must be released with `llmbridge_free`.

The module owns allocated Go byte slices in a map keyed by pointer. `llmbridge_free` deletes the map entry. Inputs and outputs are bounded to `math.MaxUint32`; larger payloads fail before allocation.

## Envelope Format

All envelopes are strict JSON. Decoders use `DisallowUnknownFields`.

Common profile shape:

```json
{
  "type": "openai",
  "config": {}
}
```

`bridge_request` input:

```json
{
  "src": 1,
  "dst": 2,
  "body": "base64 bytes",
  "headers": {"Content-Type": ["application/json"]},
  "pendingURL": "/v1/messages",
  "profile": {"type": "openai", "config": {}}
}
```

`bridge_request` output:

```json
{
  "ok": true,
  "body": "base64 bytes",
  "contentType": "application/json"
}
```

Errors use the same response envelope:

```json
{
  "ok": false,
  "error": "llmbridge: build openaiChatCompletions request: ..."
}
```

`bridge_non_stream` uses `src`, `upstream`, `body`, `headers`, and `profile`. `aggregate_stream` uses `format`, `contentType`, `body`, and `profile`.

## Streaming

The host preserves the existing streaming `io.ReadCloser` contract. Identity streaming (`src == upstream`) returns the upstream reader directly and does not call WASM, preserving byte-for-byte passthrough for the current 1:1 path.

Bridged streaming checks out one module slot for the full stream lifetime and runs the current axonhub-backed stream conversion inside the guest. The host does not split provider events or implement provider semantic conversion.

The WASI reactor imports two host functions from module `picotera_llmbridge_host`:

```go
//go:wasmimport picotera_llmbridge_host llmbridge_stream_read
func hostStreamRead(ptr, cap uint32) uint64

//go:wasmimport picotera_llmbridge_host llmbridge_stream_write
func hostStreamWrite(ptr, len uint32) uint32
```

`hostStreamRead` reads from the current module slot's upstream `io.ReadCloser` into guest memory at `(ptr, cap)`. It returns `uint64(status)<<32 | uint64(n)`, where status `0` means bytes read, `1` means EOF, and `2` means error. `hostStreamWrite` writes guest memory bytes to the current slot's `io.PipeWriter` and returns the same status codes without a byte count.

`pkg/llmbridgeimpl` exposes a stream setup primitive used by the guest:

```go
type StreamBridge interface {
    Pump(ctx context.Context, w io.Writer) error
    Close() error
}

func OpenStream(ctx context.Context, src, upstream Format, upstreamBody io.ReadCloser, upstreamCT string, profile OutboundProfile) (StreamBridge, error)
```

`OpenStream` performs the current transformer setup and returns errors before the host returns a reader to the server. `Pump` drains the axonhub event stream and writes source-format SSE bytes to the supplied writer.

The guest wraps the host imports as `io.Reader` and `io.Writer`. Stream setup is synchronous:

1. The host sets the module slot's upstream reader and pipe writer.
2. The host calls `llmbridge_bridge_stream_open` with stream metadata.
3. The guest calls `llmbridgeimpl.OpenStream`, which creates the transformer stream or returns a setup error.
4. The guest stores the returned stream bridge behind a stream handle and returns that handle.
5. The host returns the pipe reader to the server and starts a goroutine that calls `llmbridge_bridge_stream_pump`.
6. The guest pump calls `StreamBridge.Pump` with a writer backed by `hostStreamWrite`.
7. The guest closes and deletes the stream handle when the host calls `llmbridge_bridge_stream_close` after pumping finishes.

`llmbridge_bridge_stream_open` input:

```json
{
  "src": 1,
  "upstream": 2,
  "contentType": "text/event-stream",
  "profile": {"type": "anthropic", "config": {}}
}
```

`llmbridge_bridge_stream_open` output:

```json
{
  "ok": true,
  "streamID": 1
}
```

`llmbridge_bridge_stream_pump` returns an operation response with `ok: true` after EOF. Closing the returned host reader closes the upstream body and the pipe writer. The pump observes the closed host resources through the stream imports, exits, and then the pump goroutine calls `llmbridge_bridge_stream_close` on the same module slot. The host never calls two exported functions on one module concurrently.

## Runtime Lifecycle

`llmbridge.New(ctx, config)`:

1. Creates one wazero runtime.
2. Instantiates `wasi_snapshot_preview1`.
3. Selects module bytes:
   - if `config.WASMPath` is non-empty, read that exact external file
   - otherwise return a disabled bridge with `Enabled() == false`
4. Compiles the selected `llmbridge.wasm`.
5. Builds a fixed-size module pool.
6. Calls `_initialize` and `llmbridge_abi_version` on every module instance.
7. Fails startup when an explicitly configured external path is unreadable, the module cannot compile, or the ABI version differs from the host constant.

The pool size defaults to `runtime.GOMAXPROCS(0)` and is configurable through `PICOTERA_LLMBRIDGE_WASM_POOL_SIZE`. Non-stream calls check out one module for the duration of the call. Bridged stream calls check out one module for the duration of the stream. If selected module bytes cannot load, PicoTera startup fails.

Every call receives the request context. Context cancellation aborts the wazero function call and returns an error to the gateway path.

## Server Integration

`Server` gains:

```go
llmBridge llmbridge.Bridge
```

`NewServer` initializes the bridge after config parsing and before route registration:

```go
bridge, err := llmbridge.New(ctx, llmbridge.Config{
    PoolSize: config.LLMBridgeWASMPoolSize,
    WASMPath: config.LLMBridgeWASMPath,
})
if err != nil {
    return nil, fmt.Errorf("failed to initialize llmbridge wasm: %w", err)
}
```

The unified handler replaces package-level transformation calls:

- `llmbridge.BridgeRequest(...)` -> `s.llmBridge.BridgeRequest(...)`
- `llmbridge.BridgeStream(...)` -> `s.llmBridge.BridgeStream(...)`
- `llmbridge.BridgeNonStream(...)` -> `s.llmBridge.BridgeNonStream(...)`
- `llmbridge.AggregateStream(...)` -> `s.llmBridge.AggregateStream(...)`

Tests that instantiate `Server` helpers directly use a test bridge implementation that calls `pkg/llmbridgeimpl` only from test files.

The five unified generation routes remain registered in both binaries. If a request reaches a conversion path while `s.llmBridge.Enabled()` is false, the handler fails fast with a clear 503-style gateway error stating that `llmbridge.wasm` is not configured. Identity upstream attempts that do not need conversion still pass through without wasm.

Response aggregation that depends on llmbridge runs only when `s.llmBridge.Enabled()` is true. When no module is configured, the server skips aggregated response artifacts and still records the raw artifacts.

## Config

Add:

```go
LLMBridgeWASMPoolSize int `mapstructure:"llmbridge_wasm_pool_size"`
LLMBridgeWASMPath     string `mapstructure:"llmbridge_wasm_path"`
```

Environment variable:

| Variable | Default | Description |
| --- | ---: | --- |
| `PICOTERA_LLMBRIDGE_WASM_POOL_SIZE` | `runtime.GOMAXPROCS(0)` | Number of initialized llmbridge WASM module instances. |
| `PICOTERA_LLMBRIDGE_WASM_PATH` | empty | External `llmbridge.wasm` path. Enables bridge when set. |

`PICOTERA_LLMBRIDGE_WASM_PATH` is read strictly as provided. The config layer does not trim, expand `~`, search default directories, or guess alternate paths. The loaded module must match the host ABI version.

## Build and Release

Add build tasks:

```bash
mise run llmbridge-wasm       # writes dist/llmbridge.wasm
mise run build                # writes picotera
```

Default build:

```bash
go build -o picotera ./cmd/picotera
```

Update Docker into two runtime targets:

1. `runtime`: image containing `picotera` only. Users enable bridge by mounting a wasm file and setting `PICOTERA_LLMBRIDGE_WASM_PATH`.
2. `runtime-lgpl`: image built `FROM runtime`, adds `/app/llmbridge.wasm`, and sets `PICOTERA_LLMBRIDGE_WASM_PATH=/app/llmbridge.wasm`. It reuses the same binary layer as `runtime`. Users override the path by setting the env var to a mounted replacement.

`go build ./cmd/picotera` on a clean checkout continues to work without generating wasm. When code under `pkg/llmbridgeimpl` or `cmd/llmbridge-wasm` changes, regenerate `dist/llmbridge.wasm` before publishing the LGPL runtime artifact.

## Dependencies

The host uses `github.com/tetratelabs/wazero` as the in-process WebAssembly runtime. It is already present in the module graph through QuickJS, but this change makes it a direct dependency.

`pkg/llmbridgeimpl` continues to use `github.com/looplj/axonhub/llm`. That dependency is no longer linked into the main binary; it is linked into the generated WASM module.

## Licensing Notes

This design isolates LGPL-covered transformer code into a replaceable WebAssembly module and keeps the main Go binary free of direct axonhub imports. The shared binary does not include `llmbridge.wasm` but can load one supplied by the operator. The LGPL Docker target supplies the module as a separate replaceable file through `PICOTERA_LLMBRIDGE_WASM_PATH`.

The implementation must update `THIRD_PARTY_NOTICES.md` to describe the LGPL-covered wasm component, its source package, the pinned version, and the command used to rebuild it.
