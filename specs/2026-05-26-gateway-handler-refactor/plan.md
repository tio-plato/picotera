# 执行计划：Gateway Handler 编排层重构

## Step 1: 建立 context 基础

新增 `pkg/server/gateway_flow_context.go`：

1. 定义 `gatewayContexts`：

   ```go
   type gatewayContexts struct {
       Request       context.Context
       Persist       context.Context
       CancelPersist context.CancelFunc
   }
   ```

2. 实现 `newGatewayContexts(r *http.Request, cfg *configx.Config) gatewayContexts`：
   - `Request = r.Context()`。
   - `Persist` 基于 `context.WithoutCancel(r.Context())`。
   - `Persist` 使用 bounded timeout：`max(cfg.GatewayReadTimeout, 5*time.Second) + 2*time.Second`；未配置时为 7 秒。
3. `gatewayFlow.run` 开始时创建 contexts，并在返回前 `defer CancelPersist()`。
4. endpoint match、auth、model annotation、provider lookup、JS session、bridge、web search、upstream request 使用 `Request` 或 attempt context。
5. request row 更新、trace upsert、cost lookup、artifact enqueue 使用 `Persist`。
6. 每个 attempt 使用 `context.WithCancel(ctxs.Request)` 生成 `attemptCtx`。

## Step 2: 调整 project seen context

修改 `pkg/server/gateway_helpers.go`：

1. 将 `upsertProjectSeen(projectID int32, seenAt time.Time)` 改为 `upsertProjectSeen(ctx context.Context, projectID int32, seenAt time.Time)`。
2. 函数内部使用 `context.WithTimeout(ctx, 5*time.Second)`。
3. gateway 调用点改为 `go h.upsertProjectSeen(flow.ctxs.Persist, projectID, metaCreatedAt)`。
4. 保留错误吞掉并 warn log 的行为。

## Step 3: 定义 flow 状态与 route config

新增 `pkg/server/gateway_flow.go`：

1. 定义 `gatewayRouteKind`：
   - `gatewayRoutePath`
   - `gatewayRouteUnified`
2. 定义 `gatewayFlow`，包含 handler、response writer、request、startedAt、contexts、config、body、preRewriteBody、meta/auth/model/session。
3. 定义 `gatewayFlowConfig`，包含：
   - route kind
   - endpoint
   - path vars
   - source format
   - credentials resolver
   - `ExtractModel`
   - `SetBodyModel`
   - `ResolveCandidates`
   - `PrepareAttempt`
   - `HandleSuccess`
4. 定义 `gatewayMetaState`、`gatewayAuthState`、`gatewayModelState`、`gatewayModelMode`。
5. 实现 `newGatewayFlow(h, w, r, startedAt, config)`。
6. 实现 `(*gatewayFlow).run()`，只串联：
   - `readBody`
   - `insertMetaRequest`
   - `authenticateAndBackfill`
   - `resolveAndRewriteModel`
   - `resolveAndSortCandidates`
   - `runAttempts`
   - `failAllProviders`
7. 保证 `run` 不内联 route-specific 细节。

## Step 4: 抽出 meta request 生命周期

在 `gateway_flow.go` 实现：

1. `readBody()`：
   - `io.ReadAll(f.r.Body)`。
   - close body。
   - 失败时写 500；此时 meta row 尚未创建。
2. `insertMetaRequest()`：
   - 生成 meta id 与 created_at。
   - clone request header。
   - 提取 parent span。
   - 提取 user message preview。
   - 提取 project id。
   - 调用 `insertRequest(f.ctxs.Persist, db.InsertRequestParams{...})`。
   - 上传 meta request artifact。
   - 有 project id 时异步 `upsertProjectSeen(f.ctxs.Persist, id, metaCreatedAt)`。
3. `authenticateAndBackfill()`：
   - 调用 `authenticateClient(f.ctxs.Request, f.r, f.config.Credentials)`。
   - 保存 `apiKeyID`、`apiKeyJS`、`apiKeyAnno`。
   - 回填 meta row 的 `api_key_id`。
4. `updateMetaModel(model string)`：
   - 使用 `f.ctxs.Persist` 调 `updateRequestModel`。

## Step 5: 集中错误处理

新增 `pkg/server/gateway_flow_errors.go`：

