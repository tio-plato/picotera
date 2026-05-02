# Refactor MPE: Inline provider models into provider.provider_models

## 原始需求

我要重构整个 model-endpoint-upstream 映射(MPE)。

将上游供应商的模型字段，原本是 `string[]`，重构为：

```ts
Record<string, {
  upstreamModelName?: string
  endpoints?: string[]
  priority?: number
  annotations?: Record<string, string>
}>
```

- key 是 picotera 内部逻辑模型名（`model.name`）。
- `upstreamModelName`、`priority`、`annotations` 与原 MPE 表中含义一致。
- `endpoints` 表示该模型在该 provider 上支持哪些端点；未填或空数组表示支持该 provider 已绑定的全部端点。

## 数据库

数据库结构变动做一次性迁移：
- 新增列 / 重写 `provider.provider_models` 的 JSON 形态（清空原值，无需保留旧数据）。
- 删除 `model_provider_endpoint` 表。

## fetchModels 行为

原本的「自动获取并写库」改为：
- 服务端只返回上游 models 列表，不再写库。
- 由前端决策合并：
  - 上游有但本地没有的 → 直接合并到现有对象。
  - 上游没有但本地有的 → 列表+勾选让用户确认是否删除。

## 前端 UI

模型列表的编辑入口从内嵌在 ProviderForm 里，改为像 `ProviderEndpointsPanel` 那样的独立侧边面板（弹窗）。
