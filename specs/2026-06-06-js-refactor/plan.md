# 执行计划：JS 脚本功能重构

按以下顺序实施；每个阶段结束应能 `go build ./...` 通过（除明确标注的过渡点）。

## 阶段 0：依赖切换

1. `go get modernc.org/quickjs@latest`，确认进入 `go.mod`。
2. 先不删 `github.com/fastschema/qjs`（旧实现还在引用），阶段 4 末统一移除。

## 阶段 1：`pkg/jsx` 类型与接口

3. 在 `pkg/jsx/types.go` 重写共享类型，对齐 `api.md`：
   - 保留 / 调整：`EndpointSummary`、`ModelSummary`、`RequestShape`、`ApiKeySummary`、
     `ProviderSummary`（去掉 `ProviderModels` 字段，dispatch 不再用）、`LastError`、
     `BeforeRequestDecision`、`PendingRequestShape`、`OutboundProfile`、
     `ProviderModelEntry`。
   - 新增：`ProviderModel`（单 endpoint）、`AttemptState`、`CandidateView`、`ContextPatch`。
   - 删除：旧的 `Candidate`、`CandidateMPE`、`SortInput`、`BeforeRequestInput`、
     `RewriteModelInput`、`RewriteInput`、`BeforeTransformInput`、
     `RewriteProviderModelsInput`（被 ctx + waterfall 值取代）。
4. 新增 `pkg/jsx/iface.go`：定义 `Engine`、`Session` 接口（见 `design.md`）。

## 阶段 2：QuickJS 进程内实现

5. 重写 `pkg/jsx/engine.go`：`Engine` 接口的进程内实现 `qjsEngine`，`NewEngine(...)`
   返回 `Engine`；`NewSession` 返回 `Session`。
6. 重写 `pkg/jsx/session.go` 为 `qjsSession`：
   - 持有 `*quickjs.VM`；`newSession` 内 `NewVM` → `SetMemoryLimit` / `SetGCThreshold`
     → 注册宿主函数（阶段 3）→ 载入 `sdk.js` → 注入 `globalThis.ctx`（全 null）
     → 逐个 `Eval` 已启用脚本。
   - `PatchContext(patch)`：marshal patch → `Eval("Object.assign(globalThis.ctx, <json>)")`。
   - 六个 `Run*`：每次 `SetEvalTimeout(cfg.HookTimeout)` →
     `Eval("JSON of picotera.hooks.<name>.runWaterfall(globalThis.ctx, <initial>)")`，
     用 `*Object.Into` / `Value.MarshalJSON` 取回结果（不再依赖字符串 workaround）。
     超时 / 中断映射为 `ErrHookTimeout` 并 `tainted`，后续 `Run*` 快速失败。
   - 各 `Run*` 的 passthrough / 容错语义照 `api.md`（空数组、非 string、非 array 等）。
   - `Logs()`、`Close()` 沿用现状（日志缓冲逻辑不变）。
   - 删除 `runHookExpr` / `evalWithTimeout` 的 goroutine+channel 超时机制，改用
     `SetEvalTimeout` / `Interrupt`。
7. 重写 `pkg/jsx/helpers.go`：
   - `__picotera_fetch` 改同步（`RegisterFunc`，返回 JSON 字符串）。
   - 移除 `__picotera_setTimeout`。
   - `__picotera_kv_*`、`__picotera_console` 用 `RegisterFunc` 重新接上（语义不变）。
8. 重写 `pkg/jsx/sdk.js`：
   - `Waterfall.runWaterfall` 改同步。
   - `picotera.fetch` 改同步返回。
   - 删除 `setTimeout` shim。
   - `kv` / `console` 不变。
9. `pkg/jsx/store.go`、`validate.go` 按编译需要微调（接口签名变化处）。

## 阶段 3：server 调用方改造

10. `pkg/server/hook_shapes.go` / `gateway_flow_candidates.go`：
    - `buildJS*` 改为产出 `CandidateView`（`provider` + `providerModel(ProviderModel)` +
      `annotations`），`buildJSMPE` → `buildProviderModel`。
    - sidecar 里的 `UpstreamFormat` 在 unified 时写入 `ProviderModel.UpstreamFormat`。
11. `pkg/server/gateway_flow.go`：
    - `resolveAndRewriteModel`：建 session 后 `PatchContext`（endpointType/endpoint/
      requestModel/request/apiKey/annotations/stream/sourceFormat）；跑 `RunRewriteModel`；
      改写后再 `PatchContext`（routedModel/annotations/request）。
    - `resolveAndSortCandidates`：`RunSortProviders(candidates []CandidateView)`。
    - `gatewayJSContext` 结构精简 / 删除（公共字段移入 ctx，候选 sidecar 仍 Go 侧维护）。
12. `pkg/server/gateway_flow_attempts.go`：
    - 每次尝试 `PatchContext`（provider/providerModel/attempt/annotations）后
      `RunBeforeRequest`。
    - `buildRewrittenUpstreamRequest`：`RunRewriteRequest(pending)`。
    - `prepareUnifiedOutboundProfile`：`RunBeforeTransform(profile)`。
    - 删除各处旧 `*Input` 的组装代码。
13. `pkg/server/handle_provider_endpoint.go`：fetch-models 改为
    `PatchContext{provider, annotations, upstreamResponse}` + `RunRewriteProviderModels`。
14. `pkg/server/handle_simulate.go`：按新接口与 `ProviderModel` 命名同步模拟逻辑。
15. `pkg/contract/simulate.go`：`SimulateMPE` → 对齐 `ProviderModel` 字段（`endpoint` 单数等）。

## 阶段 4：清理与收口

16. 移除对 `github.com/fastschema/qjs` 的全部引用，`go mod tidy` 去掉该依赖。
17. `go build ./...` 全绿。

## 阶段 5：测试

18. 重写 `pkg/jsx/engine_test.go`：覆盖同步 waterfall、ctx 注入 / patch、各 hook 输入输出、
    超时（`SetEvalTimeout`）、内存限制、fetch / kv / console。
19. 调整 `pkg/server/gateway_flow_test.go` 等随结构变更的断言。
20. `go test ./pkg/jsx/... ./pkg/server/...` 通过。

## 阶段 6：OpenAPI / 前端（若 simulate 契约有变）

21. 若 `pkg/contract/simulate.go` 字段变更：`mise run openapi` +
    `pnpm --dir dashboard generate-openapi`，并检查 `ScriptsView`/模拟器调用处。

## 验收

- `go build ./...`、`go test ./pkg/jsx/... ./pkg/server/...` 通过。
- 网关 path / unified 两条链路、fetch-models、simulate 在新接口下行为与重构前一致。
- `go.mod` 不再含 `fastschema/qjs`；JS 侧无 async（waterfall / fetch 均同步）。
- 调用方持有 `jsx.Engine` / `jsx.Session` 接口；无任何兼容层 / 旧路径分支。