1. 实现 `collectLogs() []artifacts.LogEntry`。
2. 实现 `failMeta(status int32, errMsg string)`。
3. 实现 `failGatewayError(err error)`：
   - 用 `handleGatewayErr` 写 response。
   - 更新 meta failed row。
   - 上传 meta response artifact。
4. 实现 `failHook(err error)`：
   - `errors.Is(err, jsx.ErrHookTimeout)` 返回 503。
   - 其他 hook 错误返回 502。
   - 更新 meta failed row。
   - 写 JSON error。
   - 上传 meta response artifact，包含 logs。
5. 实现 `failInternal(status int, message string, code string)`。
6. 实现 `failAllProviders(lastErr error)`。
7. 实现 `failSuccessPath(input successInput, errMsg string)`。
8. 删除两个 handler 内部重复的 `collectLogs`、`failMeta`、`failMetaResponse`、`failHook` closure。

## Step 6: 标准化 candidate 与 sidecar

新增 `pkg/server/gateway_flow_candidates.go`：

1. 定义 `gatewayCandidate`、`gatewayCandidateSidecar`、`candidateSet`。
2. 实现 `buildPathCandidateSet(providers []providerCandidateRow, apiKeyAnno map[string]string, modelAnno map[string]string, endpoint db.Endpoint) (candidateSet, error)`。
3. 实现 `buildUnifiedCandidateSet(providers []db.GetProvidersByEndpointTypesAndModelRow, apiKeyAnno map[string]string, modelAnno map[string]string, virtualEndpoint db.Endpoint) (candidateSet, error)`。
4. 抽出 `buildJSProviderSummary` 与 `buildJSMPE`，减少 path/unified 重复。
5. 实现 `candidateSidecarMap(set candidateSet) map[string]gatewayCandidateSidecar`。
6. 实现 `candidateKey(kind gatewayRouteKind, cand jsx.Candidate) string`：
   - path：provider id。
   - unified：`providerID|endpointPath`。
7. 实现 `lookupCandidateSidecar(kind gatewayRouteKind, sidecars map[string]gatewayCandidateSidecar, cand jsx.Candidate) (gatewayCandidateSidecar, bool)`。
8. unknown JS-injected candidate 保持 skip 行为，不写失败 row。
9. 保持 annotation merge 顺序 `model < provider < entry < apiKey`。

## Step 7: 抽出 model 解析与 rewriteModel

在 `gateway_flow.go` 实现 `resolveAndRewriteModel()`：

1. 调用 route config `ExtractModel(f.r, f.body, f.config.PathVars)`。
2. 保存 `originalModel`、`routedModel`、`streaming`。
3. 调 `updateMetaModel` 写入初始 model。
4. 创建 JSX session：`h.jsxEngine.NewSession(f.ctxs.Request, f.meta.ID)`。
5. 构造 initial `serializeClientRequest`。
6. 获取 model annotations。
7. 在 `RunRewriteModelHook` 前保存 `f.preRewriteBody = f.body`。
8. 调用 `RunRewriteModelHook`。
9. path no-model endpoint 若 hook 返回非空 model，调用 `failHook` 并停止 flow。
10. model 变化时调用 route config `SetBodyModel`。
11. `SetBodyModel` 失败时写 500 internal error 并上传 meta artifact。
12. rewrite 成功后刷新 model annotations，并调用 `updateMetaModel`。
13. 更新 `f.body`、`f.model`。

## Step 8: 抽出 sortProviders 流程

在 `gateway_flow.go` 实现 `resolveAndSortCandidates()`：

1. 调 route config `ResolveCandidates(f.ctxs.Request, f.model.Mode, f.auth)`。
2. 若 candidate set 携带 model annotations，用它覆盖 `f.model.Annotations`。
3. 构造 `gatewayJSContext`：
   - endpoint summary
   - model summary
   - client request
   - API key summary
   - merged model/apiKey annotations
4. 调用 `session.RunSortHook`。
5. 返回 sorted candidates、sidecar map 和 JS context。
6. hook 错误走 `failHook`。

## Step 9: 抽出 attempt loop

新增 `pkg/server/gateway_flow_attempts.go`：

