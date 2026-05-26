# 设计：Gateway Handler 编排层重构

## 目标

将 path-based gateway 与 unified gateway 的共同请求生命周期提取为共享编排层，保留两条入口各自的路由、模型解析、provider 解析、桥接和 web search 行为。重构后：

- `gatewayHandler.ServeHTTP` 只负责 path endpoint 匹配、model-list 分支和调用共享编排。
- `Server.handleUnifiedGenerate` 只负责构造 unified route 配置并调用共享编排。
- meta request、JS session、rewriteModel、sortProviders、beforeRequest、rewriteRequest、attempt 记录、失败收尾、artifact 上传和成功响应收尾由共享代码处理。
- 所有 DB、artifact、trace、bridge 与 upstream 操作使用明确的 context，避免在请求路径上直接使用 `context.Background()`。
- 错误路径通过统一 helper 完成 response 写入、request row 更新和 artifact 记录，减少漏写或重复写。

## 当前问题

`pkg/server/handle_gateway.go` 的 `ServeHTTP` 把以下职责集中在一个函数中：

- endpoint 匹配与 dashboard fallback。
- body 读取与 meta request 插入。
- API key 鉴权。
- model 提取、rewriteModel 和 request body 回写。
- provider 查询、候选列表和 JS sidecar 构造。
- sortProviders、beforeRequest、rewriteRequest hook。
- upstream attempt 插入、forward、retry 状态维护。
- 成功响应流式写回、artifact 聚合、token/TTFT/cost 计算。
- 各种失败路径的 request row 与 artifact 收尾。

`pkg/server/handle_unified_gateway.go` 复制了其中大部分生命周期，并在 attempt 中加入 unified 特有的 source/upstream format 选择、beforeTransform、llmbridge、Anthropic web search emulation 和 unified success 响应桥接。

这两个文件已经有底层 helper，但缺少一个中层编排抽象来表达“网关请求从 meta 到 attempts 的生命周期”。

## 新的结构

新增 `pkg/server/gateway_flow.go`，作为共享编排层。它只在 `pkg/server` 包内使用，不新增公开 API。

核心类型：

```go
type gatewayRouteKind int

const (
    gatewayRoutePath gatewayRouteKind = iota
    gatewayRouteUnified
)

type gatewayFlow struct {
    h              *gatewayHandler
    w              http.ResponseWriter
    r              *http.Request
    startedAt      time.Time
    opCtx          context.Context
    persistCtx     context.Context
    persistCancel  context.CancelFunc
    body           []byte
    preRewriteBody []byte
    meta           gatewayMetaState
    session        *jsx.Session
}

type gatewayFlowConfig struct {
    Kind                gatewayRouteKind
    Endpoint            db.Endpoint
    PathVars            map[string]string
    SourceFormat        llmbridge.Format
    ExtractModelAndMode func(*http.Request, []byte, map[string]string) (gatewayModelMode, error)
    SetBodyModel        func([]byte, string) ([]byte, error)
    ResolveCandidates   func(context.Context, string, bool) (candidateSet, error)
    PrepareAttempt      func(context.Context, *gatewayFlow, attemptInput) (attemptPrepared, error)
    HandleSuccess       func(successInput)
}
```

`gatewayFlow` 持有单次请求的状态，`gatewayFlowConfig` 持有 path/unified 差异点。共享编排按固定顺序执行：

1. 读取 body。
2. 插入 meta request 并上传 meta request artifact。
3. 鉴权并回填 `api_key_id`。
4. 提取 model 与 streaming 标志。
5. 创建 JSX session。
6. 执行 rewriteModel 并回写 body/model。
7. 解析 provider candidate。
8. 构造 JS-visible candidate 并执行 sortProviders。
9. 运行 attempt loop。
10. 成功时调用 route-specific success handler。
11. 全部失败时统一完成 meta 失败并写 502 artifact。

## 候选与 sidecar 抽象

新增统一 candidate 数据结构，用于替代两个 handler 内部各自定义的 sidecar map：

```go
type gatewayCandidate struct {
    Candidate jsx.Candidate
    Sidecar   gatewayCandidateSidecar
}

type gatewayCandidateSidecar struct {
    Key                     string
    ProviderID              int32
    UpstreamURL             string
    Credentials             string
    SendResolver            int32
    ProxyURL                string
    EndpointPath            string
    EndpointType            int32
    UpstreamFormat          llmbridge.Format
    Annotations             map[string]string
    SupportsNativeWebSearch bool
}

type candidateSet struct {
    Items      []gatewayCandidate
    ModelAnno  map[string]string
}
```

