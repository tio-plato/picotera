# JS API：rewriteRequest hook

## 注册

```js
picotera.hooks.rewriteRequest.tap(name, fn, priority?)
```

## 回调签名

```js
function (ctx, pendingRequest) → pendingRequest | undefined
```

### ctx 字段

| 字段 | 类型 | 说明 |
|---|---|---|
| `endpoint` | object | 命中的 endpoint 行（同现状） |
| `model` | object \| null | 当前 model 行（v1 始终 null） |
| `provider` | object | 当前候选 provider |
| `mpe` | object | 当前 model_provider_endpoint 行 |
| `clientRequest` | ClientRequest | **只读**，客户端原始请求快照 |
| `currentRetryCount` | number | 同 provider 已重试次数 |
| `totalAttemptCount` | number | 总尝试次数 |

ctx 中**不再有** `request`、`upstreamRequest` 字段。

### pendingRequest（hook 入参 / 返回值）

```ts
type PendingRequest = {
  url: string,                              // 完整 upstream URL
  method: string,
  headers: Record<string, string[]>,        // key 全小写
  body?: any,                               // 仅当 content-type 是 application/json 时存在
}
```

返回语义：
- 返回 `pendingRequest`（同一对象，可在原对象上 mutate）→ 作为下一个 tap 的输入；
- 返回 `undefined` / 不 return → 沿用上一 tap 的值；
- 返回新对象 → 替换。

最终值会被 Go 侧反序列化并据此构造 outgoing http.Request。

### clientRequest

```ts
type ClientRequest = {
  path: string,
  method: string,
  headers: Record<string, string[]>,
  model: string,
  body?: any,                               // 同 pendingRequest 的 body 规则
}
```

只读：mutate 不会影响发出请求；存在仅供脚本参考。

### body 字段规则

判定 content-type：取 `headers['content-type'][0]`，去除参数后比对。

- `application/json`（含 `; charset=...`）：body 字段是已 parse 的 JSON 值；脚本可以直接读写。返回时如为 object/array/string，会被 `JSON.stringify` 一次后传给 Go。
- 其它 / 缺失 / body 不是合法 JSON：序列化时省略 body 字段。脚本无法读取或修改 body；返回值里 body 字段 Go 一律忽略，发出去的请求体保留 hook 调用前 Go 侧已构造好的字节（已含 model 替换）。

### 修改 content-type 的影响

如果 hook 通过 `pendingRequest.headers['content-type']` 改了类型，Go 侧仍以**原始 content-type**判定 body 字段是否回收——对返回对象做反向变换时只看入参时的判定结果。换言之：不要靠改 content-type 同时改 body 来"绕过"非 JSON 的 body 隔离规则。

## 示例

```js
// 给 upstream 加一个 header，并往 JSON body 注入字段
picotera.hooks.rewriteRequest.tap('inject', (ctx, pending) => {
  pending.headers['x-tenant'] = [ctx.endpoint.path]
  if (pending.body) {
    pending.body.metadata = { tenant: ctx.endpoint.path }
  }
  return pending
})

// 切换到备用 URL（同 provider 的另一个上游域名）
picotera.hooks.rewriteRequest.tap('failover-url', (ctx, pending) => {
  if (ctx.currentRetryCount > 0) {
    pending.url = pending.url.replace('://api.', '://api-backup.')
  }
})
```
