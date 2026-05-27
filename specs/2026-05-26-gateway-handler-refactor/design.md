# 设计：Gateway Handler 编排层重构

## 范围

本重构覆盖 `pkg/server/handle_gateway.go`、`pkg/server/handle_unified_gateway.go` 以及为承载共享编排而新增的 `pkg/server/gateway_flow*.go` 内部文件。

本重构不修改管理 API、不修改 OpenAPI、不修改数据库 schema、不新增第三方依赖、不加入兼容旧路径的分支。

## 目标

重构后的 gateway 代码满足以下条件：

- path-based gateway 与 unified gateway 共用 meta request、JS hook、candidate 排序、attempt retry、失败收尾、artifact 记录和 request row 更新流程。
- `gatewayHandler.ServeHTTP` 只负责 path endpoint 匹配、dashboard fallback、model-list 分支和启动共享 flow。
- `Server.handleUnifiedGenerate` 返回的 handler 只负责构造 unified virtual endpoint、route config 和启动共享 flow。
- `pkg/server/handle_gateway.go` 与 `pkg/server/handle_unified_gateway.go` 中不再出现 500 行级别的编排函数。
- 新增或重写的 production 函数均控制在 100 行以内；超过 100 行的 success/read/transform 路径必须继续拆分。
- 原先用编号注释串起来的大步骤改为具名小函数，注释只解释非显然的边界和错误语义。
- handler 请求路径不直接调用 `context.Background()`；DB、artifact、trace、cost、hook、bridge、upstream、web search 均使用明确的 request、persist 或 attempt context。
- 错误路径集中处理 response 写入、meta row 收尾和 meta artifact 上传，避免 duplicated closure 和遗漏。

## 当前结构问题

`gatewayHandler.ServeHTTP` 同时承担 endpoint 匹配、body 读取、meta request 插入、鉴权、model 提取、rewriteModel、provider 查询、candidate sidecar 构造、sortProviders、beforeRequest、rewriteRequest、upstream attempt 插入、forward、retry、成功流式写回、artifact 聚合、token/TTFT/cost 计算和失败收尾。函数体过长，逻辑顺序依赖编号注释维持可读性。

`handleUnifiedGenerate` 的匿名 handler 复制了同一生命周期，并额外混入 unified 专属的 source/upstream format 选择、beforeTransform、llmbridge、Anthropic web search emulation 和 response bridge。两者已有底层 helper，但缺少表达“单次 gateway 请求生命周期”的中层编排。

## 新结构

新增内部共享编排层：

- `pkg/server/gateway_flow.go`：flow 主流程、route config、model rewrite、sort hook。
- `pkg/server/gateway_flow_context.go`：request/persist/attempt context。
- `pkg/server/gateway_flow_errors.go`：meta 失败、hook 失败、all-providers 失败、success-path 失败。
- `pkg/server/gateway_flow_candidates.go`：标准化 candidate 与 sidecar 构造。
- `pkg/server/gateway_flow_attempts.go`：retry loop、attempt row、rewriteRequest、forward、non-200 处理。
- `pkg/server/gateway_flow_success.go`：header received、success completion、shared streaming helpers。

`gatewayFlow` 持有单次请求状态：

```go
type gatewayFlow struct {
    h              *gatewayHandler
    w              http.ResponseWriter
    r              *http.Request
    startedAt      time.Time
    ctxs           gatewayContexts
    config         gatewayFlowConfig
    body           []byte
    preRewriteBody []byte
    meta           gatewayMetaState
    auth           gatewayAuthState
    model          gatewayModelState
    session        *jsx.Session
}
```

`gatewayFlowConfig` 只描述 path/unified 差异：

```go
type gatewayFlowConfig struct {
    Kind              gatewayRouteKind
    Endpoint          db.Endpoint
    PathVars          map[string]string
    SourceFormat      llmbridge.Format
    Credentials       int32
    ExtractModel      func(*http.Request, []byte, map[string]string) (gatewayModelMode, error)
    SetBodyModel      func([]byte, string) ([]byte, error)
    ResolveCandidates func(context.Context, gatewayModelMode, gatewayAuthState) (candidateSet, error)
    PrepareAttempt    func(context.Context, *gatewayFlow, attemptInput) (attemptPrepared, error)
    HandleSuccess     func(successInput)
}
```

共享 flow 按固定顺序执行：

