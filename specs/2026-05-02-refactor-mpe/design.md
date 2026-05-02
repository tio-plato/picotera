# Design: Inline Provider Models, Drop MPE Table

## 1. 数据模型

### 1.1 新 `provider.provider_models` 形态

`provider.provider_models` 仍是 `JSONB`，从 `["m1", "m2"]` 变成对象：

```jsonc
{
  "claude-3-5-sonnet": {
    "upstreamModelName": "claude-3-5-sonnet-20241022", // 可选
    "endpoints": ["/v1/messages"],                     // 可选；未填/空数组 = 全部已绑定端点
    "priority": 10,                                    // 可选；未填视为 0
    "annotations": { "tier": "premium" }               // 可选；未填视为 {}
  },
  "gpt-4o": {}
}
```

- 顶层对象的 key 是 picotera 逻辑模型名（对应 `model.name`）。
- 各内部字段全可选，便于「只填了 model 名」最简形态。
- 序列化保持 `omitempty` 风格——空字符串/空数组/空对象/0 不写出，前端读到时统一兜底。

### 1.2 删除 `model_provider_endpoint` 表

原 MPE 表中所有逻辑信息（`upstream_model_name` / `priority` / `annotations`）迁入 `provider.provider_models[modelName]` 的对象，加上新增的 `endpoints` 字段后已完全覆盖；表本身删除。

### 1.3 一次性迁移 `006_inline_provider_models.sql`

```sql
-- +goose Up
UPDATE provider SET provider_models = '{}'::jsonb;
DROP TABLE model_provider_endpoint;
CREATE INDEX idx_provider_models_gin
  ON provider USING GIN (provider_models jsonb_path_ops);

-- +goose Down
DROP INDEX idx_provider_models_gin;
CREATE TABLE model_provider_endpoint (...);  -- 复刻 001 的定义
UPDATE provider SET provider_models = '[]'::jsonb;
```

接受「清空 `provider_models`」的丢失，因为目前没有真实生产数据迁移诉求。

### 1.4 路由查询的 GIN 索引

在 `provider.provider_models` 上建 `GIN (provider_models jsonb_path_ops)`，让 `provider_models ? $modelName`（顶层 key 存在性检查）走索引。`jsonb_path_ops` 比默认 GIN opclass 体积更小、`?` / `@>` 命中更快，代价是不支持其他较少使用的 jsonb 操作符——本场景够用。

## 2. 路由查询重构

### 2.1 `GetProvidersByEndpointAndModel` 的新实现

不再 JOIN `model_provider_endpoint`，改为：
- 在 `provider` × `provider_endpoint` 上做 JOIN；
- 用 `provider_models ? $modelName` 走 GIN 索引筛掉不含该模型的 provider；
- 在 SQL 层用 `provider_models -> $modelName` 提取该模型的子对象；
- 在 SQL 层做 endpoints 包含性过滤：`endpoints` 缺失/为空数组 → 通过；否则要求 `endpoints @> [endpointPath]`；
- 返回字段：providerID / providerName / providerCredentials / providerPriority / providerAnnotations / upstreamUrl / endpointPath / 子对象（`upstream_model_name`、`priority`、`annotations`）。

由于排序、`UpstreamModelName` 取值、JSX hook 输入字段都依赖这些值，行结构与现有 `GetProvidersByEndpointAndModelRow` 保持兼容（重命名子对象字段时与旧字段一一对齐）。

### 2.2 服务端 sortProviders 输入

JSX hook 的 `MPE` 字段保持不变：仍包含 `modelName / providerId / endpointPath / upstreamModelName / priority / annotations`。来源从行对象切换到「`provider_models[modelName]` 子对象 + 路由上下文」。

## 3. API 变更

### 3.1 删除 model-provider-endpoints 资源

下列 5 个 operation 全部删除：
- `listModelProviderEndpoints`
- `getModelProviderEndpoint`
- `upsertModelProviderEndpoint`
- `deleteModelProviderEndpoint`

`pkg/contract/model_provider_endpoint.go`、`pkg/server/handle_model_provider_endpoint.go`、`db/queries/model_provider_endpoint.sql`、对应 sqlc 生成代码全部删除。

### 3.2 ProviderView.providerModels 类型变更

