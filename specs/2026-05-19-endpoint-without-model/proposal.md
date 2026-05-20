# Endpoint Without Model

## 原始需求

增加一种没有模型的端点。如果端点不包括模型字段路径，请求的时候，不再报错，而是将所有绑定了该端点的 provider 都纳入考虑。此时没有 mpe，因此涉及 mpe 的地方，传入一个兼容对象，modelName 为空。

## 澄清

- **触发条件**：`endpoint.model_path = ''`（空字符串）。不新增 `endpoint_type` 枚举值，不新增列。
- **候选 provider 集**：`provider_endpoint` 表中绑定该 `endpoint_path` 且 `provider.disabled = false` 的所有 provider。完全脱离 `model` / `model_provider_endpoint` / `provider_models` 这三张表/字段，不做按模型的过滤。
- **`request` 行**：meta 与 upstream 两条 request 行的 `model`、`upstream_model` 列都留空（NULL）。upstream 请求体也不替换 model 字段。
- **JS 钩子**：全部按原顺序运行；`Candidate.MPE` 是兼容对象（`modelName: ""`、`upstreamModelName: ""`、`priority: 0`、`annotations: {}`，仍保留 `providerId` 与 `endpointPath`），`candidate.annotations` 仅按 provider + apiKey 两层合并（没有 model 层与 entry 层）。
- **`rewriteModel` 钩子**：仍然以 `model: ""` 调用；如果返回非空字符串，直接以错误结束本次请求（不接受脚本在无模型端点上指定模型）。
- 没有兼容层：`extractModel` 在 `modelPath` 为空时的现有 400 报错路径直接被替换，新增分支不试图兼容历史 caller。
