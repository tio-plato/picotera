# Models Endpoint on Provider

## 原始需求

重构一下对 modelList 端点的处理。我希望在 provider 里面，增加一个 models endpoint url 的字段，并增加一个 models endpoint resolver 的字段。功能上，和现在的 models 绑定相同，拉取模型时也固定拉取这个端点。原有的 generalListModels 功能也全部移除。

## 澄清

- 新字段 `models_endpoint_url`（TEXT，可空）与 `models_endpoint_resolver`（INTEGER，复用 `CredentialsResolver` 枚举：`generalApiKey / bearerToken / xApiKey / searchKey / googApiKey / unknown`）落在 `provider` 表上。
- 迁移策略：对每个 provider，取其 `generalListModels` 类型的 `provider_endpoint` 绑定（按 `endpoint_path` 字典序首条）的 `upstream_url` 与 `credentials_resolver` 填入新字段；随后删除所有 `generalListModels` 类型的 `provider_endpoint` 行和 `endpoint` 行。
- `EndpointType_GeneralListModels` 常量、枚举值、`generalListModels` 字符串全部删除，整数 `6` 自此不再使用。
- 拉取模型时端点不再可选——后端固定使用 provider 上的 `models_endpoint_url` + `models_endpoint_resolver`，前端去掉端点下拉选择器。
- `rewriteProviderModels` JS 钩子输入移除 `endpointPath` 字段。脚本如果引用了该字段会取到 `undefined`，需要使用者改写——不留兼容层。
- Provider 未配置 `models_endpoint_url` 时，前端禁用拉取按钮，文案提示去渠道编辑表单填写；后端在 URL 为空时拒绝 400。
