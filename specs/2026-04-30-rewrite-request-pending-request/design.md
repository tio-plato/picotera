# Design

## 现状

`pkg/server/handle_gateway.go` 在 retry loop 里：

1. `buildUpstreamRequest` 构造 `*http.Request` + `reqBody []byte`（已替换 model、补 auth header）。
2. 把 req 的字段拍成 `jsx.UpstreamRequestShape{URL, Method, Headers, Body string}` 传进 `RunRewriteHook`。
3. Hook 返回 `RewriteOutput`（partial override：`URL *string`、`Method *string`、`Headers *map`、`Body json.RawMessage`）。
4. `applyRewrite` 就地 mutate `*http.Request`：URL 字段重新解析、Headers 整体替换、Body 重写并修正 `ContentLength`。

问题：
- `RewriteInput` 同时暴露 `request`、`clientRequest`、`upstreamRequest` 三个字段，语义重复。
- Hook 返回 partial override，Go 侧必须把每种 nil 检查写一遍；mutate-in-place 让"发出去的到底是什么"难追溯——req 还携带原始 buildUpstream 时设的字段。
- Body 始终以字符串往返，二进制/非 JSON 场景没有清晰契约。

## 目标

- ctx 上只暴露两个请求对象：
  - `clientRequest`：客户端原始请求快照，只读参考。
  - `pendingRequest`：即将发往 upstream 的请求；同时是 rewriteRequest waterfall 的输入值（initial value）。
- Hook 返回完整 `pendingRequest`（同 SDK 既有 waterfall 语义：return undefined 即沿用 mutate 后的；return 新对象则替换）。
- Go 侧：调用 hook 前把 upstream 请求**全字段**序列化进 JSON；hook 返回后**完全丢弃**先前构造的 `*http.Request`，从返回的 JSON 形状**重新构造**新 request 发出，保证"严格按改写后发出"。
- Body 按 content-type 区分（见下）。

## 数据形状（Go ↔ JS）

```
PendingRequest = {
  url: string,
  method: string,
  headers: { [lower-name: string]: string[] },
  body?: <see body rule>,
}

ClientRequest = {
  path: string,
  method: string,
  headers: { [lower-name: string]: string[] },
  model: string,
  body?: <see body rule>,
}
```

`ClientRequest` 不带 `url` 字段（客户端打到 picotera 的路径用 path 表示），保持现状。`PendingRequest` 是要发给 upstream 的，需要完整 url。

### Body 规则（双方对称）

判定依据：headers 中 `content-type` 第一个值（lower-cased），是否以 `application/json` 开头（允许 `; charset=...`）。

- **JSON**：Go 把原始 body 字节直接当作 JSON value 嵌入到序列化结果里（`json.RawMessage`，前提是其本身是合法 JSON；非法时退回到下一条规则）。JS 侧读到的是已 parse 过的对象/数组/标量。回程时 SDK 把 `body` 字段如果是 object/array/string 统一 `JSON.stringify` 一次，让 Go 拿到一段 JSON 文本——这段文本就是发出去的 body 字节。
- **非 JSON / 非法 JSON / 没有 content-type**：序列化时**省略** body 字段（不是 null，是缺省）。JS 看不到、改不到。Go 侧无论 hook 返回什么 `body` 字段都忽略，沿用调用 hook 之前的原始 body 字节。

注：clientRequest 也按这条规则；clientRequest 永远是只读快照，body 改了也不会被 Go 读回去——但仍按相同规则隐藏非 JSON body，保持 ctx 上两个 request 对象语义一致。

## Hook 返回值合约

```js
picotera.hooks.rewriteRequest.tap('name', function (ctx, pendingRequest) {
  // ctx.clientRequest, ctx.endpoint, ctx.provider, ctx.mpe, ...
  pendingRequest.headers['x-foo'] = ['bar']
  pendingRequest.body.extra = 1            // 仅当 content-type 是 application/json
  return pendingRequest                    // 或 mutate 后不 return
})
```

Waterfall 既有语义不变：
- return undefined → 沿用上一个 tap 处理过的值；
- return 新对象 → 作为下一个 tap 的输入；
- 最终值即 Go 拿来重建 request 的 pendingRequest。

不再支持 partial-override `{url?, method?}`，全部字段都必须出现在返回对象里。SDK 给 hook 的 input 已经是完整对象，自然满足。

## Go-side 流程

```
buildUpstreamRequest(ctx, original, body, ...) → req, reqBody
                                              ↓
serializePending(req, reqBody)               // → PendingRequestShape (含 body 视 content-type)
                                              ↓
RunRewriteHook(input=PendingRequestShape, ctx={clientRequest, ...})
                                              ↓
returned PendingRequestShape
                                              ↓
buildRequestFromPending(ctx, returned, fallbackBody=reqBody)
                                              ↓
新的 req' + reqBody'  // 丢弃旧的 req；artifact upload / forwardRequest 都用新的
```

`fallbackBody=reqBody`：当 content-type 非 JSON 时，hook 看不到 body、也无法改 body，Go 重建 request 时仍用 buildUpstreamRequest 时算好的字节。

## 为什么"丢弃旧 req 重建"

mutate-in-place 容易留下脏字段：例如 `req.URL.Host` 与 `req.Host` 不同步、`req.ContentLength` 与新 body 长度不一致、`req.Body` 与 `reqBody` 状态不一致。彻底用 `http.NewRequestWithContext(ctx, method, url, bytes.NewReader(body))` 从新形状起，每个字段一处来源，不会有遗漏。

## 影响范围

- **Breaking**：现有 JS 脚本中调用 `picotera.hooks.rewriteRequest` 且使用 `return { url: ... }` 这类 partial 写法的会失效。生产 / 测试 fixtures 需要更新。`pkg/jsx/engine_test.go` 里两个 rewrite 用例需要重写。
- 不影响 sortProviders、beforeRequest hook 的签名。

## 不涉及

- 二进制 body 支持（非 JSON 直接绕过 hook 即可）。
- request artifact 落库格式不变（依然是原始字节）。
- Go 测试新增——目前包内没有 Go test 框架（CLAUDE.md：No Go tests exist yet），engine_test.go 是少数例外，沿用即可。
