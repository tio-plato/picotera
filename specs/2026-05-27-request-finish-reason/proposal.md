# Request Finish Reason

为请求增加一个结束原因字段 `finish_reason`，meta 和 upstream 请求都要包括。

## 可选值

- `internal` (1) — 内部错误，包括没下游了，脚本失败了，等等
- `cancelled` (2) — 客户端取消
- `eof` (3) — 服务端正常结束请求
- `headers_timeout` (4) — 读请求头前超时，包括 TLS、连接等
- `read_timeout` (5) — 读响应体时超时

数据库字段类型为 INTEGER，与 `type`、`status` 字段保持一致。默认是 `null`，代表请求还没完成。Go 常量定义字符串名称供 API 层映射。
