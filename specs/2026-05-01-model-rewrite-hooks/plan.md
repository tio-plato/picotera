# Plan

## 1. `pkg/jsx/sdk.js`

在 `globalThis.picotera.hooks` 加一个 Waterfall（仅 rewriteModel；beforeRequest 沿用现有实例）：

```js
hooks: {
  sortProviders: new Waterfall(),
  beforeRequest: new Waterfall(),
  rewriteRequest: new Waterfall(),
  rewriteModel: new Waterfall(),
}
```

## 2. `pkg/jsx/types.go`

新增：

```go
// RewriteModelInput is the ctx passed to the rewriteModel waterfall.
type RewriteModelInput struct {
    Request RequestShape `json:"request"`
}
```

扩展 `BeforeRequestDecision`：

```go
type BeforeRequestDecision struct {
    Next          bool   `json:"next"`
    Delay         int    `json:"delay"`
    UpstreamModel string `json:"upstreamModel"`   // 新增；空字符串 = 沿用默认
}
```

## 3. `pkg/jsx/session.go`

### 3.1 新增 `RunRewriteModelHook`

```go
func (s *Session) RunRewriteModelHook(in RewriteModelInput, modelName string) (string, error)
```

实现采用与 `RunRewriteHook` 类似的 JS 表达式：

```js
(async () => {
  const ctx = %s;
  const initial = %s;   // JSON-encoded modelName 字符串字面量
  const r = await picotera.hooks.rewriteModel.runWaterfall(ctx, initial);
  if (typeof r !== 'string') return null;
  return JSON.stringify(r);
})()
```

`runHookExpr` 返回 `"null"` 时函数返回入参 `modelName`；否则 `json.Unmarshal` 进 string。返回值与入参相等也照常返回——是否触发 body 重写由调用方判断。

### 3.2 修改 `RunBeforeRequestHook`

JS 表达式里把返回对象多带一个 `upstreamModel`：

```js
(async () => {
  const ctx = %s;
  const r = await picotera.hooks.beforeRequest.runWaterfall(ctx, { next: ctx.currentRetryCount > 0, delay: 0 });
  if (r === ctx || typeof r === 'undefined' || r === null) return null;
  const um = (typeof r.upstreamModel === 'string') ? r.upstreamModel : '';
  return JSON.stringify({ next: !!r.next, delay: r.delay || 0, upstreamModel: um });
})()
```

Go 侧 `json.Unmarshal` 进新版 `BeforeRequestDecision` 即可——`UpstreamModel` 字段空字符串视作"不改"。

passthrough 路径（jsonStr 为 `"null"`）保留现行 `if in.CurrentRetryCount > 0 { dec.Next = true }` 逻辑，`UpstreamModel` 自然为空。

## 4. `pkg/server/handle_gateway.go`

### 4.1 Session 创建上移

把"8. Build jsx session"那段（`session, err = h.jsxEngine.NewSession(...)` 及 `defer session.Close()` 与失败处理）从 step 7 之后剪切到 **step 5 extractModel 之后、resolveProviders 之前**。

session 创建失败的 502 路径不动（写 meta artifact 时 logs 仍是 nil）。

### 4.2 rewriteModel hook 调用

在 session 创建成功之后、`resolveProviders` 之前插入：

```go
initialClientReq := serializeClientRequest(r, body, modelName)

newModel, err := session.RunRewriteModelHook(jsx.RewriteModelInput{
    Request: initialClientReq,
}, modelName)
if err != nil {
    failHook(err)
    return
}
if newModel != modelName {
    updated, serr := sjson.SetBytes(body, "model", newModel)
    if serr != nil {
        errMsg := "failed to set model in body: " + serr.Error()
        failMeta(http.StatusInternalServerError, errMsg)
        respBody := writeGatewayError(w, http.StatusInternalServerError, errMsg, errorx.InternalError.Error())
        h.uploadMetaResponseArtifact(bgCtx, metaID, metaCreatedAt, http.StatusInternalServerError, w.Header().Clone(), respBody, collectLogs())
        return
    }
    body = updated
    modelName = newModel
}
```

