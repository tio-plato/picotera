# Design: llmbridge go-plugin Runtime

## Goal

Replace the current TinyGo/WASI/wazero llmbridge runtime with a HashiCorp `go-plugin` based process plugin. The main PicoTera server keeps the existing `pkg/llmbridge.Bridge` interface used by gateway code, but the implementation starts and talks to a separate `picotera-llmbridge-plugin` executable over gRPC.

The WASM path is removed. There is no TinyGo build, no wazero runtime, no WASM precompile command, no WASM cache, and no old `PICOTERA_LLMBRIDGE_WASM_*` configuration.

## Runtime Model

The host process starts one local plugin child process through `github.com/hashicorp/go-plugin`. The plugin implements the existing conversion operations by calling `pkg/llmbridgeimpl`, which continues to contain the AxonHub-backed transformer implementation.

Process isolation replaces WASM isolation:

- the host imports `pkg/llmbridge` only;
- the plugin executable imports `pkg/llmbridgeimpl`;
- the host talks to the plugin through a narrow gRPC service;
- the plugin process is terminated from `Bridge.Close`.

This keeps the conversion component independently built and shipped while avoiding the TinyGo and WASI constraints that caused operational issues.

## Package Layout

```
cmd/picotera/
  main.go                         -- server CLI; no precompile-llmbridge-wasm command

cmd/picotera-llmbridge-plugin/
  main.go                         -- go-plugin server entry point

pkg/llmbridge/
  types.go                        -- Format, OutboundProfile, helper methods
  client.go                       -- Bridge interface, Config, disabled bridge
  plugin_client.go                -- go-plugin client implementation
  plugin_protocol.go              -- handshake, plugin map, shared protobuf adapters
  llmbridge.proto                 -- gRPC service definition
  llmbridge.pb.go                 -- generated protobuf messages
  llmbridge_grpc.pb.go            -- generated gRPC client/server
  tee.go                          -- stream tee helper

pkg/llmbridgeimpl/
  bridge.go                       -- BridgeRequest / BridgeNonStream implementation
  bridge_stream.go                -- streaming implementation
  aggregate.go                    -- stream aggregation implementation
  llmbridge.go                    -- transformer selection and profile validation
```

`cmd/llmbridge-wasm/` and `pkg/llmbridge/wasm_client.go` are deleted. `github.com/tetratelabs/wazero` is removed from `go.mod`; `github.com/hashicorp/go-plugin`, protobuf, and gRPC dependencies are added.

## go-plugin Integration

The host uses `plugin.NewClient` with a fixed handshake and a single plugin name:

```go
const pluginName = "llmbridge"

var handshake = plugin.HandshakeConfig{
    ProtocolVersion: 1,
    MagicCookieKey: "PICOTERA_LLMBRIDGE_PLUGIN",
    MagicCookieValue: "1",
}
```

The plugin executable uses the same handshake and serves the `llmbridge` plugin through `plugin.Serve`. Only gRPC mode is enabled.

The host validates the plugin by calling `GetInfo` immediately after startup. `GetInfo` returns `abiVersion: 1`; any other version fails server startup with a clear error.

## Host Interface

The public Go interface stays stable for gateway code:

```go
type Config struct {
    PluginPath string
}

type Bridge interface {
    Enabled() bool
    Close(ctx context.Context) error
    BridgeRequest(ctx context.Context, src, dst Format, body []byte, headers http.Header, pendingURL string, profile OutboundProfile) ([]byte, string, error)
    BridgeNonStream(ctx context.Context, src, upstream Format, upstreamBody []byte, upstreamHeaders http.Header, profile OutboundProfile) ([]byte, string, error)
    BridgeStream(ctx context.Context, src, upstream Format, upstreamBody io.ReadCloser, upstreamCT string, profile OutboundProfile) (io.ReadCloser, error)
    AggregateStream(ctx context.Context, format Format, contentType string, body []byte, profile OutboundProfile) ([]byte, error)
}
```

`llmbridge.New(ctx, cfg)` starts the plugin when `PluginPath` is non-empty. When `PluginPath` is empty, it returns `disabledBridge`. Identity conversions stay in the host adapter so same-format requests and responses pass through byte-for-byte without plugin calls. Cross-format conversions require an enabled plugin and fail fast when disabled.

