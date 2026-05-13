# Plan: llmbridge WASM Isolation

## Step 1: Split Types from Implementation

1. Move pure Picotera-owned definitions from `pkg/llmbridge` into files that remain in `pkg/llmbridge`:
   - `Format` constants and methods.
   - `OutboundProfile`.
   - `DefaultOutboundProfileForFormat`.
   - `SyntheticGeminiPath`.
   - `StreamAggregationKind`.
   - `NewUpstreamTee`.
2. Move axonhub-backed implementation files into `pkg/llmbridgeimpl`.
3. Update package names and imports in moved files.
4. Add `//go:build !wasip1` to `pkg/llmbridge` host runtime files so the guest build imports only shared type/helper files from `pkg/llmbridge`.
5. Split the current stream code into `OpenStream`, `StreamBridge.Pump`, and the existing `BridgeStream` wrapper.
6. Keep `pkg/llmbridgeimpl` tests with the moved implementation so current bridge fixture coverage continues to exercise axonhub behavior directly.

## Step 2: Add the WASI Reactor

1. Create `cmd/llmbridge-wasm/main.go`.
2. Import `pkg/llmbridgeimpl`.
3. Implement `//go:wasmexport` functions:
   - `llmbridge_abi_version`
   - `llmbridge_alloc`
   - `llmbridge_free`
   - `llmbridge_bridge_request`
   - `llmbridge_bridge_non_stream`
   - `llmbridge_bridge_stream_open`
   - `llmbridge_bridge_stream_pump`
   - `llmbridge_bridge_stream_close`
   - `llmbridge_aggregate_stream`
4. Implement the guest allocation map keyed by `uint32` pointer.
5. Implement strict JSON envelope decoding and error response encoding.
6. Implement guest wrappers for imported host stream read/write functions.
7. Implement the guest stream handle table for bridged stream readers.
8. Use `llmbridgeimpl.OpenStream` in `llmbridge_bridge_stream_open` and `StreamBridge.Pump` in `llmbridge_bridge_stream_pump`.
9. Compile with `GOOS=wasip1 GOARCH=wasm go build -buildmode=c-shared`.

## Step 3: Implement the Host WASM Client

1. Make `github.com/tetratelabs/wazero` a direct dependency.
2. Create `pkg/llmbridge/client.go` with the `Bridge` interface and `Config`.
3. Create `pkg/llmbridge/wasm_client.go`.
4. Initialize one wazero runtime and instantiate `wasi_snapshot_preview1`.
5. Select module bytes from `Config.WASMPath`; return disabled state when it is empty.
6. Compile the selected module once.
7. Create a module pool with one initialized module per slot.
8. Add host imports under `picotera_llmbridge_host`:
   - `llmbridge_stream_read`
   - `llmbridge_stream_write`
9. Call `_initialize` and validate `llmbridge_abi_version` on each module.
10. Implement helper methods for:
   - guest allocation
   - guest memory write
   - export invocation
   - output memory read
   - guest pointer release
   - strict response envelope decoding

## Step 4: Implement Host Bridge Methods

1. Implement `BridgeRequest` through `llmbridge_bridge_request`.
2. Implement `BridgeNonStream` through `llmbridge_bridge_non_stream`.
3. Implement `AggregateStream` through `llmbridge_aggregate_stream`.
4. Keep identity request, non-stream, and stream paths in the host adapter so exact-format calls avoid unnecessary wasm work.
5. Implement `BridgeStream` as a host `io.Pipe`:
   - Return upstream reader directly when `src == upstream`.
   - Check out one module slot for the full stream lifetime.
   - Set the slot's current upstream reader and pipe writer.
   - Call `llmbridge_bridge_stream_open` synchronously and return setup errors immediately.
   - Start a goroutine that calls `llmbridge_bridge_stream_pump`.
   - Let the guest perform axonhub stream decoding, conversion, and SSE emission.
   - On host reader close, close the upstream body and pipe writer.
   - Call `llmbridge_bridge_stream_close` from the pump goroutine after `llmbridge_bridge_stream_pump` returns.
   - Close the pipe with the returned conversion error.