path gateway 的 key 为 provider id，unified gateway 的 key 为 `providerID|endpointPath`。统一 helper `candidateKey(kind, candidate)` 根据 route kind 计算 key，并拒绝 JS hook 注入的未知候选。

path provider 查询继续使用 `resolveProviders`。unified provider 查询继续使用 `resolveProvidersByTypes`、`candidateEndpointTypes`、`dedupeUnifiedRows` 和 `upstreamFormatFor`。共享层只消费标准化后的 `candidateSet`。

`handle_simulate.go` 已有相似 candidate/sidecar 逻辑。Simulator 的复用已拆为独立需求（`specs/2026-05-26-simulator-candidate-reuse`），本次重构不修改 `handle_simulate.go`。

## Attempt 编排

新增 `gatewayAttemptRunner` 或 `runAttempts` 方法承接 retry loop。它处理共同逻辑：

- `JSMaxTotalAttempts` 限制。
- `beforeRequest` hook。
- hook delay，使用请求 context 可取消的 timer。
- `dec.Next` 跳过候选。
- upstream request row 插入。
- `buildUpstreamRequest`。
- `rewriteRequest` hook。
- upstream request artifact。
- `forwardRequest`。
- 非 200 响应读取、artifact 上传、失败 row 更新、`lastErr` 与 `LastError` 更新。

route-specific `PrepareAttempt` 在 `rewriteRequest` 后、artifact 上传前运行：

- path gateway：返回原请求，不做转换。
- unified gateway：执行 web search emulation、beforeTransform、llmbridge request bridge，并返回最终 upstream request/body、outbound profile、web search context 和 upstream format。

成功响应处理保留两个 route-specific 函数：

- path gateway 使用现有 `streamSuccess` 逻辑，改为接收结构体参数并使用共享 completion helper。
- unified gateway 使用现有 `unifiedStreamSuccess` 逻辑，改为接收共享 success state 和 route-specific transform state。

## 错误处理

新增 `gatewayMetaState` 与 `gatewayResponder` 风格 helper，集中处理 meta 失败：

```go
type gatewayMetaState struct {
    ID              string
    CreatedAt       time.Time
    InsertCreatedAt time.Time
    ParentSpanID     pgtype.Text
    APIKeyID         pgtype.Int4
    ProjectID        pgtype.Int4
    RequestHeader    http.Header
    RequestMethod    string
    RequestURL       string
}
```

错误 helper 负责：

- 将 `gatewayError` 映射到 HTTP status、message、code。
- hook timeout 映射为 503，其余 hook 错误为 502。
- update meta row 为 failed。
- 写 JSON error response。
- 上传 meta response artifact，包含已收集的 JSX console logs。

共享层不吞掉编排错误。能返回给客户端的错误在同一层完成 response 与 meta 收尾；内部记录失败但不中断响应的操作使用 `logx.WithContext(ctx)` 记录。

`writeGatewayError` 的写入错误仍不改变 response 语义，但 helper 会明确忽略并保持 artifact body 来自 marshaled bytes。

## Context 策略

新增 context 划分：

- `reqCtx := r.Context()`：客户端请求生命周期，用于 endpoint/router 查找、鉴权、hook session、provider 查询、模型 annotation 查询、attempt request 创建、bridge、web search 和 upstream RoundTrip。
- `persistCtx`：从 `reqCtx` 派生，带 bounded timeout，用于 request row 更新、trace upsert、cost lookup 和 artifact enqueue。它不会直接使用 `context.Background()`。超时时间使用现有 gateway read timeout 与最小下限组合，保证客户端取消后仍有短窗口完成 request row 收尾。
- `attemptCtx`：每次 attempt 从 `reqCtx` 派生，可由 idle timeout reader cancel，用于 upstream request、request bridge 和 response bridge。
- `server lifecycle ctx`：本次重构不修改 CLI 启动与 shutdown 行为；handler 内部不依赖 `context.Background()`。

`upsertProjectSeen` 改为接收 parent context：

```go
func (s *Server) upsertProjectSeen(ctx context.Context, projectID int32, seenAt time.Time)
```

调用端用 `go h.upsertProjectSeen(persistCtx, id, seenAt)`。函数内部继续设置 5 秒 timeout，但从传入 ctx 派生。