1. `readBody`
2. `insertMetaRequest`
3. `authenticateAndBackfill`
4. `resolveAndRewriteModel`
5. `resolveAndSortCandidates`
6. `runAttempts`
7. `failAllProviders`，仅在所有 attempt 都失败或被跳过后执行

`gatewayFlow.run` 只串联这些函数，不包含业务细节。

## Route-specific 边界

Path gateway 保留以下差异点：

- endpoint 来源于 `endpointRouter.Match`。
- model 来源于 endpoint `model_path` 或 path vars；no-model endpoint 跳过 body/path model 提取。
- provider 来源于 `resolveProviders(endpoint.Path, model)`。
- `PrepareAttempt` 不执行 format bridge。
- success path 按 endpoint type 做 response aggregation。

Unified gateway 保留以下差异点：

- endpoint 为 synthetic `db.Endpoint{Name: "(unified)", Path: r.URL.Path, EndpointType: sourceEndpointType(srcFormat)}`。
- model/stream 来源于 `extractUnifiedModelAndStream`。
- provider 来源于 `resolveProvidersByTypes(model, candidateEndpointTypes(srcFormat, streaming), sourceEndpointType(srcFormat))`。
- `PrepareAttempt` 执行 source body model rewrite、Anthropic web search emulation、beforeTransform、request bridge。
- success path 执行 upstream-format metric extraction、response bridge、web search response transform、source-format meta artifact aggregation。

## Candidate 与 sidecar