## Step 5: Wire the Server to the Runtime Instance

1. Add `llmBridge llmbridge.Bridge` to `Server`.
2. Add `LLMBridgeWASMPoolSize` and `LLMBridgeWASMPath` to `configx.Config`.
3. Set the default pool size to `runtime.GOMAXPROCS(0)`.
4. Initialize the bridge in `NewServer` after config parsing.
5. Store the bridge on `Server`.
6. Keep unified generation routes registered in both binaries.
7. Fail conversion attempts with a clear gateway error when `s.llmBridge.Enabled()` is false.
8. Let identity attempts pass through without wasm.
9. Replace direct package calls in unified gateway response handling:
   - `llmbridge.BridgeRequest`
   - `llmbridge.BridgeStream`
   - `llmbridge.BridgeNonStream`
   - `llmbridge.AggregateStream`
10. Update `response_aggregation.go` so aggregation receives a bridge instance instead of calling the package-level implementation.
11. Skip aggregated response artifacts when `s.llmBridge.Enabled()` is false.
12. Keep `NewHuma` free of bridge initialization because it only builds the OpenAPI schema.

## Step 6: Update Build Commands

1. Add build tasks to `mise.toml`:

   ```bash
   GOOS=wasip1 GOARCH=wasm go build -buildmode=c-shared -o dist/llmbridge.wasm ./cmd/llmbridge-wasm
   go build -o picotera ./cmd/picotera
   ```

2. Keep the default build independent from generated wasm.
3. Build `dist/llmbridge.wasm` before the LGPL runtime image build.
4. Update Dockerfile with two runtime targets:
   - default target containing `picotera` only
   - LGPL target built from the default target, adding `/app/llmbridge.wasm`
5. In the LGPL image, set `PICOTERA_LLMBRIDGE_WASM_PATH=/app/llmbridge.wasm`.
6. Document that mounted external wasm enables bridge in the default image and overrides the shipped wasm in the LGPL image by changing `PICOTERA_LLMBRIDGE_WASM_PATH`.
7. Document that `dist/llmbridge.wasm` must be regenerated after touching `pkg/llmbridgeimpl` or `cmd/llmbridge-wasm`.

## Step 7: Update Notices and Dependency Metadata

1. Move `github.com/looplj/axonhub/llm` out of the main package import graph.
2. Keep the dependency in `go.mod` for the wasm build.
3. Promote `github.com/tetratelabs/wazero` to a direct dependency.
4. Update `THIRD_PARTY_NOTICES.md`:
   - identify the LGPL-covered llmbridge wasm component
   - list the pinned axonhub module version
   - include the exact rebuild command
   - point to source locations used to build the module

## Step 8: Tests

1. Keep existing `pkg/llmbridgeimpl` tests for direct transformer behavior.
2. Add `pkg/llmbridge` host adapter tests that load the generated wasm and cover:
   - ABI version mismatch handling with a test module
   - disabled bridge when no path exists
   - external path enabling bridge
   - `BridgeRequest` Anthropic -> OpenAI Chat
   - `BridgeNonStream` OpenAI Chat -> Anthropic
   - bridged SSE stream conversion
   - identity stream byte-for-byte passthrough
   - profile validation errors
3. Update server tests to use a fake `llmbridge.Bridge` where the test does not need real conversion.
4. Add a `mise run llmbridge-wasm-check` task that rebuilds to a temporary path and compares it with `dist/llmbridge.wasm`.

## Step 9: Verification

Run:

```bash
mise run llmbridge-wasm
go test ./pkg/llmbridgeimpl ./pkg/llmbridge ./pkg/server
go test ./...
go build ./cmd/picotera
```

Then verify the main binary no longer imports axonhub packages:

```bash
go list -deps ./cmd/picotera | rg 'github.com/looplj/axonhub/llm' && exit 1 || true
```

Run the unified gateway manually with at least one cross-format non-stream request and one cross-format streaming request.
