# 设计：JS 脚本功能重构

## 目标

1. 把底层 JS 运行时从 `github.com/fastschema/qjs`（wazero / WASM）换成
   `modernc.org/quickjs`（纯 Go、无 cgo、同步）。
2. 去掉 async：waterfall、`fetch` 全部改同步。
3. 为 JS 功能定义一套 Go 接口（`Engine` / `Session`），当前用进程内 QuickJS 实现，
   为将来改造成 go-plugin（参照 `pkg/llmbridge`）留好接缝。
4. 明确生命周期：每个 meta request 共享一个持久 `ctx` 全局对象，逐阶段写入字段，
   请求结束销毁。
5. 重构 hook 输入 / 输出：waterfall 值不再重复 ctx 字段，重复部分移到 ctx 读取。

## 三方依赖

### `modernc.org/quickjs`

纯 Go QuickJS（ES2023），无 cgo。关键 API：

- `NewVM() (*VM, error)` / `(*VM).Close()` —— VM 即 runtime+context，**非并发安全**。
- `(*VM).Eval(js string, EvalGlobal) (any, error)` —— 同步求值，返回原生 Go 值；
  JS object 返回 `*Object`，可 `.Into(&dst)`（走 `encoding/json`）或 `.MarshalJSON()`。
  这绕开了当前 qjs `JSONStringify()` 不可用（fastschema/qjs#44）而被迫走
  「hook 只返回字符串」的 workaround。
- `(*VM).RegisterFunc(name string, f any, wantThis bool) error` —— 注册同步宿主函数。
- `(*VM).SetMemoryLimit(uintptr)` / `SetGCThreshold(uintptr)` —— 内存限制。
- `(*VM).SetEvalTimeout(d time.Duration)` / `(*VM).Interrupt()` —— 内置超时 / 中断，
  超时返回 `InternalError: interrupted`。**替换**当前「goroutine + channel + cancel +
  tainted」那一整套手工超时逻辑。

替换 `fastschema/qjs`，从 `go.mod` 移除后者。

#### 超时的边界

`SetEvalTimeout` 通过解释器循环里的中断回调生效，**无法打断**正阻塞在宿主 Go 函数里的调用
（如 `picotera.fetch` 的 HTTP 请求）。因此：

- 纯 JS 死循环 / 长计算 —— `SetEvalTimeout` 能打断。
- 阻塞在 `fetch` —— 由 `fetchClient.Timeout`（5s，沿用现状）兜底。

每个 hook 求值前调用一次 `SetEvalTimeout(cfg.HookTimeout)`。超时后把 session 标记为
`tainted`，后续 hook 直接快速失败返回 `ErrHookTimeout`（语义与现状一致）。

## 去 async

- `sdk.js` 的 `Waterfall.runWaterfall` 改同步：去掉 `async/await`，依次同步调用各 tap，
  tap 返回非 `undefined` 即替换 waterfall 值。tap 函数签名仍是 `(ctx, value)`。
- `picotera.fetch(url, init)` 改同步：直接返回 `{status, headers, body}`，不再返回 Promise。
  宿主侧 `__picotera_fetch` 注册为同步函数，内部做阻塞 HTTP 调用并返回 JSON 字符串，
  SDK `JSON.parse` 后返回。
- 移除基于 Promise 的 `setTimeout` shim：JS 侧不需要延时（`beforeRequest` 的 `delay`
  在 Go 侧 `waitHookDelay` 执行）。按 CLAUDE.md「不写兼容层」，直接删除，不留 shim。
- `console.*`、`picotera.kv.*` 本就同步，语义不变。

## 接口接缝（为 go-plugin 预留）

仿照 `llmbridge.Bridge`：在 `pkg/jsx` 定义接口，`New*` 返回进程内实现，将来可加一个
基于 gRPC 的 plugin 实现而不动调用方。

```go
type Engine interface {
    NewSession(ctx context.Context, requestID string) (Session, error)
    Config() Config
}

type Session interface {
    // PatchContext 把 patch 里的字段浅合并到 globalThis.ctx（保留脚本挂的自定义字段）。
    PatchContext(patch ContextPatch) error

    RunRewriteModel(initial string) (string, error)
    RunSortProviders(initial []CandidateView) ([]CandidateView, error)
    RunBeforeRequest(initial BeforeRequestDecision) (BeforeRequestDecision, error)
    RunRewriteRequest(initial PendingRequestShape) (PendingRequestShape, error)
    RunBeforeTransform(initial OutboundProfile) (OutboundProfile, error)
    RunRewriteProviderModels(initial []ProviderModelEntry) ([]ProviderModelEntry, error)

    Logs() []LogEntry
    Close()
}
```