`insertRequest`、`updateRequestOnHeader`、`updateRequestModel`、`updateRequestOnComplete`、`completeFailedAttempt`、`costsFor`、artifact build log、aggregation 均使用 `persistCtx` 或 `attemptCtx`，不在 handler 中构造 `context.Background()`。

`artifacts.Sink.Put` 只是 enqueue，worker 内部上传仍保留自身 30 秒 timeout。该 worker 不属于请求路径中的 context misuse。

## 函数拆分目标

重构后的函数尺寸目标：

- `gatewayHandler.ServeHTTP`：少于 80 行。
- `Server.handleUnifiedGenerate` 返回的 handler：少于 80 行。
- `gatewayFlow.run`：少于 120 行，只串联生命周期。
- `runAttempts`：少于 160 行。
- route-specific candidate builder、model resolver、attempt preparer、success handler 各自少于 120 行。

这些限制用于保持 review 可读性，不作为运行时行为。

## API 与数据库

不新增管理 API、不修改 OpenAPI、不修改数据库 schema、不新增第三方依赖。

request table 写入语义保持不变：

- path gateway meta row 的 `endpoint_path` 记录 matched endpoint path。
- unified meta row 的 `endpoint_path` 记录 unified route path。
- upstream row 的 `endpoint_path` 记录实际 upstream endpoint path。
- `model` 记录 rewriteModel 后的 routed model。
- `upstream_model` 记录 attempt 最终发送的 upstream model。
- meta artifact 包含 JSX console logs，upstream artifact 不包含 logs。
- identity unified attempt 保持 byte-for-byte passthrough。

## 测试策略

新增单元测试覆盖新共享层的纯逻辑：

- candidate key：path 与 unified 的 sidecar lookup。
- hook delay 使用 context timer，context cancelled 时返回错误。
- provider sidecar 构造保持 annotations 合并顺序：model < provider < entry < apiKey。
- unknown JS-injected candidate 被跳过。
- unified request bridge disabled 时只让 cross-format attempt 失败，identity attempt 不触发 bridge。
- non-200 upstream response 完成 upstream row 的失败信息。
- hook error 映射：timeout 为 503，其余为 502。

保留并扩展现有 `handle_unified_gateway_test.go` 中的 helper 测试。项目没有 postgres-backed gateway integration harness；本计划通过不依赖真实 DB 的共享层单元测试覆盖可纯测逻辑。

## preRewriteBody 传递

Unified handler 在 rewriteModel 前保存一份 `preRewriteBody`，用于 web search emulation 的 `originalRequestBody`。共享编排层在 `resolveAndRewriteModel` 返回值中携带 `preRewriteBody []byte`：

- 当 rewriteModel hook 修改了 body 时，`preRewriteBody` 为 hook 执行前的 body 快照。
- 当 body 未被修改时，`preRewriteBody` 与当前 body 指向同一 slice（零拷贝）。

`gatewayFlow` 将 `preRewriteBody` 存储为字段，`PrepareAttempt` callback 可通过 `flow.preRewriteBody` 访问。Path gateway 的 `PrepareAttempt` 不使用该字段。

## Success 路径失败处理

当前 path handler 在 `streamSuccess` 内联处理了 "200 响应在读取/解码过程中失败" 的场景（`handle_gateway.go:676-687`），unified handler 将其抽为 `failUnifiedSuccess`。两者的逻辑一致：

1. 完成 upstream row（failed，携带实际 status code）。
2. 写 502 gateway error response。
3. 完成 meta row（failed，502）。
4. 上传 meta response artifact（含 logs）。
5. 关闭 upstream response body。

新增共享 helper `failSuccessPath(flow, attemptState, errMsg)`，在 `gateway_flow.go` 中实现。Path 和 unified 的 success handler 均调用此 helper 处理读取/解码/bridge 失败。

## 文件布局

新增：

- `pkg/server/gateway_flow.go`
- `pkg/server/gateway_flow_context.go`
- `pkg/server/gateway_flow_candidates.go`
- `pkg/server/gateway_flow_attempts.go`
- `pkg/server/gateway_flow_errors.go`
- `pkg/server/gateway_flow_test.go`

调整：

- `pkg/server/handle_gateway.go`
- `pkg/server/handle_unified_gateway.go`
- `pkg/server/gateway_helpers.go`

## 第三方库

不引入第三方库。