两个 handler 的候选构造统一为：

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
    Items     []gatewayCandidate
    ModelAnno map[string]string
}
```

Path key 为 decimal provider id。Unified key 为 `providerID|endpointPath`。`lookupCandidateSidecar` 根据 route kind 计算 key；JS hook 返回未知 provider 或未知 provider/path 时跳过该 candidate，并重置当前 candidate retry count。

annotation 合并顺序保持为 `model < provider < entry < apiKey`。SQL 路由结果携带 model annotations 时，使用该结果刷新 request-scoped model annotations，避免 rewrite 前后读到不同快照。

## Attempt 编排

`runAttempts` 维护以下状态：

- sorted candidates index
- `currentRetryCount`
- `totalAttemptCount`
- `lastErr`
- `lastJSErr`

每次 attempt 执行：

1. lookup sidecar，未知 sidecar 直接跳过。
2. `beforeRequest` hook。
3. hook delay 使用可取消 timer。
4. `dec.Next` 跳过当前 candidate。
5. 创建 `attemptCtx`。
6. 选择 upstream model：hook `upstreamModel`、MPE `upstreamModelName`、routed model。
7. 插入 upstream request row。
8. `buildUpstreamRequest`。
9. `rewriteRequest` hook。
10. `PrepareAttempt` route callback。
11. 上传 upstream request artifact。
12. `forwardRequest`。
13. 200 调用 `HandleSuccess` route callback 并结束 flow。
14. 非 200 读取 decoded body，上传 upstream response artifact，完成 upstream failed row，更新 `lastErr` 和 `lastJSErr`。

所有 attempt 失败函数通过 `recordAttemptFailure` 更新 request row 与 last error，并在返回前 cancel attempt context。所有 non-200 response body 在读取后关闭。

## Context 策略

`gatewayContexts` 由 `newGatewayContexts(r, cfg)` 创建：

```go
type gatewayContexts struct {
    Request       context.Context
    Persist       context.Context
    CancelPersist context.CancelFunc
}
```

语义如下：

- `Request` 等于 `r.Context()`，用于 endpoint/router 查找、鉴权、JS session、provider 查询、model annotation 查询、attempt 创建、bridge、web search 和 upstream RoundTrip。
- `Persist` 使用 `context.WithoutCancel(r.Context())` 继承 request values，再加 bounded timeout；用于 request row 更新、trace upsert、cost lookup、artifact enqueue 和 success/failure 收尾。它不受客户端断开直接取消，也不会无限运行。
- `attemptCtx` 每次从 `Request` 派生，由 idle timeout reader 或 attempt 收尾 cancel；用于 upstream request、request bridge、response bridge 和 web search transform。

`Persist` timeout 为 `max(config.GatewayReadTimeout, 5*time.Second)`，再加 `2*time.Second` 收尾余量；当 `GatewayReadTimeout` 未配置或小于 5 秒时使用 7 秒。

`upsertProjectSeen` 改为：

```go
func (s *Server) upsertProjectSeen(ctx context.Context, projectID int32, seenAt time.Time)
```

函数内部从传入 ctx 派生 5 秒 timeout。handler 调用 `go h.upsertProjectSeen(flow.ctxs.Persist, id, metaCreatedAt)`。

`artifacts.Sink.Put` 内部 worker 的 `context.Background()` 保留；它属于 sink worker 生命周期，不属于 handler 请求路径。

## 错误处理

`gatewayFlow` 提供集中失败函数：

- `collectLogs`
- `failMeta`
- `failGatewayError`
- `failHook`
- `failInternal`
- `failAllProviders`
- `failSuccessPath`

规则：

- `gatewayError` 继续使用原有 HTTP status、message、code。
- hook timeout 返回 503。
- 其他 hook 错误返回 502。
- body 读取失败返回 500，并且在 meta row 尚未创建时不写 request row。
- meta row 创建后的所有客户端可见失败都更新 meta row 为 failed，并上传 meta response artifact。
- meta response artifact 包含 JSX console logs；upstream artifact 不包含 logs。
- all-providers 失败返回 502，message 使用最后一个 attempt 错误；没有 last error 时使用 `all providers failed`。

Success path 中已经拿到 upstream 200 后发生解码、bridge、web search transform 或客户端写回前准备失败时，不再 retry；`failSuccessPath` 完成 upstream failed row、meta failed row、502 response 和 meta artifact。

## Success 拆分

Path success 拆成下列函数，单个函数 100 行以内：

- `pathStreamSuccess`
- `copyPathSuccessHeaders`
- `openPathInternalReader`
- `pipePathResponse`
- `aggregatePathResponse`
- `completeGatewaySuccess`

Unified success 拆成下列函数，单个函数 100 行以内：

- `unifiedStreamSuccess`
- `copyUnifiedSuccessHeaders`
- `openUnifiedInternalReader`
- `buildUnifiedClientReader`
- `applyWebSearchResponseTransform`
- `pipeUnifiedResponse`
- `aggregateUnifiedResponses`
- `completeGatewaySuccess`

`completeGatewaySuccess` 共享 header received、metrics 转 pgtype、cost lookup、upstream complete、meta complete 逻辑。

## 保持的行为

request table 写入语义保持不变：

- path meta row 的 `endpoint_path` 记录 matched endpoint path。
- unified meta row 的 `endpoint_path` 记录 unified route path。
- upstream row 的 `endpoint_path` 记录实际 upstream endpoint path。
- `model` 记录 rewriteModel 后的 routed model。
- `upstream_model` 记录 attempt 最终发送的 upstream model。
- `project_id` 写入 meta row 和 upstream row。
- meta artifact 包含 JSX console logs。
- identity unified attempt 保持 byte-for-byte passthrough，不要求 llmbridge enabled。
- cross-format unified attempt 在 llmbridge disabled 时只失败该 attempt。

## 函数尺寸验收

重构完成后执行：

```bash
go test ./pkg/server
python3 - <<'PY'
import pathlib, re
files = [
    pathlib.Path("pkg/server/handle_gateway.go"),
    pathlib.Path("pkg/server/handle_unified_gateway.go"),
    *pathlib.Path("pkg/server").glob("gateway_flow*.go"),
]
for p in files:
    text = p.read_text()
    starts = [(m.start(), m.group(0)) for m in re.finditer(r"^func\b|^func \(", text, re.M)]
    for i, (start, sig) in enumerate(starts):
        end = starts[i + 1][0] if i + 1 < len(starts) else len(text)
        lines = text[start:end].count("\n") + 1
        if lines > 100:
            raise SystemExit(f"{p}:{text[:start].count(chr(10))+1}: function has {lines} lines")
PY
```

该检查只覆盖本重构新增或重写的 gateway production 文件。测试文件不纳入 100 行函数限制。

## 测试策略

新增单元测试覆盖共享层纯逻辑：

- path sidecar lookup。
- unified sidecar lookup。
- unknown JS-injected candidate skip。
- hook delay 遵守 context cancellation。
- hook error status mapping。
- path candidate annotation merge。
- unified candidate annotation merge 与 upstream format。
- attempt failure 更新 `jsx.LastError`。
- cross-format unified attempt 在 llmbridge disabled 时返回清晰 attempt failure。
- identity unified attempt 在 llmbridge disabled 时不触发 bridge。

保留并扩展 `handle_unified_gateway_test.go` 中的 format/type helper 测试。项目没有 postgres-backed gateway integration harness，本重构通过共享层可纯测逻辑和现有 helper 测试控制风险。
