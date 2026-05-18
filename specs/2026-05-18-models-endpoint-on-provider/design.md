# Design

## 背景

当前 fetch-models 流程依赖一对纵向耦合：

1. 一个 `endpoint_type = generalListModels` 的 `endpoint` 行（仅承载 `credentials_resolver`，不承载上游 URL）。
2. 一条 `provider_endpoint` 绑定，提供 `upstream_url`（+ 可选的 resolver 覆盖）。

UI 让用户在「拉取来源」下拉里挑这条绑定，后端按所选 `endpointPath` 解析。这套结构导致：

- "列模型用的端点" 与"用于路由的端点"共用同一个 `endpoint` 表，但语义完全不同——`generalListModels` 不参与请求路由、不参与 endpointRouter 匹配、不进入网关 catch-all，只是 fetch-models 流程的占位。
- 一个 provider 通常只需要一个列模型 URL，但当前结构允许多条；前端为此维护"分组下拉"+"默认选第一条"。
- 列模型 URL 本质上是 provider 级配置（同一 provider 全局只用一个），却被存储在 provider_endpoint 行上。

## 目标

把列模型 URL 提升为 provider 的直属字段，删除 `generalListModels` 概念以及围绕它的下拉选择、optgroup、ENDPOINT_TYPES_DIRECT 等代码。

## Schema

新增迁移 `023_provider_models_endpoint.sql`：

```sql
-- +goose Up
ALTER TABLE provider
  ADD COLUMN models_endpoint_url TEXT NOT NULL DEFAULT '',
  ADD COLUMN models_endpoint_resolver INTEGER NOT NULL DEFAULT 0; -- unknown

-- 回填：对每个 provider，挑其首条 generalListModels 类型绑定的 upstream_url / resolver
WITH first_binding AS (
  SELECT DISTINCT ON (pe.provider_id)
    pe.provider_id,
    pe.upstream_url,
    COALESCE(NULLIF(pe.credentials_resolver, 0), e.credentials_resolver) AS resolver
  FROM provider_endpoint pe
  JOIN endpoint e ON e.path = pe.endpoint_path
  WHERE e.endpoint_type = 6 -- generalListModels
  ORDER BY pe.provider_id, pe.endpoint_path
)
UPDATE provider p
SET models_endpoint_url = fb.upstream_url,
    models_endpoint_resolver = fb.resolver
FROM first_binding fb
WHERE p.id = fb.provider_id;

-- 清理 generalListModels 绑定与端点
DELETE FROM provider_endpoint
WHERE endpoint_path IN (SELECT path FROM endpoint WHERE endpoint_type = 6);
DELETE FROM endpoint WHERE endpoint_type = 6;

-- +goose Down
ALTER TABLE provider
  DROP COLUMN models_endpoint_url,
  DROP COLUMN models_endpoint_resolver;
```

`DEFAULT '' / 0` 仅服务于迁移期间已存在行（无对应 generalListModels 绑定的 provider），后续 INSERT 由 sqlc 显式提供。

## 后端

### Contract

`pkg/contract/endpoint.go`：

- 删除 `EndpointType_GeneralListModels` 常量。
- `ToEndpointType` / `FromEndpointType` 移除 `generalListModels` 分支。
- `EndpointView.EndpointType` 的 `enum:"..."` 标签去掉 `generalListModels`。

`pkg/contract/provider.go`：

- `ProviderView`、`CreateProviderRequest.Body`、`UpsertProviderRequest.Body` 增加：
  ```go
  ModelsEndpointUrl      string `json:"modelsEndpointUrl,omitempty"`
  ModelsEndpointResolver string `json:"modelsEndpointResolver,omitempty" enum:"unknown,generalApiKey,bearerToken,xApiKey,searchKey,googApiKey"`
  ```
- `ToProviderView` / `FromProviderView` 处理新字段——`ModelsEndpointResolver` 走 `FromCredentialsResolver` / `ToCredentialsResolver` 字符串互转。

`pkg/contract/provider_endpoint.go`：

- 把 `FetchModelsRequest.Body` 改成 `{ ProviderID int32 }`，去掉 `EndpointPath`。
- `OperationFetchModels` 的 `Path` 改为 `/providers/fetch-models`。

### Queries / sqlc

`db/queries/provider.sql`：

- `CreateProvider`：参数追加 `models_endpoint_url`、`models_endpoint_resolver`。
- `UpdateProvider`：追加 `set_models_endpoint_url` / `models_endpoint_url` / `set_models_endpoint_resolver` / `models_endpoint_resolver`。
- 新增 `GetProviderModelsEndpoint` 查询（fetch-models handler 用），返回 `id, name, credentials, proxy_url, models_endpoint_url, models_endpoint_resolver, annotations, provider_models, priority, disabled`——实际可直接复用 `GetProviderByID`，不必新增。

