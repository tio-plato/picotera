# Plan: Replace llmbridge WASM With go-plugin

## Step 1: Remove WASM Runtime Surface

1. Delete `cmd/llmbridge-wasm/`.
2. Delete `pkg/llmbridge/wasm_client.go` and `pkg/llmbridge/wasm_client_test.go`.
3. Remove the `wasip1` build tags that exist only for the WASM split.
4. Remove `Precompile`, `DefaultCacheDir`, WASM runtime constants, WASM ABI helpers, and WASM diagnostics types from `pkg/llmbridge`.
5. Remove the `precompile-llmbridge-wasm` cobra subcommand from `cmd/picotera/main.go`.
6. Update error strings from `llmbridge: wasm module is not configured` to `llmbridge: plugin is not configured`.

## Step 2: Add the gRPC Protocol

1. Add `pkg/llmbridge/llmbridge.proto`.
2. Add protobuf generation tooling to the normal Go toolchain workflow.
3. Generate `pkg/llmbridge/llmbridge.pb.go` and `pkg/llmbridge/llmbridge_grpc.pb.go`.
4. Implement strict adapter helpers in `pkg/llmbridge/plugin_protocol.go`:
   - `http.Header` to/from protobuf map;
   - `OutboundProfile` to/from `OutboundProfileMessage`;
   - `Format` to/from protobuf `int32`;
   - plugin ABI validation.
5. Reject malformed profile config JSON with explicit errors.

## Step 3: Implement the Plugin Server

1. Add `cmd/picotera-llmbridge-plugin/main.go`.
2. Implement a go-plugin server with the fixed handshake and plugin key `llmbridge`.
3. Add a `grpcPlugin` implementation that registers the `LLMBridge` gRPC server.
4. Implement `GetInfo` returning ABI version `1`.
5. Implement unary RPCs by calling:
   - `llmbridgeimpl.BridgeRequest`;
   - `llmbridgeimpl.BridgeNonStream`;
   - `llmbridgeimpl.AggregateStream`.
6. Implement `BridgeStream`:
   - read and validate the initial `start` frame;
   - expose incoming host `data` frames as an `io.ReadCloser`;
   - call `llmbridgeimpl.BridgeStream`;
   - copy converted bytes into outgoing `data` frames;
   - return protocol and transformer errors through gRPC errors or `error` frames.
7. Ensure plugin shutdown closes active stream readers.

## Step 4: Implement the Host Plugin Client

1. Update `pkg/llmbridge.Config` to contain `PluginPath` and `PluginStartTimeout`.
2. Implement `pkg/llmbridge/plugin_client.go`.
3. In `llmbridge.New`, return `disabledBridge` when `PluginPath` is empty.
4. When `PluginPath` is set:
   - start the executable using `plugin.NewClient`;
   - request the `llmbridge` plugin over gRPC;
   - call `GetInfo`;
   - reject ABI versions other than `1`.
5. Implement `Enabled` and `Close`; `Close` must terminate the plugin process.
6. Keep identity `BridgeRequest`, `BridgeNonStream`, and `BridgeStream` in the host adapter.
7. Implement cross-format unary calls through gRPC.
8. Implement cross-format `BridgeStream` with an `io.Pipe` and bidirectional gRPC stream.
9. Propagate context cancellation and returned reader close to the upstream body and gRPC stream.

## Step 5: Wire Server Configuration

1. Replace `LLMBridgeWASMPoolSize`, `LLMBridgeWASMPath`, `LLMBridgeWASMCacheDir`, and `LLMBridgeWASMRuntime` in `pkg/configx.Config`.
2. Add `LLMBridgePluginPath` and `LLMBridgePluginStartTimeout`.
3. Set default `llmbridge_plugin_start_timeout` to `10s`.
4. Update `NewServer` logging to report plugin path and startup timeout when configured.
5. Initialize `llmbridge.New` with plugin config.
6. Update gateway errors to say the plugin is not configured.
7. Keep `NewHuma` free of plugin initialization.

## Step 6: Update Build And Packaging

1. Remove `tinygo` from `mise.toml`.
2. Remove `PICOTERA_LLMBRIDGE_WASM_PATH` from `mise.toml`.
3. Replace `[tasks.wasm]` and `[tasks.precompile-wasm]` with `[tasks.llmbridge-plugin]`.
4. Make `[tasks.server]` depend on `llmbridge-plugin` and set `PICOTERA_LLMBRIDGE_PLUGIN_PATH` to `dist/picotera-llmbridge-plugin`.
5. Update the backend build documentation in `CLAUDE.md` to remove WASM build instructions.
6. Update `Dockerfile`:
   - remove the TinyGo builder stage;
   - build `/app/picotera`;
   - build `/app/picotera-llmbridge-plugin`;
   - copy both binaries into the runtime image;
   - set `PICOTERA_LLMBRIDGE_PLUGIN_PATH=/app/picotera-llmbridge-plugin`.
7. Remove WASM cache copy steps from Docker packaging.

## Step 7: Update Dependency And Notices

1. Add `github.com/hashicorp/go-plugin`.
2. Add protobuf and gRPC generation/runtime dependencies required by generated code.
3. Remove direct dependency on `github.com/tetratelabs/wazero`.
4. Keep `github.com/looplj/axonhub/llm` for `pkg/llmbridgeimpl` and the plugin binary.
5. Update `THIRD_PARTY_NOTICES.md` to reference `picotera-llmbridge-plugin` instead of `llmbridge.wasm`.
6. Update README and deployment docs that mention `llmbridge.wasm`, TinyGo, or `PICOTERA_LLMBRIDGE_WASM_PATH`.

## Step 8: Tests

1. Keep `pkg/llmbridgeimpl` tests for direct transformer behavior.
2. Add `pkg/llmbridge` tests for:
   - disabled bridge when plugin path is empty;
   - startup failure for missing plugin path;
   - ABI mismatch;
   - Anthropic request to OpenAI Chat request conversion;
   - OpenAI Chat response to Anthropic response conversion;
   - bridged SSE stream conversion;
   - identity stream byte-for-byte passthrough;
   - profile config JSON validation.
3. Add plugin server protocol tests for:
   - first stream frame must be `start`;
   - duplicate `start` fails;
   - `data` before `start` fails;
   - plugin returns clear errors for transformer failures.
4. Update server tests and fakes to use the unchanged `llmbridge.Bridge` interface.

## Step 9: Verification

Run:

```bash
mise run llmbridge-plugin
go test ./pkg/llmbridgeimpl ./pkg/llmbridge ./pkg/server
go test ./...
go build ./cmd/picotera
go build ./cmd/picotera-llmbridge-plugin
```

Verify dependency shape:

```bash
go list -deps ./cmd/picotera | rg 'github.com/tetratelabs/wazero' && exit 1 || true
go list -deps ./cmd/picotera | rg 'cmd/llmbridge-wasm' && exit 1 || true
```

Manually verify the unified gateway with:

1. a cross-format non-stream request;
2. a cross-format streaming request;
3. a same-format request confirming byte-for-byte passthrough.