1. 定义 `gatewayJSContext`、`attemptInput`、`attemptState`、`attemptPrepared`、`successInput`、`attemptResult`。
2. 实现 `runAttempts(sorted []jsx.Candidate, sidecars map[string]gatewayCandidateSidecar, js gatewayJSContext) attemptResult`。
3. `runAttempts` 只保留 retry skeleton，具体步骤拆成小函数：
   - `nextCandidateAttempt`
   - `runBeforeRequest`
   - `waitHookDelay`
   - `insertUpstreamAttempt`
   - `buildRewrittenUpstreamRequest`
   - `handleUpstreamResponse`
   - `recordAttemptFailure`
4. `waitHookDelay` 使用 `time.NewTimer` 和 `select` 监听 `f.ctxs.Request.Done()`。
5. attempt context 在每个 continue 或 return 前 cancel。
6. `recordAttemptFailure` 完成 upstream failed row，并返回新的 `lastErr` 与 `jsx.LastError`。
7. non-200 response 统一在 `handleUpstreamNonOK` 读取 decoded body、关闭 body、上传 upstream response artifact、完成 upstream failed row。
8. 200 response 调 route config `HandleSuccess(successInput)` 并返回 handled。
9. attempt cap 使用 `h.config.JSMaxTotalAttempts`，行为保持不变。

## Step 10: 实现 path route config

重写 `pkg/server/handle_gateway.go`：

1. `ServeHTTP` 保留以下逻辑并控制在 100 行以内：
   - 记录 start。
   - `resolveEndpoint(r.Context(), r.URL.Path)`。
   - route miss 且 browser nav 时调用 static handler。
   - gateway error 时写 structured error。
   - model-list endpoint 调 `handleModelList`。
   - 构造 path flow config。
   - 调 `newGatewayFlow(...).run()`。
2. 新增 `newPathGatewayFlowConfig(endpoint, pathVars)`。
3. path `ExtractModel`：
   - endpoint `ModelPath == ""` 返回空 model 和 `streaming=false`。
   - 其他情况调用 `extractModel`。
4. path `SetBodyModel` 使用 `sjson.SetBytes(body, "model", newModel)`。
5. path `ResolveCandidates`：
   - 调 `resolveProviders(ctx, endpoint.Path, routedModel)`。
   - 调 `buildPathCandidateSet`。
6. path `PrepareAttempt` 直接返回 rewritten request/body。
7. path `HandleSuccess` 调 `pathStreamSuccess`。
8. 删除原 `ServeHTTP` 内 retry loop 和 failure closure。

## Step 11: 实现 unified route config

重写 `pkg/server/handle_unified_gateway.go` 的 handler 主体：

1. `handleUnifiedGenerate(srcFormat)` 返回的 handler 控制在 100 行以内。
2. 新增 `newUnifiedGatewayFlowConfig(srcFormat, r)`。
3. virtual endpoint 固定为：

   ```go
   db.Endpoint{
       Name:                "(unified)",
       Path:                r.URL.Path,
       ModelPath:           "",
       CredentialsResolver: contract.CredentialsResolver_Unknown,
       EndpointType:        sourceEndpointType(srcFormat),
   }
   ```

4. unified `ExtractModel` 使用 `extractUnifiedModelAndStream`。
5. unified `SetBodyModel` 使用 `setUnifiedModel`。
6. unified `ResolveCandidates`：
   - 计算 `typeSet := candidateEndpointTypes(srcFormat, streaming)`。
   - 调 `resolveProvidersByTypes(ctx, model, typeSet, sourceEndpointType(srcFormat))`。
   - 调 `buildUnifiedCandidateSet`。
7. unified `PrepareAttempt` 拆成小函数：
   - `prepareUnifiedSourceBody`
   - `prepareUnifiedWebSearch`
   - `prepareUnifiedOutboundProfile`
   - `bridgeUnifiedRequest`
8. unified `HandleSuccess` 调 `unifiedStreamSuccess`。
9. 保留并复用 helper：
   - `sourceEndpointType`
   - `upstreamFormatFor`
   - `candidateEndpointTypes`
   - `extractUnifiedModelAndStream`
   - `setUnifiedModel`
   - `chiURLParams`
   - `resolveProvidersByTypes`
   - `dedupeUnifiedRows`
   - `betterRow`
   - `clientStreamContentType`
10. 删除 anonymous handler 内重复的 meta/auth/model/candidate/attempt 编排。

## Step 12: 拆分 success path

新增 `pkg/server/gateway_flow_success.go`，并调整现有 success 函数：

