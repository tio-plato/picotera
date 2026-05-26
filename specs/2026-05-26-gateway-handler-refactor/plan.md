# 执行计划：Gateway Handler 编排层重构

## Step 1: 建立共享 context 与 meta 状态

新增 `pkg/server/gateway_flow_context.go`：

1. 定义 `gatewayContexts`：

   ```go
   type gatewayContexts struct {
       Request context.Context
       Persist context.Context
       CancelPersist context.CancelFunc
   }
   ```

2. 新增 `newGatewayContexts(r *http.Request, cfg *configx.Config) gatewayContexts`：
   - `Request` 使用 `r.Context()`。
   - `Persist` 从 `Request` 派生 timeout。
   - timeout 使用 `cfg.GatewayReadTimeout` 与 `5*time.Second` 的较大值。
3. handler 结束时 `defer CancelPersist()`。
4. 将 handler 内所有 meta/upstream row 更新、trace/cost/artifact enqueue 改为使用 `Persist`。
5. 将 endpoint match、auth、model annotation、provider lookup、JS session 使用 `Request`。
6. 每个 attempt 使用 `context.WithCancel(Request)` 生成 `attemptCtx`。

## Step 2: 调整 project seen context

修改 `pkg/server/gateway_helpers.go`：

1. 将 `upsertProjectSeen(projectID int32, seenAt time.Time)` 改为 `upsertProjectSeen(ctx context.Context, projectID int32, seenAt time.Time)`。
2. 函数内部使用 `context.WithTimeout(ctx, 5*time.Second)`。
3. 两个 gateway 调用点改为 `go h.upsertProjectSeen(ctxs.Persist, projectID, metaCreatedAt)`。

## Step 3: 抽出 meta request 生命周期

新增 `pkg/server/gateway_flow.go`：

1. 定义 `gatewayFlow`、`gatewayFlowConfig`、`gatewayMetaState`。
2. 实现 `readGatewayBody()`：读取并关闭 `r.Body`，失败时写 500。
3. 实现 `insertMetaRequest(endpointPath string, endpointType int32)`：
   - 生成 meta id。
   - 复制请求 header、method、URL。
   - 提取 parent span、project、user message preview。
   - 调用 `insertRequest(ctxs.Persist, db.InsertRequestParams{...})`。
   - 上传 meta request artifact。
   - 异步 `upsertProjectSeen`。
4. 实现 `authenticateAndBackfill(resolver int32)`：
   - 调用 `authenticateClient(ctxs.Request, r, resolver)`。
   - 回填 meta row `api_key_id`。
   - 返回 `apiKeyID`、`apiKeyJS`、`apiKeyAnno`。
5. 实现 `updateMetaModel(model string)`，集中调用 `updateRequestModel`。

## Step 4: 统一错误与 hook 失败处理

新增 `pkg/server/gateway_flow_errors.go`：

1. 定义 `gatewayFailureWriter` 或在 `gatewayFlow` 上实现：
   - `collectLogs() []artifacts.LogEntry`
   - `failMeta(status int32, errMsg string)`
   - `failGatewayError(err error)`
   - `failHook(err error)`
   - `failInternal(status int, message string, code string)`
   - `failAllProviders(lastErr error)`
2. 所有 helper 使用 `ctxs.Persist` 更新 request row 与上传 artifact。
3. hook timeout 使用 503，其余 hook error 使用 502。
4. 普通 `gatewayError` 继续走 `handleGatewayErr` 语义。
5. 替换两个 handler 内重复的 `collectLogs`、`failMeta`、`failMetaResponse`、`failHook` closure。

## Step 5: 标准化 candidate 与 sidecar

新增 `pkg/server/gateway_flow_candidates.go`：

1. 定义 `gatewayCandidate`、`gatewayCandidateSidecar`、`candidateSet`。
2. 实现 `buildPathCandidateSet(providers []providerCandidateRow, apiKeyAnno map[string]string, modelAnno map[string]string, endpoint db.Endpoint) (candidateSet, error)`。
3. 实现 `buildUnifiedCandidateSet(providers []db.GetProvidersByEndpointTypesAndModelRow, apiKeyAnno map[string]string, modelAnno map[string]string, virtualEndpoint db.Endpoint) (candidateSet, error)`。
4. 抽出共同的 annotations decode 与 `jsx.Candidate` 构造。
5. 实现 `candidateSidecarMap(kind gatewayRouteKind, set candidateSet) map[string]gatewayCandidateSidecar`。
6. 实现 `lookupCandidateSidecar(kind gatewayRouteKind, sidecars map[string]gatewayCandidateSidecar, cand jsx.Candidate) (gatewayCandidateSidecar, bool)`。
7. path key 使用 provider id；unified key 使用 `providerID|endpointPath`。
8. 保持 unknown JS-injected candidate 的行为：跳过，不报错。

