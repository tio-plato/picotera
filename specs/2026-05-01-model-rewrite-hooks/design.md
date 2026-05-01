# Design

## 现状

`pkg/server/handle_gateway.go` 的处理顺序：

1. resolveEndpoint
2. 读 body
3. insert meta request
4. validateClientAuth
5. extractModel（从 body 抽出 modelName）
6. resolveProviders(endpointPath, modelName) → MPE 候选列表
7. 构建 jsx Session
8. retry loop：
   - sortProviders → beforeRequest → buildUpstreamRequest → rewriteRequest → forwardRequest

模型名一旦从 body 抽出来就直接用于 MPE 检索；upstream 模型名由 `mpe.upstreamModelName` 决定（为空则沿用 modelName），在 `buildUpstreamRequest` 内通过 sjson 写入 body.model。

beforeRequest 当前返回 `{next: bool, delay: number}`：决定是否跳过当前候选 / 是否延迟重试。

## 目标

- **新增 rewriteModel hook**：在 MPE 检索之前改写 modelName。
- **扩展 beforeRequest 返回值**：增加 `upstreamModel?: string` 字段，让 JS 在每次 attempt 决定 next/delay 的同时，也能改写最终发往 upstream 的模型名。

## Session 生命周期调整

rewriteModel 在 resolveProviders 之前需要 JS session。当前 session 在 resolveProviders 之后才创建，需要前移到 extractModel 之后、rewriteModel 之前。

调整后顺序：

1. resolveEndpoint
2. 读 body
3. insert meta request
4. validateClientAuth
5. extractModel
6. **建 session**（移上来）
7. **rewriteModel hook**
8. resolveProviders（用可能已改写的 modelName）
9. retry loop：sortProviders → **beforeRequest（返回值含 upstreamModel）** → buildUpstreamRequest（用 hook 给的 upstreamModel 写 body）→ rewriteRequest → forwardRequest

session 创建失败原本走 502 + meta artifact 上传（无 logs）；逻辑保持不变，仅位置上移。`collectLogs` 闭包对 nil session 的兼容已经写好。

## rewriteModel：数据流

```
extractModel(body) → modelName0
                  ↓
serializeClientRequest(r, body, modelName0) → clientRequest0
                  ↓
RunRewriteModelHook({request: clientRequest0}, modelName0) → modelName1
                  ↓
if modelName1 != modelName0:
    body = sjson.SetBytes(body, "model", modelName1)
    modelName = modelName1
                  ↓
resolveProviders(endpoint.Path, modelName)
                  ↓
（重新基于新 body 构造 jsClientRequest 给后续 hook）
```

`request` 字段是抽 model 时的快照——hook 看到的就是真正客户端原始数据。rewrite 完成后再统一替换 modelName 与 body.model。

后续 hook（sortProviders / beforeRequest / rewriteRequest）的 ctx.request 由 `serializeClientRequest(r, body, modelName)` 在新值下重新构造，`body.model` 与 `request.model` 一致。

## beforeRequest 扩展：数据流

retry loop 内每次进入新候选时：

```
cand = sortedCandidates[i]
                  ↓
upstreamModel0 := cand.mpe.upstreamModelName 非空 ? : modelName    // 默认入参
                  ↓
RunBeforeRequestHook(ctx) → { next, delay, upstreamModel? }
                  ↓
if dec.next: continue                       // 与当前一致
sleep(dec.delay)                             // 与当前一致
upstreamModel := dec.upstreamModel ?: upstreamModel0   // 新增分支
                  ↓
buildUpstreamRequest(ctx, r, body, side.upstreamURL, upstreamModel, side.credentials, endpoint.CredentialsResolver)
```

由于 hook 之前已经把 upstreamModel 用 modelName 兜底，传给 buildUpstreamRequest 永远是非空字符串，等同于"每次都用 sjson 强制把 body.model 设为最终 upstream 模型名"。这是行为变化但更直观——之前 mpe.upstreamModelName 为空时 body 不动是隐式约定，现在显式写入。

## Hook 签名

### rewriteModel（新）

```js
picotera.hooks.rewriteModel.tap('name', function (ctx, modelName) {
  // ctx = { request: ClientRequest }
  return 'new-model-name'   // 字符串；undefined / 同值 = 不改
})
```

ctx 仅含 `request`（同 `RequestShape`）；不暴露 endpoint / provider / mpe，因为此时还没有 MPE 检索。

### beforeRequest（扩展返回值）

```js
picotera.hooks.beforeRequest.tap('name', function (ctx) {
  // ctx 同现状
  return {
    next: false,
    delay: 0,
    upstreamModel: 'gpt-4o-2024-08-06',   // 新字段，可选
  }
})
```

- 不返回 / 返回不带 `upstreamModel` → 沿用 `cand.mpe.upstreamModelName ?: modelName` 计算出的默认值。
- 返回 `upstreamModel` 是非空字符串 → 替换为该值。
- 返回 `upstreamModel` 是空字符串、null、非字符串 → 视作"不改"，仍用默认值。

## Waterfall 返回值约定

### rewriteModel

字符串 waterfall：

- 每个 tap 接收 `(ctx, value)`，return 字符串 → 替换 value，return undefined → 沿用。
- 最终 value 是 Go 拿到的新 modelName。
- 与原值相等 → 走快路径，不重写 body 也不重新检索 MPE。

### beforeRequest

继续走对象 waterfall（与现状一致）：

- 每个 tap 接收 `(ctx, decision)`，可以 mutate 或 return 新对象。
- 最终 decision 是 `{next, delay, upstreamModel?}`。
- Go 侧合并字段：`next` 默认 `currentRetryCount > 0`，`delay` 默认 0，`upstreamModel` 默认空（走 fallback）。

## SDK 注册

`pkg/jsx/sdk.js` 在 `globalThis.picotera.hooks` 上加一个 Waterfall 实例：`rewriteModel`。`beforeRequest` 仍是原来的实例，无需改动。

## 错误处理

- Hook 抛错 / 超时 → 走现有 `failHook` 路径（502 / 503 + meta artifact 上传）。`runHookExpr` 已经把 timeout 标 tainted，session 后续 hook 都会快速失败。
- rewriteModel 返回非字符串 → SDK 表达式视作 `null`，Go 侧把它当"不改"，不打 warn（与 sortProviders 处理一致）。
- beforeRequest 返回 `upstreamModel` 是非字符串 → JSON 解码时类型不匹配则忽略该字段；Go 侧用 `*string` 接收，nil 即"不改"。

## 不涉及

- 新的 Go 测试（沿用现有 engine_test.go 的 jsx 单测）。
- HTTP API contract（不动 openapi.yaml）。
- 新的配置项（沿用 JSHookTimeout / JSMemoryLimit / JSMaxTotalAttempts）。
- 数据库 schema（不动）。
- 单独的 rewriteUpstreamModel hook（不再引入）。