跑 `sqlc generate` 生成新代码。

### Handler

`pkg/server/handle_providers.go`：

- `handleCreateProvider` / `handleUpsertProvider` 把新字段传进 `CreateProviderParams` / `UpdateProviderParams`。
- 校验：`ModelsEndpointResolver` 字符串走 `ToCredentialsResolver` 转换；URL 不做格式校验（与 `provider_endpoint.upstream_url` 一致，允许空字符串）。

`pkg/server/handle_provider_endpoint.go::handleFetchModels` 重写：

- 只读 `provider`；不再查 `provider_endpoint` / `endpoint`。
- 校验 `provider.ModelsEndpointUrl != ""`，否则 `huma.Error400BadRequest("provider has no models endpoint configured")`。
- 用 `provider.ModelsEndpointUrl` 构造请求。
- `applyCredentials(req, provider.Credentials, provider.ModelsEndpointResolver, nil)`——直接拿 provider 自带的 resolver，去掉 `effectiveSendResolver` 这次调用。
- 其余流程（forward / decode / parseModels / aggregate / runHook）保持不变；唯一差异在传给 JS 钩子的 `RewriteProviderModelsInput` 不再含 `EndpointPath`。

### JS 钩子

`pkg/jsx/types.go`：

- `RewriteProviderModelsInput` 删除 `EndpointPath` 字段。

`pkg/jsx/sdk.js`、`pkg/jsx/engine_test.go`、`docs/example-scripts/*` 同步——示例脚本里用到 `endpointPath` 的（如 `zenmux-models.js`）改为不依赖该字段或换其它信号。

## 前端

### 类型与常量 (`dashboard/src/api/index.ts`)

- `EndpointType` 自动从 openapi 生成，无需手改；但 `ENDPOINT_TYPES_DIRECT` 删除（不再有 `generalListModels`）。
- `ENDPOINT_TYPE_LABELS` 去掉 `generalListModels` 键。

### `ProviderForm.vue`

新增两个字段（放在「代理 URL」下方）：

- `Field label="模型列表 URL"`：纯 `<Input>`，placeholder `https://api.openai.com/v1/models`，可空。
- `Field label="模型列表凭证解析"`：`<Select>`，选项 `generalApiKey / bearerToken / xApiKey / searchKey / googApiKey`，默认 `generalApiKey`。

提交时把两个字段一起塞进 `upsertProvider` body。`unknown` 选项不展示——空 URL 时 resolver 取值无所谓，但要稳定地写一个值。

### `ProviderModelsPanel.vue`

- 去掉 `fetchEndpointPath` 状态、`groupedFetchSources` 计算属性、`Select` 下拉。
- 拉取按钮：
  - `disabled = fetching || !provider.modelsEndpointUrl`
  - 当 URL 为空时，按钮旁文案显示「请先在渠道编辑表单配置模型列表 URL」。
- `fetchProviderModels` 调用改为只传 `{ providerId }`。
- `applyLoadedData` 不再初始化 `fetchEndpointPath`。

### `EndpointForm.vue`

`endpointTypeOptions` 自动从 `ENDPOINT_TYPE_LABELS` 派生，删除 `generalListModels` 后下拉自动收缩。

### `ModelsView.vue`

`routablePathSet` 里的 `.filter((e) => e.endpointType !== 'generalListModels')` 删掉——所有 endpoint 现在都是 routable 类型（或 `general` / `unknown`）。

### 其它扫尾

- `dashboard/src/components/ProviderModelsPanel.vue` 模板里 `groupedFetchSources` / optgroup 移除。
- `openapi.yaml` 重新生成。
- `dashboard/src/openapi-types.d.ts` + `dashboard/src/api/openapi.ts` 重新生成（`mise run openapi` + `pnpm generate-openapi`）。

## 测试

- `pkg/server/handle_unified_gateway_test.go`、`pkg/server/user_message_preview_test.go` 中引用 `contract.EndpointType_GeneralListModels` 的位置替换为 `contract.EndpointType_Unknown`（语义等同——"非生成类型"）。
- 现有 JS 钩子测试若校验 `endpointPath` 传入，更新断言。

## 风险与权衡

- 删除 `EndpointType_GeneralListModels` 整数常量后，整数 `6` 保留为空位。若未来要新增 endpoint type，可以重新占用 `6`，但本次不主动复用以避免心智负担。
- 迁移按 `endpoint_path` 字典序首条 binding 回填，可能与运维直觉不一致；若结果不对，运维直接在新 UI 编辑 provider 即可修正。
- JS 钩子破坏性变更：使用 `endpointPath` 的脚本会读到 `undefined`，需要使用者改写。仓库内 example-scripts 同步更新；外部脚本由使用者承担。