## gRPC Service

The plugin exposes four RPCs:

```proto
service LLMBridge {
  rpc GetInfo(GetInfoRequest) returns (GetInfoResponse);
  rpc BridgeRequest(BridgeRequestRequest) returns (BridgeBodyResponse);
  rpc BridgeNonStream(BridgeNonStreamRequest) returns (BridgeBodyResponse);
  rpc BridgeStream(stream BridgeStreamChunk) returns (stream BridgeStreamChunk);
  rpc AggregateStream(AggregateStreamRequest) returns (AggregateStreamResponse);
}
```

Unary operations carry request/response bodies as bytes. Headers use `map<string, HeaderValues>` because protobuf maps cannot directly contain repeated values.

`OutboundProfile.Config` is encoded as strict JSON object bytes. The host rejects a profile config that cannot marshal to a JSON object. The plugin rejects profile config JSON that cannot unmarshal into `map[string]any` or contains multiple JSON values.

## Streaming

`BridgeStream` uses a bidirectional gRPC stream. The host opens the RPC, sends a metadata frame, then copies upstream bytes into `data` frames. The plugin reconstructs an `io.Reader` from incoming data frames, runs `llmbridgeimpl.BridgeStream`, and sends converted bytes back as `data` frames.

Frame shape:

```proto
message BridgeStreamChunk {
  oneof payload {
    BridgeStreamStart start = 1;
    bytes data = 2;
    BridgeStreamEnd end = 3;
    BridgeStreamError error = 4;
  }
}
```

Rules:

- the first host frame must be `start`;
- host `data` frames must follow `start`;
- host sends `end` after upstream EOF;
- plugin sends converted `data` frames as they are produced;
- plugin sends `error` and closes the stream when conversion fails;
- receiving frames out of order is a protocol error;
- identity streams return the original upstream body directly and do not call the plugin.

The host returns an `io.PipeReader` immediately after the plugin accepts the `start` frame. Two goroutines manage the stream: one copies upstream bytes into the gRPC send side, and one copies plugin data frames into the pipe. Closing the returned reader closes the upstream body and the gRPC stream.

## Configuration

Configuration moves from WASM-specific keys to plugin-specific keys:

| Env var | Default | Meaning |
| --- | --- | --- |
| `PICOTERA_LLMBRIDGE_PLUGIN_PATH` | empty | Path to the `picotera-llmbridge-plugin` executable. Empty disables cross-format conversion. |
| `PICOTERA_LLMBRIDGE_PLUGIN_START_TIMEOUT` | `10s` | Maximum time allowed for starting and validating the plugin. |

The path is used exactly as provided. Invalid paths or non-executable files fail server startup. The old `PICOTERA_LLMBRIDGE_WASM_PATH`, `PICOTERA_LLMBRIDGE_WASM_POOL_SIZE`, `PICOTERA_LLMBRIDGE_WASM_CACHE_DIR`, and `PICOTERA_LLMBRIDGE_WASM_RUNTIME` settings are removed.

## Build And Packaging

`mise.toml` builds the plugin with the normal Go toolchain:

```bash
go build -o dist/picotera-llmbridge-plugin ./cmd/picotera-llmbridge-plugin
```

`mise run server` depends on the plugin build and sets `PICOTERA_LLMBRIDGE_PLUGIN_PATH` to `dist/picotera-llmbridge-plugin` for local development.

The Docker image builds both binaries. Runtime copies:

- `/app/picotera`
- `/app/picotera-llmbridge-plugin`

and sets:

```bash
PICOTERA_LLMBRIDGE_PLUGIN_PATH=/app/picotera-llmbridge-plugin
```

Operators can replace the plugin by mounting a different executable and setting `PICOTERA_LLMBRIDGE_PLUGIN_PATH` to that path.

## Notices

`THIRD_PARTY_NOTICES.md` is updated from `llmbridge.wasm` wording to `picotera-llmbridge-plugin` wording. The notice continues to state that the conversion component imports `github.com/looplj/axonhub/llm` and is built from `cmd/picotera-llmbridge-plugin/` plus `pkg/llmbridgeimpl/`.
