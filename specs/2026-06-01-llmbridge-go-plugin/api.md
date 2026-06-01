# API: llmbridge go-plugin Runtime

## Go Config

`pkg/configx.Config` contains:

```go
LLMBridgePluginPath         string        `mapstructure:"llmbridge_plugin_path"`
LLMBridgePluginStartTimeout time.Duration `mapstructure:"llmbridge_plugin_start_timeout"`
```

Environment variables:

| Variable | Default | Description |
| --- | --- | --- |
| `PICOTERA_LLMBRIDGE_PLUGIN_PATH` | empty | Executable path for the llmbridge plugin. Empty disables cross-format conversion. |
| `PICOTERA_LLMBRIDGE_PLUGIN_START_TIMEOUT` | `10s` | Startup and ABI validation deadline. |

Removed environment variables:

- `PICOTERA_LLMBRIDGE_WASM_PATH`
- `PICOTERA_LLMBRIDGE_WASM_POOL_SIZE`
- `PICOTERA_LLMBRIDGE_WASM_CACHE_DIR`
- `PICOTERA_LLMBRIDGE_WASM_RUNTIME`

## Go Runtime Interface

`pkg/llmbridge` exposes:

```go
type Config struct {
    PluginPath         string
    PluginStartTimeout time.Duration
}

type Bridge interface {
    Enabled() bool
    Close(ctx context.Context) error
    BridgeRequest(ctx context.Context, src, dst Format, body []byte, headers http.Header, pendingURL string, profile OutboundProfile) ([]byte, string, error)
    BridgeNonStream(ctx context.Context, src, upstream Format, upstreamBody []byte, upstreamHeaders http.Header, profile OutboundProfile) ([]byte, string, error)
    BridgeStream(ctx context.Context, src, upstream Format, upstreamBody io.ReadCloser, upstreamCT string, profile OutboundProfile) (io.ReadCloser, error)
    AggregateStream(ctx context.Context, format Format, contentType string, body []byte, profile OutboundProfile) ([]byte, error)
}

func New(ctx context.Context, cfg Config) (Bridge, error)
```

`New` returns `disabledBridge` when `PluginPath` is empty. `New` starts and validates the plugin when `PluginPath` is non-empty.

## Plugin Handshake

The host and plugin use:

```go
plugin.HandshakeConfig{
    ProtocolVersion:  1,
    MagicCookieKey:   "PICOTERA_LLMBRIDGE_PLUGIN",
    MagicCookieValue: "1",
}
```

Plugin map key:

```go
"llmbridge"
```

Only gRPC transport is supported.

## Protobuf

File: `pkg/llmbridge/llmbridge.proto`

```proto
syntax = "proto3";

package picotera.llmbridge.v1;

option go_package = "picotera/pkg/llmbridge;llmbridge";

service LLMBridge {
  rpc GetInfo(GetInfoRequest) returns (GetInfoResponse);
  rpc BridgeRequest(BridgeRequestRequest) returns (BridgeBodyResponse);
  rpc BridgeNonStream(BridgeNonStreamRequest) returns (BridgeBodyResponse);
  rpc BridgeStream(stream BridgeStreamChunk) returns (stream BridgeStreamChunk);
  rpc AggregateStream(AggregateStreamRequest) returns (AggregateStreamResponse);
}

message GetInfoRequest {}

message GetInfoResponse {
  uint32 abi_version = 1;
}

message HeaderValues {
  repeated string values = 1;
}

message OutboundProfileMessage {
  string type = 1;
  bytes config_json = 2;
}

message BridgeRequestRequest {
  int32 src = 1;
  int32 dst = 2;
  bytes body = 3;
  map<string, HeaderValues> headers = 4;
  string pending_url = 5;
  OutboundProfileMessage profile = 6;
}

message BridgeNonStreamRequest {
  int32 src = 1;
  int32 upstream = 2;
  bytes body = 3;
  map<string, HeaderValues> headers = 4;
  OutboundProfileMessage profile = 5;
}

message BridgeBodyResponse {
  bytes body = 1;
  string content_type = 2;
}

message AggregateStreamRequest {
  int32 format = 1;
  string content_type = 2;
  bytes body = 3;
  OutboundProfileMessage profile = 4;
}

message AggregateStreamResponse {
  bytes body = 1;
}

message BridgeStreamStart {
  int32 src = 1;
  int32 upstream = 2;
  string content_type = 3;
  OutboundProfileMessage profile = 4;
}

message BridgeStreamEnd {}

message BridgeStreamError {
  string message = 1;
}

message BridgeStreamChunk {
  oneof payload {
    BridgeStreamStart start = 1;
    bytes data = 2;
    BridgeStreamEnd end = 3;
    BridgeStreamError error = 4;
  }
}
```

## RPC Semantics

### `GetInfo`

Returns:

```json
{"abiVersion": 1}
```

The host rejects any value other than `1`.

### `BridgeRequest`

Converts a client request body from `src` to `dst`.

Errors:

- unknown `src` or `dst`;
- invalid profile config JSON;
- transformer parse/build failure.

### `BridgeNonStream`

Converts a non-streaming upstream response body from `upstream` to `src`.

Errors:

- unknown `src` or `upstream`;
- invalid profile config JSON;
- transformer parse/write failure.

### `BridgeStream`

The host sends exactly one `start` frame, zero or more `data` frames, then one `end` frame. The plugin sends zero or more converted `data` frames. Either side can send `error` to terminate with a clear message.

Protocol errors:

- first host frame is not `start`;
- duplicate `start`;
- `data` before `start`;
- frames after `end`;
- empty error message.

### `AggregateStream`

Aggregates a captured stream body into the non-stream response shape for the same format.

Errors:

- unsupported stream content type;
- empty stream chunks;
- invalid profile config JSON;
- transformer aggregation failure;
- transformer returned invalid JSON.