1. 定义 `successInput`，包含 flow、attempt state、response、provider id、routed model、upstream model、endpoint paths、upstream start time、API key id、route-specific prepared state。
2. 实现共享 helper：
   - `markHeadersReceived`
   - `completeGatewaySuccess`
   - `copyResponseHeaders`
   - `pipeReaderToClient`
3. path success 拆成：
   - `pathStreamSuccess`
   - `copyPathSuccessHeaders`
   - `openPathInternalReader`
   - `pipePathResponse`
   - `aggregatePathResponse`
4. unified success 拆成：
   - `unifiedStreamSuccess`
   - `copyUnifiedSuccessHeaders`
   - `openUnifiedInternalReader`
   - `buildUnifiedClientReader`
   - `applyWebSearchResponseTransform`
   - `pipeUnifiedResponse`
   - `aggregateUnifiedResponses`
5. path 和 unified 的 decode/bridge/transform 失败统一调用 `failSuccessPath`。
6. path aggregation 只在 `responseAggregationFormat(endpointType)` 支持时执行。
7. unified upstream artifact 使用 upstream format aggregation；meta artifact 使用 source format aggregation。
8. 每个 production 函数保持在 100 行以内。

## Step 13: 清理 context.Background

执行：

```bash
rg -n "context\\.Background\\(" pkg/server
```

处理要求：

1. `pkg/server/handle_gateway.go` 不出现 `context.Background()`。
2. `pkg/server/handle_unified_gateway.go` 不出现 `context.Background()`。
3. `pkg/server/gateway_helpers.go` 的 `upsertProjectSeen` 不从 background 派生。
4. tests 中的 `context.Background()` 保留。
5. `pkg/artifacts/sink.go` worker 内部 timeout 保留。
6. `cmd/picotera/main.go` 启动 context 不在本次范围。

## Step 14: 添加测试

新增或扩展 `pkg/server/gateway_flow_test.go`：

1. `TestGatewayCandidateSidecarLookupPath`。
2. `TestGatewayCandidateSidecarLookupUnified`。
3. `TestGatewayUnknownCandidateSkipped`。
4. `TestGatewayDelayRespectsContextCancellation`。
5. `TestGatewayHookErrorStatusMapping`。
6. `TestBuildPathCandidateSetAnnotations`。
7. `TestBuildUnifiedCandidateSetAnnotationsAndFormat`。
8. `TestRecordAttemptFailureLastError`。
9. `TestPersistContextSurvivesRequestCancelUntilTimeout`。
10. `TestPersistContextKeepsRequestValues`。

扩展 `pkg/server/handle_unified_gateway_test.go`：

1. 保留现有 format/type helper 测试。
2. 增加 identity unified attempt 在 bridge disabled 状态下保持直通的纯 helper 测试。
3. 增加 cross-format disabled bridge 返回清晰 attempt failure 的 helper 测试。

## Step 15: 函数尺寸检查

新增本地检查命令并在实现后执行：

```bash
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
    for i, (start, _) in enumerate(starts):
        end = starts[i + 1][0] if i + 1 < len(starts) else len(text)
        lines = text[start:end].count("\n") + 1
        if lines > 100:
            raise SystemExit(f"{p}:{text[:start].count(chr(10))+1}: function has {lines} lines")
PY
```

检查失败时继续拆分函数，直到通过。

## Step 16: 运行格式化与测试

执行：

```bash
gofmt -w pkg/server/handle_gateway.go pkg/server/handle_unified_gateway.go pkg/server/gateway_helpers.go pkg/server/gateway_flow*.go
go test ./pkg/server
go test ./pkg/llmbridge ./pkg/llmbridgeimpl ./pkg/artifacts
go test ./...
go build ./cmd/picotera
```

如果 `go test ./...` 或 `go build` 因 TinyGo/WASM 产物、外部服务或本地环境失败，记录具体失败命令和错误。

## Step 17: 人工回归检查

用本地配置覆盖：

1. path gateway identity 请求成功。
2. path gateway upstream 非 200 后 retry。
3. no-model endpoint 不允许 rewriteModel 返回非空 model。
4. unified Anthropic -> OpenAI non-stream bridge。
5. unified stream bridge。
6. unified identity attempt byte-for-byte passthrough。
7. Anthropic web search emulation route。
8. hook timeout 返回 503 并完成 meta failed row。
9. 客户端断开后 request row 在 persist timeout 窗口内完成失败或成功收尾。
10. meta artifact 包含 console logs，upstream artifact 不包含 logs。