## Step 6: 抽出 model 解析与 rewriteModel 流程

在 `gateway_flow.go` 实现 `resolveAndRewriteModel`：

1. route config 提供 `ExtractModelAndMode`：
   - path gateway：endpoint.ModelPath 为空时返回空 model 和 `streaming=false`；否则调用 `extractModel`。
   - unified gateway：调用 `extractUnifiedModelAndStream`。
2. 插入/更新 meta model。
3. 创建 JSX session：`h.jsxEngine.NewSession(ctxs.Request, meta.ID)`。
4. 构造 initial `serializeClientRequest`。
5. 获取 model annotations。
6. 在调用 `RunRewriteModelHook` 前，保存 `preRewriteBody = body`（slice header 拷贝，零拷贝）。
7. 执行 `RunRewriteModelHook`。
8. route config 提供 `SetBodyModel`：
   - path gateway：使用 `sjson.SetBytes(body, "model", newModel)`；no-model endpoint 禁止 hook 返回非空 model。
   - unified gateway：使用 `setUnifiedModel(srcFormat, body, newModel)`。
9. 当 body 被修改时，`preRewriteBody` 自然指向旧 slice；未修改时两者相同。
10. rewrite 后刷新 model annotations 并更新 meta model。
11. 将 `preRewriteBody` 存入 `gatewayFlow.preRewriteBody` 字段。
12. 返回 `originalModel`、`routedModel`、`streaming`、`modelAnno` 和 updated body。

## Step 7: 抽出 sortProviders 流程

在 `gateway_flow.go` 实现 `resolveAndSortCandidates`：

1. route config 提供 `ResolveCandidates(ctx, routedModel, streaming)`：
   - path gateway 调用 `resolveProviders(ctx, endpoint.Path, routedModel)` 并 `buildPathCandidateSet`。
   - unified gateway 计算 `typeSet := candidateEndpointTypes(srcFormat, streaming)`，调用 `resolveProvidersByTypes` 并 `buildUnifiedCandidateSet`。
2. 如果 SQL 结果携带 model annotations，以 route SQL 结果刷新 request-scoped model annotations。
3. 构造 `modelJS`、`endpointJS`、`jsClientRequest`。
4. 调用 `session.RunSortHook`。
5. 返回 sorted candidates、sidecar map 和 JS context bundle。

## Step 8: 抽出 attempt loop

新增 `pkg/server/gateway_flow_attempts.go`：

1. 定义：

   ```go
   type attemptInput struct { ... }
   type attemptPrepared struct { ... }
   type successInput struct { ... }
   ```

2. 实现 `runAttempts(sorted []jsx.Candidate, sidecars map[string]gatewayCandidateSidecar, js gatewayJSContext) bool`：
   - 维护 `i`、`currentRetryCount`、`totalAttemptCount`、`lastErr`、`lastJSErr`。
   - 执行 `beforeRequest`。
   - delay 用 `time.NewTimer` 和 `select { case <-timer.C; case <-ctxs.Request.Done(): ... }`，不使用不可取消的 `time.Sleep`。
   - `dec.Next` 跳到下一个 candidate。
   - 生成 attempt context。
   - 插入 upstream request row。
   - 构建 upstream request。
   - 执行 `rewriteRequest`。
   - 调用 route-specific `PrepareAttempt`。
   - 上传 upstream request artifact。
   - 调用 `forwardRequest`。
   - 200 时调用 route-specific `HandleSuccess` 并返回 true。
   - 非 200 时读取 decoded body、上传 response artifact、完成 upstream failed row、更新 last error。
3. 将重复的 “attempt 失败并更新 lastJSErr” 抽成 `recordAttemptFailure`。
4. 确保每个 continue 前调用 attempt cancel。
5. 确保 response body 在所有非成功路径关闭。

## Step 9: 实现 path route 配置

重写 `pkg/server/handle_gateway.go`：

1. `ServeHTTP` 保留：
   - `gatewayStart := time.Now()`。
   - `resolveEndpoint(ctx, path)`。
   - route not found 的 dashboard fallback。
   - model-list endpoint 调 `handleModelList`。
   - 构造 `gatewayFlowConfig` 并调用 `flow.run()`。
2. path config：
   - `Kind: gatewayRoutePath`
   - `Endpoint: endpoint`
   - `PathVars: pathVars`
   - `ExtractModelAndMode` 使用 endpoint.ModelPath。
   - `SetBodyModel` 使用 `sjson.SetBytes`。
   - `ResolveCandidates` 使用 `resolveProviders`。
   - `PrepareAttempt` 直接返回 rewritten pending request。
   - `HandleSuccess` 调用 path success handler。
