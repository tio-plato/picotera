# Design — API Key 管理

## 目标

- 给运营者一个完整的 API Key CRUD 资源（管理 API + Dashboard 视图），形态对齐现有 `script` 资源。
- 网关侧（所有命中 `endpoint` 的 LLM 请求）必须验证客户端携带的 API Key 是否存在且未被禁用；管理 API（`/api/picotera/*`）与 Dashboard SPA 静态资源不受影响。
- API Key 以明文存储，前端可见、可复制、可在创建/编辑时自定义；默认格式 `sk_pt_<32 hex>`。
- 钩子脚本能在请求生命周期内读取当前命中的 `apiKey` 元数据（`id` / `name` / `annotations`），但不暴露 key 本体。

## 数据模型

### Migration `010_api_key_management.sql`

现有 `api_key` 表只用作 `request.api_key_id` 的外键名义，`api_key_hash` / `api_key_masked` 列从未被读写。本次直接重写：

```sql
-- +goose Up
ALTER TABLE api_key DROP COLUMN api_key_hash;
ALTER TABLE api_key DROP COLUMN api_key_masked;
ALTER TABLE api_key ADD COLUMN key TEXT NOT NULL;
ALTER TABLE api_key ADD COLUMN disabled BOOLEAN NOT NULL DEFAULT FALSE;
ALTER TABLE api_key ADD COLUMN created_at TIMESTAMPTZ NOT NULL DEFAULT now();
ALTER TABLE api_key ADD COLUMN updated_at TIMESTAMPTZ NOT NULL DEFAULT now();
CREATE UNIQUE INDEX api_key_key_idx ON api_key (key);
```

`Down` 反向回滚到旧列形态（保留兼容性，回滚时新增的运行期数据会丢）。

### sqlc 查询（`db/queries/api_key.sql`）

- `ListApiKeys` — `ORDER BY created_at DESC, id DESC`，列表用。
- `GetApiKey` — 按 `id` 取单条。
- `GetApiKeyByKey` — 按明文 `key` 命中，用于网关鉴权热路径。返回完整行（含 `disabled`、`annotations`），让上层区分 401 与 403。
- `InsertApiKey` — 插入 `(name, key, disabled, annotations)`。
- `UpdateApiKey` — 全量更新可变字段；`updated_at = now()`。
- `DeleteApiKey` — 按 id 删除。

`pkg/db/` 由 `sqlc generate` 重新产出，包括 `Querier` 接口里的新方法和 `db.ApiKey` 模型字段调整（移除 `ApiKeyHash` / `ApiKeyMasked`，新增 `Key`、`Disabled`、`CreatedAt`、`UpdatedAt`）。

`request.api_key_id` 字段保持现状（已是 nullable 整型）；网关命中 API Key 后写入这个字段。

## 网关鉴权

> 前置依赖：`google-credential-resolvers` 已落地，`validateClientAuth(r *http.Request, resolver int32) error` 已经按 resolver 决定可接受的客户端凭证位置。本节在它的基础上替换为带 DB 查询的版本。

`pkg/server/gateway_helpers.go` 的 `validateClientAuth` 改造方向：

1. 替换为 `(s *Server) authenticateClient(ctx context.Context, r *http.Request, resolver int32) (*db.ApiKey, error)`。
2. **Token 提取位置跟随 resolver**，与 `google-credential-resolvers` 设计的「客户端凭证识别」表完全一致：
   - `bearerToken`：仅从 `Authorization: Bearer <token>` 取。
   - `xApiKey`：仅从 `X-Api-Key` 取。
   - `searchKey`：仅从 URL 查询参数 `key` 取。
   - `googApiKey`：仅从 `X-Goog-Api-Key` 取。
   - `generalApiKey` / `unknown` / 其它：按嗅探优先级（Bearer → X-Api-Key → `?key=` → X-Goog-Api-Key）取第一个非空。
   - 任一可接受位置都没填 → `401 unauthorized: missing credentials`。
3. 提取出来的 token 用 `GetApiKeyByKey(ctx, token)` 查库：
   - `pgx.ErrNoRows` → `401 unauthorized: invalid api key`。
   - `disabled = TRUE` → `403 forbidden: api key disabled`。
   - 其余 DB 错误 → `500 internal error`。
4. 命中的行返回给 `handle_gateway.go`：
   - 在 step 4 的位置（`validateClientAuth` 之后）保存到局部变量 `apiKey *db.ApiKey`。
   - 后续 `insertRequest` / `updateRequestOnHeader` 调用把 `ApiKeyID: pgtype.Int4{Int32: apiKey.ID, Valid: true}` 写入。
   - 在构建 jsx ctx 时通过新增的 `jsx.ApiKeySummary` 注入。
5. 鉴权失败的请求当前会落到 `failMeta` + `failMetaResponse`，路径不变。

