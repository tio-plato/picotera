# Endpoint type field

## 原始需求

增加端点类型字段，和其它枚举一样，在 golang contract 侧定义枚举值，数据库中用整型。枚举类型有：`general`、`openaiChatCompletions`、`openaiResponses`、`anthropicMessages`、`anthropicCountTokens`、`generalListModels` 这几类。顺手更新 UI。

同时模型字段路径改为仅有 `openaiXX` 和 `anthropicXX` 类型必填，其它类型选填。迁移的时候默认是 `general`。

之前前端渠道模型配置界面，有个选 endpoint 然后拉取的功能嘛，这个地方限制一下只能选 `generalListModels` 端点和 `general` 端点（用 optgroup 分组一下）。

## 澄清

- "端点类型" 是 `endpoint` 表上新增的整型列，配合 contract 侧的 `EndpointType_*` 常量与字符串视图互转，复用 `CredentialsResolver` 的模式。
- 枚举值（按字符串）：`unknown`、`general`、`openaiChatCompletions`、`openaiResponses`、`anthropicMessages`、`anthropicCountTokens`、`generalListModels`。
- `unknown` 始终保留为整型 `0`，其他依顺序编号；存量行通过迁移的默认值落到 `general`。
- "模型字段路径必填" 仅在前端表单层校验：`openaiChatCompletions` / `openaiResponses` / `anthropicMessages` / `anthropicCountTokens` 必填，其它类型选填，可留空字符串。
- `model_path` 列保持 `NOT NULL`，未填走空字符串落库；网关 `extractModel` 在 `model_path == ""` 时直接报错（沿用 `model_not_found` 语义即可）。
- "fetch-models 来源端点选择器" 的过滤只在前端做：仅展示 `generalListModels` 和 `general` 类型的已绑定端点，使用 `<optgroup>` 分组。
- 迁移：新列 `endpoint_type INTEGER NOT NULL DEFAULT 1`（即 `general`），不动 `model_path`。
