# API: llmbridge WASM Runtime

## Go Config

Add to `pkg/configx.Config`:

```go
LLMBridgeWASMPoolSize int `mapstructure:"llmbridge_wasm_pool_size"`
LLMBridgeWASMPath     string `mapstructure:"llmbridge_wasm_path"`
```

Environment variable:

| Variable | Default | Description |
| --- | ---: | --- |
| `PICOTERA_LLMBRIDGE_WASM_POOL_SIZE` | `runtime.GOMAXPROCS(0)` | Number of initialized llmbridge WASM module instances. |
| `PICOTERA_LLMBRIDGE_WASM_PATH` | empty | External wasm path. Enables bridge when set. |

Pool size values must be strict positive integers. Invalid values fail config parsing or bridge initialization. `PICOTERA_LLMBRIDGE_WASM_PATH` is used exactly as provided.

## Go Runtime Interface

`pkg/llmbridge` exposes:

```go
type Config struct {
    PoolSize int
    WASMPath string
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

`New` loads the configured external module when `WASMPath` is non-empty. When `WASMPath` is empty, it returns a disabled bridge. All loaded modules are initialized, ABI-checked, and rejected on mismatch.

## WASM Exports

The WASI reactor exports:

```go
llmbridge_abi_version() uint32
llmbridge_alloc(n uint32) uint32
llmbridge_free(ptr uint32)
llmbridge_bridge_request(ptr uint32, len uint32) uint64
llmbridge_bridge_non_stream(ptr uint32, len uint32) uint64
llmbridge_bridge_stream_open(ptr uint32, len uint32) uint64
llmbridge_bridge_stream_pump(streamID uint32) uint64
llmbridge_bridge_stream_close(streamID uint32) uint64
llmbridge_aggregate_stream(ptr uint32, len uint32) uint64
```

`uint64` return values pack `(ptr, len)` as `uint64(ptr)<<32 | uint64(len)`.

## WASM Host Imports

The host provides these functions to the WASI reactor under module `picotera_llmbridge_host`:

```go
llmbridge_stream_read(ptr uint32, cap uint32) uint64
llmbridge_stream_write(ptr uint32, len uint32) uint32
```

`llmbridge_stream_read` returns `uint64(status)<<32 | uint64(n)`.

Status codes:

| Status | Meaning |
| ---: | --- |
| `0` | Success |
| `1` | EOF |
| `2` | Error |

`llmbridge_stream_write` returns only the status code.

## JSON Envelopes

All envelope decoders reject unknown fields.

### `bridge_request`

Request:

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

Response:

```json
{
  "ok": true,
  "body": "base64 bytes",
  "contentType": "application/json"
}
```

### `bridge_non_stream`

Request:

```json
{
  "src": 1,
  "upstream": 2,
  "body": "base64 bytes",
  "headers": {"Content-Type": ["application/json"]},
  "profile": {"type": "openai", "config": {}}
}
```

Response:

```json
{
  "ok": true,
  "body": "base64 bytes",
  "contentType": "application/json"
}
```

### `bridge_stream_open`

Request:

```json
{
  "src": 1,
  "upstream": 2,
  "contentType": "text/event-stream",
  "profile": {"type": "anthropic", "config": {}}
}
```

Response:

```json
{
  "ok": true,
  "streamID": 1
}
```

### `bridge_stream_pump`

Input is the `streamID` integer returned by `bridge_stream_open`.

Response:

```json
{
  "ok": true
}
```

### `bridge_stream_close`

Input is the `streamID` integer returned by `bridge_stream_open`.

Response:

```json
{
  "ok": true
}
```

### `aggregate_stream`

Request:

```json
{
  "format": 5,
  "contentType": "application/jsonl",
  "body": "base64 bytes",
  "profile": {"type": "gemini", "config": {}}
}
```

Response:

```json
{
  "ok": true,
  "body": "base64 json bytes"
}
```

### Error Response

Every exported operation returns this envelope on failure:

```json
{
  "ok": false,
  "error": "llmbridge: unsupported outbound type \"madeup\""
}
```

The host maps `ok: false` to a Go `error` with the exact error string.
