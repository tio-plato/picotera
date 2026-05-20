# Endpoint Type: exaSearch

## 原始需求

给 endpoint 新增一种类型，叫做 `exaSearch`，用于提供 Exa 兼容格式的搜索。不需要填写模型解析。

## 澄清

- **定位**：纯标签。`exaSearch` 仅作为 `endpoint_type` 枚举新值，用于 UI 标识与未来分析；不引入任何新的网关运行时分支。
- **运行时行为**：完全沿用 no-model 端点路径（`endpoint.model_path = ""`），由现有 2026-05-19 实现承接 —— 跳过 `extractModel`，把所有绑定 provider 当作候选，请求/响应不做模型字段改写。`exaSearch` 端点的 `model_path` 强制为空字符串。
- **前端表单**：`EndpointForm` 选中 `exaSearch` 时，"模型字段路径" 输入框被禁用且强制为空字符串提交；类型选择器列出 `exaSearch`。凭证解析默认仍 `generalApiKey`，可手动改成 `xApiKey`（Exa 习惯用 `x-api-key`），不做默认值切换。
- **fetch-models 选择器**：`exaSearch` 不进入 `ProviderModelsPanel` 的 fetch-models 来源候选（保持仅 `general` / `generalListModels` 的策略，由于本仓库当前未保留 `generalListModels`，实际策略由前端代码现状决定，本特性不动）。
- **响应处理 / preview**：`extractUserMessagePreview` 和 `responseAggregationFormat` 的 switch 不新增 `exaSearch` 分支 —— 落入 default 后 preview 走"尝试所有 LLM 抽取器"的兜底（搜索请求体不会命中任何一个，返回空），aggregation 直接 `FormatUnknown`，与目标一致。
- **迁移**：新增 `endpoint_type = 9` 编号（顺接 Gemini Stream = 8），不动其它枚举值；不写 SQL 迁移（列已存在）。
- **没有兼容层**：不为旧前端预留 `exaSearch` 解析；后端 `ToEndpointType` / `FromEndpointType` 直接扩展。