所有方法参数 / 返回值都是 JSON 可序列化类型，将来 gRPC plugin 直接传 JSON 字符串即可。
进程内实现 `qjsSession` 持有 `*quickjs.VM`；`PatchContext` 在 VM 内执行
`Object.assign(globalThis.ctx, <patch>)`；`Run*` 执行
`picotera.hooks.<name>.runWaterfall(globalThis.ctx, <initial>)`。

`Engine` / `Session` 由 `gateway_flow.go` 等调用方持有接口类型而非具体结构体。

## 生命周期与 ctx

### 持久 ctx

session 创建时注入 `globalThis.ctx`（所有字段初始 null），整个 meta request 期间是
**同一个对象**：宿主只浅合并（`Object.assign`）已确定的字段，不整体替换，从而保留脚本在
ctx 上挂的自定义状态。请求结束 `Close()` 销毁 VM。

### gateway flow 注入时序（`pkg/server/gateway_flow.go` / `gateway_flow_attempts.go`）

| 时机 | 动作 |
| --- | --- |
| `resolveAndRewriteModel`：建 session 后 | `PatchContext{endpointType, endpoint, requestModel, request, apiKey, annotations, stream, sourceFormat}` |
| 同上：跑 `rewriteModel` | `RunRewriteModel(routed)`；改写后 `PatchContext{routedModel, annotations, request}` |
| `resolveAndSortCandidates` | `RunSortProviders(candidates)`，无需额外 patch |
| 每次尝试前（`runSingleAttempt`） | `PatchContext{provider, providerModel, annotations, attempt}` 后 `RunBeforeRequest(...)` |
| 构造上游请求（`buildRewrittenUpstreamRequest`） | `RunRewriteRequest(pending)` |
| unified 出站 profile（`prepareUnifiedOutboundProfile`） | `RunBeforeTransform(profile)`（`stream`/`sourceFormat`/`providerModel.upstreamFormat` 已在 ctx） |
| `run()` 收尾 | `defer session.Close()`（现状已有） |

`provider` 轮换、`attempt` 计数变化时，都是再发一次 `PatchContext` 覆盖对应字段，体现
「字段变了就重写 ctx」。

### rewriteProviderModels（管理路由，独立）

`handle_provider_endpoint.go` 的 fetch-models 流程不在 meta request 生命周期内：单独
`NewSession` → `PatchContext{provider, annotations, upstreamResponse}`（model/request/apiKey/
attempt/endpoint 为 null）→ `RunRewriteProviderModels(aggregated)` → `Close()`。

## hook 数据结构

六个 hook 全部保留，重命名 / 重组为「ctx + waterfall 值」两段式。详细字段见 `api.md`。
要点：

- 每个 hook 不再有各自的大 `*Input` 结构；ctx 统一承载 endpoint / model / provider /
  providerModel / apiKey / annotations / attempt 等公共字段。
- waterfall 值 = 该 hook 真正要改写并返回的对象（去掉与 ctx 重复的字段）：
  `rewriteModel` 是模型名字符串，`sortProviders` 是候选列表，`beforeRequest` 是决策对象，
  `rewriteRequest` 是待发请求，`beforeTransform` 是出站 profile，`rewriteProviderModels`
  是模型配置项列表。
- `annotations`：每层（endpoint/routedModel/apiKey/provider/providerModel）各自带自身
  annotations；同时保留顶层 `ctx.annotations` 预合并便利 map（model+provider+entry+apiKey，
  后者覆盖前者），随各层填充逐阶段重算。
- `providerModel`：当前候选解析后的**单** endpoint 配置（`endpoint` 单数）。与
  `rewriteProviderModels` 用的 `ProviderModelEntry`（配置项，`endpoints` 复数）是两种结构，
  不要混淆。

## 受影响范围

- `pkg/jsx/`：engine.go、session.go、helpers.go、types.go、sdk.js 全面改写；新增接口定义。
- `pkg/server/`：`gateway_flow.go`、`gateway_flow_attempts.go`、`gateway_flow_candidates.go`、
  `hook_shapes.go`、`gateway_helpers.go`、`handle_provider_endpoint.go`、`handle_simulate.go`
  改为按 ctx + waterfall 值调用新接口。
- `pkg/contract/simulate.go`：模拟器输出沿用候选 / profile 结构，按新 `ProviderModel`
  字段命名同步。
- `go.mod`：`+ modernc.org/quickjs`，`- github.com/fastschema/qjs`。
- 测试：`pkg/jsx/engine_test.go` 重写为同步 + 新接口；`pkg/server` 相关测试随结构调整。

不引入任何兼容层 / 旧路径分支（遵循 CLAUDE.md）。