`providerModels` 从 `string[]` 变为 `Record<string, ProviderModelEntry>`：

```ts
type ProviderModelEntry = {
  upstreamModelName?: string
  endpoints?: string[]
  priority?: number
  annotations?: Record<string, string>
}
```

createProvider / upsertProvider 的 body 同步变更。

### 3.3 fetchModels 行为变更

`POST /api/picotera/provider-endpoints/fetch-models` 不再写库，只回传上游解析出的模型名列表。响应 shape 不变。前端拿到列表后做 diff，然后手动调用 `upsertProvider` 写入新的 `providerModels`。

详见 `api.md`。

## 4. 前端拆分

### 4.1 新建 `ProviderModelsPanel.vue`

参考 `ProviderEndpointsPanel.vue` 的结构：
- 入口：在 `ProvidersView` 行操作里加一个图标按钮（`brand-yarn` / `cube` 等），打开 panel。
- panel 内部：
  - 顶部：模型行列表，每行展示 `modelName`、可折叠展开编辑 `upstreamModelName / priority / endpoints / annotations`。`endpoints` 用多选下拉（来源于该 provider 已绑定的 `provider_endpoint`），不选即「全部端点」。
  - 中部：「从上游拉取」按钮。点击后：
    1. 调用 `POST /provider-endpoints/fetch-models`（仍要选一个 endpoint 作为来源）。
    2. 拿到上游 `string[]` 后，与本地 `providerModels` key 集合 diff：
       - 上游新增的：直接以 `{}` 形态合并到 `providerModels`，并在 UI 顶部摘要里标记「新增 N」。
       - 上游缺失但本地存在的：弹出二次确认子面板（带勾选框列表），用户确认勾选后才删除；默认不勾选。
    3. 用户在 panel 内最终点「保存」时，整体调一次 `PUT /providers` 持久化。
  - 底部：保存 / 取消按钮（与 ProviderForm 结构对齐）。
- 拉取按钮和现有 `ProviderEndpointsPanel` 里的「拉取模型」按钮重叠，删掉后者；模型列表不再在端点 panel 里展示。

### 4.2 修改 `ProviderForm.vue`

- 移除 `ModelListEditor` 的引入与字段。
- 创建 / 编辑 provider 时，`providerModels` 默认 `{}`；模型列表改由独立的 ProviderModelsPanel 管理。
- 新建 provider 时，UI 上提示「保存后在『模型』面板配置模型列表」。

### 4.3 删除 `ModelListEditor.vue`

不再被任何地方引用（确认无其它使用方）。如有保留价值，留作未来引用——目前删除。

### 4.4 `MappingsView` / `MappingForm` / 路由

整套「映射」UI 删除：
- `dashboard/src/views/MappingsView.vue`
- `dashboard/src/components/MappingForm.vue`
- `src/router/index.ts` 里的 `mappings` 路由
- `src/App.vue` `pageMeta` 里的 `mappings` 条目
- `AppSidebar.vue` 里的「模型映射」入口

模型在哪个 provider/endpoint 上可用，完全由 `provider.providerModels` 表达。「列出所有 (model, provider, endpoint) 三元组」这件事改在 `ModelsView` 里以聚合视图解决（详见 4.5），或在本次先不做、由前端用户自行去 ProvidersView 查看每个 provider 的模型 panel。本方案先不做聚合视图——`ModelsView` 暂时只展示模型清单本身，与现有行为一致。

### 4.5 `ModelsView` 行为

保持现状，不做反向聚合视图（避免范围蔓延）。

## 5. JSX hook 兼容

`MPE` shape 保持向后兼容：JS 看到的字段名和类型不变。`endpoints` 不暴露给 hook（路由层已经过滤掉不匹配 endpoint 的候选项）。

## 6. 不引入新依赖

- 后端：仅利用 PostgreSQL 现有 JSONB 操作符（`->`、`->>`、`@>`）。
- 前端：复用 `SidePanel`、`AnnotationsEditor`、`useApi`、`useSidePanel` 等现有原语，不引入新库。

## 7. 范围外

- 不迁移现有数据。`provider_models` 一律清空，由用户重新配置。
- `request` 表结构与历史记录不变。
- 不实现「编辑 provider 模型时实时校验上游名称是否存在」类的高级特性。
