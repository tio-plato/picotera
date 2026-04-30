# rewriteRequest hook：分清 clientRequest / pendingRequest

改造 rewriteRequest hook 脚本，明确分清 clientRequest 和 pendingRequest——前者是客户端的请求，只用来参考；后者是即将发出去的请求。在发请求出去前，golang 侧对请求的所有字段都序列化为 json，重写后，要能还原，然后严格按照这个改写后请求发出去。

## 决策约束

- **Hook 返回值**：JS 必须返回完整 pendingRequest 对象，不再支持 partial-override（`{url?, method?, headers?, body?}` 那套去掉）。
- **字段清理**：删除 RewriteInput 中残留的 legacy `request` 字段，`ctx` 上只保留 `clientRequest`（只读）和 `pendingRequest`（hook 输入值，可写）。
- **Body 表达**：按 content-type 区分——
  - `application/json`：body 在 JS 侧自动 parse（JS 拿到的是对象），返回时如果是对象 SDK 自动 stringify。
  - 其它 content-type：body 不传给 JS，JS 也无法读取或修改，Go 侧以原始 body 字节为准。