需要在 `handle_gateway.go` 加 `import "github.com/tidwall/sjson"`（gateway_helpers.go 已 import，handle_gateway.go 当前未用到）。

### 4.3 jsClientRequest 重新序列化

现有 `jsClientRequest := serializeClientRequest(r, body, modelName)` 调用保持在 sortProviders 之前——此时的 body 与 modelName 都是 rewriteModel 后的最终值，所有后续 hook 看到一致快照。

### 4.4 retry loop 内：beforeRequest 决策消费 upstreamModel

修改 retry loop 内对 `dec` 的处理：

```go
dec, err := session.RunBeforeRequestHook(jsx.BeforeRequestInput{ ... })
if err != nil { failHook(err); return }
if dec.Delay > 0 { /* 同现状 sleep */ }
if dec.Next { i++; currentRetryCount = 0; continue }

// 计算待写入 upstream body 的 model 名
upstreamModel := dec.UpstreamModel
if upstreamModel == "" {
    upstreamModel = candidateUpstreamModel(cand)
}
if upstreamModel == "" {
    upstreamModel = modelName
}

attemptStart := time.Now()
ctx, cancel := context.WithCancel(r.Context())
// ... existing insertRequest, buildUpstreamRequest(... upstreamModel ...)
```

并删除原本 `upstreamModel := candidateUpstreamModel(cand)` 那一行（已经合并到上面）。

### 4.5 buildUpstreamRequest 调用

签名不变。传入的 `upstreamModel` 现在永远是非空字符串（hook 之前的 fallback 链保证）。`buildUpstreamRequest` 内 `if upstreamModel != ""` 分支永远命中——行为等价于"始终用 sjson 写 body.model"。注释里说明这一点；不强制改 if。

## 5. `pkg/jsx/engine_test.go`

新增四个用例：

- `TestSession_Hooks_RewriteModel_Passthrough`：脚本不 return，期望返回入参 modelName。
- `TestSession_Hooks_RewriteModel_Replace`：脚本 `return 'new-model'`，期望返回 `'new-model'`。
- `TestSession_Hooks_BeforeRequest_UpstreamModel`：脚本 `return { upstreamModel: 'foo' }`，期望 `dec.UpstreamModel == "foo"`、`Next == false`、`Delay == 0`。
- `TestSession_Hooks_BeforeRequest_UpstreamModel_NonString`：脚本 `return { upstreamModel: 42 }`，期望 `dec.UpstreamModel == ""`（视作不改）。

现有 BeforeRequest 用例（如有）保持，断言不要因为新字段失败——新字段缺省值是空字符串，Go 零值兼容。

## 6. 验证

- `go build ./...`
- `go test ./pkg/jsx/...`
- 手动 smoke：
  1. `docker compose up -d` → `mise run server`。
  2. 通过 `/api/picotera/scripts` 注入：
     ```js
     picotera.hooks.rewriteModel.tap('t1', (ctx, m) => m === 'foo' ? 'bar' : undefined)
     picotera.hooks.beforeRequest.tap('t2', (ctx) => ({ upstreamModel: ctx.request.model + '-pinned' }))
     ```
  3. 配一对 endpoint + provider + MPE，model="bar"，upstreamModelName 为空。客户端 POST `model=foo`。
  4. 期望：MPE 命中 model="bar"；upstream 收到 body `model=bar-pinned`。
  5. 关掉脚本再跑一次，期望 MPE 命中 model="foo"，upstream body model="foo"。
- 不需要 `mise run openapi`（不动 HTTP API）。

## 7. 文档/Memo

无 README / docs 更新。Hook 用法在 specs 目录内的 api.md。

## 8. 提交分块

1. `feat(jsx): add rewriteModel waterfall and upstreamModel field on beforeRequest decision`
2. `feat(gateway): wire rewriteModel before MPE lookup, consume beforeRequest.upstreamModel per attempt`
3. `test(jsx): cover rewriteModel and beforeRequest.upstreamModel`
