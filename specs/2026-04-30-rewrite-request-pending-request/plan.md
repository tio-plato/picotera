# Plan

## 1. `pkg/jsx/types.go`

- 删除 `UpstreamRequestShape`。
- 新增 `PendingRequestShape`：
  ```go
  type PendingRequestShape struct {
      URL     string              `json:"url"`
      Method  string              `json:"method"`
      Headers map[string][]string `json:"headers"`
      // Body 是已序列化好的 JSON value（仅 content-type=application/json 时有效）。
      // 缺省（nil）= 不暴露给 JS / 不接受 JS 改写。
      Body    json.RawMessage     `json:"body,omitempty"`
  }
  ```
- 改 `RequestShape`（用于 clientRequest）：移除 `Body string` 字段，新增 `Body json.RawMessage \`json:"body,omitempty"\``。其余字段不动。
- 改 `RewriteInput`：删 `Request`、删 `UpstreamRequest`，保留 `ClientRequest RequestShape`，新增 `PendingRequest PendingRequestShape`。
- **删除** `RewriteOutput`（不再使用 partial-override）。

## 2. `pkg/jsx/session.go` `RunRewriteHook`

- 函数签名改为 `RunRewriteHook(in RewriteInput) (PendingRequestShape, error)`。
- waterfall 调用：
  ```js
  const r = await picotera.hooks.rewriteRequest.runWaterfall(ctx, ctx.pendingRequest);
  ```
- waterfall 默认值是 `ctx.pendingRequest`（这样 passthrough 时 r 就是入参 pending）。
- 返回时不再做 partial-decode；统一 `json.Unmarshal` 进 `PendingRequestShape`。
- 删除 SDK 端 "if r.body is object then stringify" 的特殊逻辑——挪到 `sdk.js` 的 waterfall wrapper 或 `RunRewriteHook` 的 JS 表达式里：返回前对 `r.body`（若是 object/array）`JSON.stringify` 一次。

## 3. `pkg/jsx/sdk.js`

不动 `Waterfall`；保持纯净。Body 序列化逻辑放在 `RunRewriteHook` 的 JS 表达式里（与现状一致），改成处理新的 PendingRequest 结构：

```js
(async () => {
  const ctx = %s;
  const initial = ctx.pendingRequest;
  const r = await picotera.hooks.rewriteRequest.runWaterfall(ctx, initial);
  const out = (r === undefined || r === null) ? initial : r;
  // 如果 body 还是非字符串，stringify 一次让 Go 拿到 JSON 文本
  if (out && typeof out.body !== 'undefined' && typeof out.body !== 'string') {
    out.body = JSON.stringify(out.body);
  }
  return JSON.stringify(out);
})()
```

注意：`out.body` 如果是 string，意味着脚本主动写成了字符串——视作 JSON 文本，直接发给 Go。

## 4. `pkg/server/gateway_helpers.go`

- 删除 `applyRewrite`。
- 新增 `serializePendingRequest(req *http.Request, body []byte) jsx.PendingRequestShape`：
  - URL = `req.URL.String()`、Method = `req.Method`、Headers = `mapLowerKeys(req.Header.Clone())`。
  - body 处理：判定 content-type（`req.Header.Get("Content-Type")`，截到 `;` 前 trim 后小写）；若是 `application/json` 且 `json.Valid(body)`，把 `Body` 设为 `json.RawMessage(body)`；否则 `Body=nil`。
- 新增 `serializeClientRequest(r *http.Request, body []byte, model string) jsx.RequestShape`：和现有 `mapLowerKeys` 同结构，body 字段按上面相同规则填充。
- 新增 `buildRequestFromPending(ctx context.Context, p jsx.PendingRequestShape, fallbackBody []byte) (*http.Request, []byte, error)`：
  - 决定 outBody：若 `p.Body == nil` → `fallbackBody`。否则 `p.Body` 是 SDK 表达式 stringify 后的 JSON string token，`json.Unmarshal(p.Body, &s)` 得到真实 body 文本；解码失败直接冒泡当作 hook 错误返回。
  - `http.NewRequestWithContext(ctx, p.Method, p.URL, bytes.NewReader(outBody))`，复制 headers，`req.ContentLength = len(outBody)`。
  - 错误（URL 解析失败、method 非法）冒泡返回。

## 5. `pkg/server/handle_gateway.go`

retry loop 内：

- 先调用 `serializePendingRequest(req, reqBody)` → `pending`。
- 调用 `RunRewriteHook(jsx.RewriteInput{ ..., ClientRequest: jsClientRequest, PendingRequest: pending })`，拿到 `newPending`。
- `req2, reqBody2, err := buildRequestFromPending(ctx, newPending, reqBody)`：失败时（hook 把 URL 改坏了）走 `failHook(err)`，cancel context。
- 用 `req2` / `reqBody2` 替换原变量（包括 `uploadRequestArtifact` 用 `req2.Method/URL/Header` 与 `reqBody2`、`forwardRequest(req2)`）。
- `jsClientRequest` 改用新的 `serializeClientRequest(r, body, modelName)` 构造（替换原地构造 `jsx.RequestShape{...}`），以让 client body 也按 content-type 规则暴露。

## 6. `pkg/jsx/engine_test.go`

更新两个 rewrite 用例，新增几个新规则用例：

- `TestSession_Hooks_Rewrite_Passthrough`：脚本不 return，期望返回值等于入参 pending。
- `TestSession_Hooks_Rewrite_FullReplace`：脚本 `return Object.assign({}, pending, { url: 'https://y' })`，期望 URL 变更，其它字段保留。
- `TestSession_Hooks_Rewrite_BodyJSON_Roundtrip`：input `Body=json.RawMessage(\`{"a":1}\`)`、`headers['content-type']=['application/json']`；脚本 `pending.body.b = 2`，期望返回 body = `{"a":1,"b":2}`（顺序按 JS 引擎；用 `json.Unmarshal` 比较语义）。
- `TestSession_Hooks_Rewrite_BodyHidden_NonJSON`：input `Body=nil`、content-type=`text/plain`；脚本 `pending.body = 'evil'; return pending`。期望返回的 `Body` 仍是 nil（SDK 表达式可保留脚本写入的 body string，但 Go 侧规则在 `buildRequestFromPending` 用 fallback 字节——这部分由 server 侧测试覆盖；此处只验证 jsx 层契约，断言 hook 看不到原 body）。

## 7. 验证

- `go build ./...`
- `go test ./pkg/jsx/...`
- 手动 smoke：`docker compose up -d` → `mise run server` → 配一个 enabled JS 脚本，在 `/api/picotera/scripts` 注入 `pending.headers['x-test'] = ['1']`，跑一次 chat completion 对 echo upstream，确认 upstream 收到了改写后的 header；再跑一次脚本写 `pending.body.foo = 'bar'`，确认请求体新增字段且 Content-Length 一致。
- `mise run openapi` 不需要——本次改动不触及 HTTP API contract。

## 8. 文档/Memo

无 README / docs 更新。Hook 相关说明在 specs 目录里随这一份 api.md。