> 静态 SPA 路径（`resolveEndpoint` 走到 `isRouteNotFound` 后转发给 `staticHandler`）与管理 API（`/api/picotera/*`，由 `huma.NewGroup` 在 router 上挂载，**早于** `gatewayHandler`）都不会进入 `validateClientAuth`，符合“管理面板 API 不需要鉴权”的要求。

错误码沿用 `pkg/errorx` 现有常量：未授权用 `Unauthorized`，禁用用新增的 `Forbidden`（如不存在则在 `errorx` 里添加一行）。

## Hook 暴露

### 类型（`pkg/jsx/types.go`）

```go
type ApiKeySummary struct {
    ID          int32             `json:"id"`
    Name        string            `json:"name"`
    Annotations map[string]string `json:"annotations"`
    Disabled    bool              `json:"disabled"`
}
```

`SortInput` / `RewriteModelInput` / `BeforeRequestInput` / `RewriteInput` 各新增一个 `ApiKey *ApiKeySummary` 字段（`json:"apiKey"`）。`RewriteProviderModelsInput` 不动——它是后台 fetch-models 流程，与具体客户端请求无关，没有 `apiKey` 概念。

key 本体 **不** 进入 JS 边界，与 provider credentials 处理一致。

### 注入

`handle_gateway.go` 在第 6 步前后构造 `apiKeyJS *jsx.ApiKeySummary`：
- 若 `apiKey.Annotations` 是 JSONB，需要解析成 `map[string]string`。可参考 `pkg/db/models.go` 中现有 annotations 字段的处理（`provider_endpoint.go` 等已有先例）。
- 把 `apiKeyJS` 传给 `RunRewriteModelHook` / `RunSortHook` / `RunBeforeRequestHook` / `RunRewriteHook`。这些函数签名都需要补充 `apiKey` 参数。

## Dashboard

### 路由 / 导航

- `dashboard/src/router/index.ts`：新增 `{ path: '/api-keys', name: 'apiKeys', component: () => import('@/views/ApiKeysView.vue') }`。
- `dashboard/src/App.vue` 的 `pageMeta` map：`apiKeys: { title: 'API Key', hint: '客户端调用网关的访问凭证' }`。
- `dashboard/src/components/AppSidebar.vue` 的 `nav`：在 `scripts` 之前插入 `{ name: 'apiKeys', label: 'API Key', icon: 'key' }`。`@tabler/icons-vue` 的 `key` 图标需要补到 `dashboard/src/ui/icons/paths.ts` 与 `IconName` 类型。

### 视图（`ApiKeysView.vue`）

形态与 `ScriptsView.vue` 完全对齐：列表展示名称、状态徽标（禁用 → `Tag variant="muted"`）、key 摘要（明文，前端用等宽字体显示，附复制按钮），右侧操作列「禁用 / 启用 / 编辑 / 删除」。
- 复制按钮调用 `navigator.clipboard.writeText(item.key)`，给出短暂的「已复制」反馈（用现有 `Tag` 或 toast 模式即可）。
- 禁用切换走 `PUT /api/picotera/api-keys/{id}`，请求体里把 `disabled` 翻转。

### 表单（`ApiKeyForm.vue`）

形态参考 `ScriptForm.vue` + `ProviderForm`：
- `名称`（`Input`，必填）。
- `Key` 字段：`Input`，可编辑，附「随机生成」按钮（前端用 `crypto.getRandomValues(new Uint8Array(16))` 转 hex 拼前缀）。新建时默认值即一次随机生成的串。
- `禁用`：`<input type="checkbox">`，与 `ScriptForm` 的「启用」复选框样式一致；语义反过来——存的是 `disabled`。
- `Annotations`：复用 `AnnotationsEditor.vue`。
- 与 `ScriptForm` 一样，用 `useApi()` 调 OpenAPI 生成的 typed client。

### 类型流

OpenAPI / TS 类型按 `CLAUDE.md` 的工作流：先在 `pkg/contract/api_key.go` 加 Huma operation，跑 `mise run openapi`，再跑 `pnpm --dir dashboard generate-openapi`，前端从 `@/api` 直接拿 `ApiKeyView`。

## 风险与权衡

- **明文存储** —— 与现有 `provider.credentials` 同等敏感，但 Postgres 已直接持久化 provider 凭证，新增 key 列与之同等防护级别即可；不引入额外加密层。
- **lookup 性能** —— 网关每个请求一次 `GetApiKeyByKey`；`UNIQUE INDEX` 命中即可，未来若热路径要求更低延迟，可在 KeyDB（已部署）上做反向缓存，本期不做。
- **hash 列移除** —— 旧表里的 `api_key_hash` / `api_key_masked` 从未被业务读写（`request.api_key_id` 一直是 NULL），删列对生产无影响；migration `Down` 仅恢复列结构，原始数据不可逆。
