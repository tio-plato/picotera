# 请求处理中状态 + 及时回填字段 + UI 修正

为请求日志增加一个状态：请求处理中。并且及时更新请求的端点和请求的模型。顺便把 UI 也调整下，未完成的请求不要显示为错误。

## 决策约束

- **不新增 DB status 值**：复用现有 `Pending(0)` / `HeaderReceived(1)`。"处理中" 是 UI 概念，不是 DB schema 变更。
- **及时回填**：meta 请求在已知信息时立刻 `UPDATE`，不再等到 `streamSuccess`。
  - 路径匹配后：写 `endpoint_path`。
  - 模型提取后：写 `model`。
- **UI**：未完成的请求（status ≠ Completed/Failed 且 statusCode 缺失）显示中性 "处理中" 徽章，不再显示红色 ERR。