3. 将 path success handler 改为结构体参数，例如 `pathStreamSuccessArgs`。
4. 删除 `ServeHTTP` 内部大块 closure 与 retry loop。

## Step 10: 实现 unified route 配置

重写 `pkg/server/handle_unified_gateway.go` 的 handler 主体：

1. `handleUnifiedGenerate(srcFormat)` 只构造 `virtualEndpoint` 和 `gatewayFlowConfig`。
2. unified config：
   - `Kind: gatewayRouteUnified`
   - `Endpoint: virtualEndpoint`
   - `PathVars: chiURLParams(r)`
   - `SourceFormat: srcFormat`
   - `ExtractModelAndMode` 使用 `extractUnifiedModelAndStream`。
   - `SetBodyModel` 使用 `setUnifiedModel`。
   - `ResolveCandidates` 使用 `candidateEndpointTypes` + `resolveProvidersByTypes`。
   - `PrepareAttempt` 执行 source body model rewrite、web search emulation、beforeTransform、llmbridge request bridge。
   - `HandleSuccess` 调用 unified success handler。
3. 保留 helper 函数：
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
4. 将 `unifiedStreamArgs` 精简为组合共享 `successInput` 与 unified transform state。

## Step 11: 收敛 success completion 逻辑

新增共享 helper：

1. `markHeadersReceived(successInput, metaEndpointPath, upstreamEndpointPath)`：
   - 更新 meta row header received。
   - 更新 upstream row header received。
2. `completeSuccessfulRequests(successInput, metrics, statusCode)`：
   - 计算 cost。
   - 完成 upstream row。
   - 完成 meta row。
3. `failSuccessPath(flow, attemptState, errMsg)`：处理 200 响应在读取/解码/bridge 过程中失败的场景：
   - 完成 upstream row（failed，携带实际 upstream status code）。
   - 写 502 gateway error response 到客户端。
   - 完成 meta row（failed，502）。
   - 上传 meta response artifact（含 JSX console logs）。
   - 关闭 upstream response body。
4. path `streamSuccess` 与 unified `unifiedStreamSuccess` 使用 `markHeadersReceived` 和 `completeSuccessfulRequests`。
5. path `streamSuccess` 与 unified `unifiedStreamSuccess` 的解码/bridge 失败路径改为调用 `failSuccessPath`，替代各自内联的失败处理（path: `handle_gateway.go:676-687`，unified: `failUnifiedSuccess`）。
6. 保持 path response aggregation 只在 `responseAggregationFormat(endpointType)` 支持时执行。
7. 保持 unified upstream artifact 使用 upstream format aggregation，meta artifact 使用 source format aggregation。

## Step 12: 清理 background context 使用

搜索并处理：

```bash
rg -n "context\\.Background\\(" pkg/server
```

要求：

- `handle_gateway.go` 和 `handle_unified_gateway.go` 不再出现 `context.Background()`。
- `gateway_helpers.go` 的 `upsertProjectSeen` 不再直接从 background 派生。
- tests 中的 `context.Background()` 保留。
- `artifacts` worker 内部 timeout 保留。
- `cmd/picotera/main.go` 启动 context 不在本次范围内。

## Step 13: 添加测试

新增或扩展 `pkg/server/gateway_flow_test.go`：

1. `TestGatewayCandidateSidecarLookupPath`。
2. `TestGatewayCandidateSidecarLookupUnified`。
3. `TestGatewayUnknownCandidateSkipped`。
4. `TestGatewayDelayRespectsContextCancellation`。
5. `TestGatewayHookErrorStatusMapping`。
6. `TestBuildPathCandidateSetAnnotations`。
7. `TestBuildUnifiedCandidateSetAnnotationsAndFormat`。
8. `TestRecordAttemptFailureLastError`。

扩展 `pkg/server/handle_unified_gateway_test.go`：

1. 保留现有 format/type helper 测试。
2. 增加 identity unified attempt 在 bridge disabled 状态下保持直通的纯 helper 测试。
3. 增加 cross-format disabled bridge 返回清晰错误的 attempt preparation 测试。

## Step 14: 运行格式化与测试

执行：

```bash
gofmt -w pkg/server/handle_gateway.go pkg/server/handle_unified_gateway.go pkg/server/gateway_helpers.go pkg/server/gateway_flow*.go
go test ./pkg/server
go test ./pkg/llmbridge ./pkg/llmbridgeimpl ./pkg/artifacts
go test ./...
go build ./cmd/picotera
```

如果 `go test ./...` 因 TinyGo/WASM 产物、外部服务或本地环境失败，记录具体失败命令和错误。

## Step 15: 人工回归检查

用本地配置手动覆盖：

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
