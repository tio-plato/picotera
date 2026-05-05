# API — Unified outbound annotations

## Annotation keys

### `ah.outbound.type`

类型：string

支持值：

| 值 | 行为 |
| --- | --- |
| `openrouter` | 在 unified bridge 目标 format 为 OpenAI Chat Completions 时使用 axonhub `openrouter` outbound transformer |
| `deepseek` | 在 unified bridge 目标 format 为 OpenAI Chat Completions 时使用 axonhub `deepseek` outbound transformer |
| `fireworks` | 在 unified bridge 目标 format 为 OpenAI Chat Completions 时使用 axonhub `fireworks` outbound transformer |
| 空 / 缺失 | 使用现有按 endpoint type 选择的默认 outbound transformer |

匹配时会 trim 空白并转小写。

### `ah.outbound.config`

类型：string，内容必须是 JSON object。

JSON 字段使用对应 axonhub Config 的 JSON tag。目前三类 transformer 都支持：

```json
{
  "base_url": "https://example.invalid/v1"
}
```

`base_url` 会被 picotera 强制覆盖成 bridge 占位 URL。该字段可出现在 config 中，但不会改变真实 upstream URL。真实 upstream URL 仍来自 provider endpoint。

## Go：`pkg/llmbridge`

新增类型：

```go
type OutboundProfile struct {
    Type      string
    ConfigRaw string
}

func OutboundProfileFromAnnotations(ann map[string]string) OutboundProfile
```

现有 bridge 函数签名增加 `OutboundProfile` 参数：

```go
func BridgeRequest(
    ctx context.Context,
    src Format,
    dst Format,
    body []byte,
    headers http.Header,
    pendingURL string,
    profile OutboundProfile,
) ([]byte, string, error)

func BridgeNonStream(
    ctx context.Context,
    src Format,
    upstream Format,
    upstreamBody []byte,
    upstreamHeaders http.Header,
    profile OutboundProfile,
) ([]byte, string, error)

func BridgeStream(
    ctx context.Context,
    src Format,
    upstream Format,
    upstreamBody io.ReadCloser,
    upstreamCT string,
    profile OutboundProfile,
) (io.ReadCloser, error)
```

不新增 `WithProfile` 兼容层，不保留旧签名。现有调用点随本次改造同步迁移，测试按新签名更新。

## Go：`pkg/server`

`providerSidecar.annotations` 是现有 merged annotations map。unified retry loop 中：

```go
profile := llmbridge.OutboundProfileFromAnnotations(candAnno)
```

桥接调用点传入 profile。`unifiedStreamArgs` 增加：

```go
outboundProfile llmbridge.OutboundProfile
```

该字段由 attempt 处填入，并在 response bridge 阶段复用。
