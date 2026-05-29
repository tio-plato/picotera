# SSE 流内错误识别

为所有网关路径设计错误识别处理功能：即使响应是 200，也可能在 SSE 中报错。

比方说：

```
data: {"type":"error","error":{"type":"server_error","code":null,"message":"upstream connect error or disconnect/reset before headers. reset reason: connection termination","param":null},"sequence_number":3}
```

这种情况，识别 SSE 数据块里面的 `error.message` 字段，记录为错误原因，并将对应的元请求、上游请求状态记录为错误。这种错误无法重试（响应体已开始回写客户端），只需记录。

## 已确认的设计决策

1. **status_code 保留真实值**：上游 HTTP 返回 200 时，被标记为错误的 meta / 上游请求行的 `status_code` 仍记录为真实的 200。仅把生命周期 `status` 置为 `failed`，并写入 `error_message` 与新的 `finish_reason`，以便区分「HTTP 层失败」与「流内错误」。
2. **仅检测 SSE**：只扫描 SSE data 块。非流式 JSON 响应体不在本次处理范围内。
3. **仅匹配 `error.message`**：只识别 SSE data 负载 JSON 中的 `error.message` 路径（非空字符串）。该路径覆盖 Anthropic Messages、OpenAI Chat Completions、Gemini 的原生错误形态。不额外处理 OpenAI Responses 的 `type:"error"` 顶层 message 等变体。
