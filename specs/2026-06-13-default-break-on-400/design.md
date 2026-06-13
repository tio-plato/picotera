# 设计

## 目标

上游返回 HTTP `400` 时，网关默认把这次响应透传回客户端并停止重试其余 provider；该默认行为通过 `afterUpstreamError` hook 的 `break` 种子值实现，因此脚本可完全改写它。

## 现状

`afterUpstreamError` 是一个 waterfall hook，在每次上游尝试失败后运行。决策 `break` 的初值由传入 hook 的 `UpstreamErrorView.Break` 种子决定（`pkg/jsx/session.go` 的 `RunAfterUpstreamError`），目前所有网关调用方（`pkg/server/gateway_flow_attempts.go` 的 `runAfterUpstreamError`）都把种子固定为 `false`。

当 `break=true && streamed=false` 时，`respondUpstreamErrorBreak` 把响应写回客户端：

- `dec.StatusCode<=0` → 沿用上游原始状态码（再回退 502）；
- `dec.Message==""` → 透传上游原始 body 与 `Content-Type`；否则用 `dec.Message` 覆盖 body 并强制 `application/json`。

## 方案

### 1. 网关层种入默认 break

在 `runAfterUpstreamError` 中，根据本次失败的状态码计算 `break` 种子：状态码恰好为 `400` 且 `streamed=false` 时种入 `true`，否则 `false`。状态码取自 `state.LastJSErr.StatusCode`（非 200 上游响应由 `updateAttemptState` 写入真实状态码）。

```go
defaultBreak := statusCode == http.StatusBadRequest && !streamed
dec, err := f.session.RunAfterUpstreamError(jsx.UpstreamErrorView{
    Break: defaultBreak, StatusCode: statusCode, Message: message, Streamed: streamed,
})
```

hook 因此能在输入 `input.break` 中看到默认已是 `true`，并通过返回 `{ break: false }` 改写为继续尝试。

### 2. 修正 passthrough 的回显语义，保证忠实透传

`RunAfterUpstreamError` 的 passthrough / 无 hook 分支当前把 `initial.StatusCode` 与 `initial.Message` 回显进决策（`zero`）。在 `break=false` 时这些字段从不被网关使用（决策被丢弃），仅作单测留痕；一旦默认种子变为 `break=true`，被回显的 `Message`（即上游错误 body 文本）会让 `respondUpstreamErrorBreak` 误判为「覆盖」，从而丢失上游原始 `Content-Type`，无法忠实透传。

将 `zero` 改为只携带 `Break`、不携带 `StatusCode` / `Message`：

```go
zero := AfterUpstreamErrorDecision{Break: initial.Break}
```

这样语义统一为「passthrough = 沿用初始 break，不做任何覆盖」，与脚本显式返回 `{ break: true }`（不带 statusCode/message）的「follow upstream」语义一致。结果：400 + 无 hook（或 hook passthrough）→ `dec = {break:true, statusCode:0, message:""}` → 透传上游原始状态码、body、`Content-Type`，即忠实透传。

`UpstreamErrorView.StatusCode` / `Message` 仍然原样传入 hook 供脚本读取，仅不再回显进 passthrough 决策。
