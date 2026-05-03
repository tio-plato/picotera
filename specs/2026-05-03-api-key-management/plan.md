# Execution Plan — API Key 管理

## 1. 数据库与 sqlc

1. 新建 `db/migrations/010_api_key_management.sql`：删除 `api_key_hash` / `api_key_masked`，新增 `key` / `disabled` / `created_at` / `updated_at`，建 `UNIQUE INDEX api_key_key_idx`。
2. 新建 `db/queries/api_key.sql`，写齐 `ListApiKeys` / `GetApiKey` / `GetApiKeyByKey` / `InsertApiKey` / `UpdateApiKey` / `DeleteApiKey`。
3. 跑 `sqlc generate`，确认 `pkg/db/api_key.sql.go`、`pkg/db/models.go`、`pkg/db/querier.go` 正常更新；`db.ApiKey` 字段变为 `ID / Name / Key / Disabled / Annotations / CreatedAt / UpdatedAt`。

## 2. Contract 层

1. 新建 `pkg/contract/api_key.go`：
   - `ApiKeyView` 结构 + `ToApiKeyView(*db.ApiKey) *ApiKeyView`（注意 annotations JSONB 反序列化为 `map[string]string`，参考 `provider_endpoint.go`）。
   - 请求 / 响应类型：`ListApiKeysResponse`、`GetApiKeyRequest/Response`、`ApiKeyMutateBody`、`CreateApiKeyRequest/Response`、`UpdateApiKeyRequest/Response`、`DeleteApiKeyRequest`。
   - Huma operation 常量：`OperationListApiKeys`、`OperationGetApiKey`、`OperationCreateApiKey`、`OperationUpdateApiKey`、`OperationDeleteApiKey`。
2. 默认 key 生成放在 contract 或 server 层均可——选 `pkg/server/handle_api_key.go` 内部辅助函数 `generateApiKey()`：`sk_pt_` + `hex.EncodeToString(crypto/rand 16 bytes)`。

## 3. Server handler

1. 新建 `pkg/server/handle_api_key.go`，实现 5 个 handler，沿用 `handle_script.go` 的错误转换模板（`pgx.ErrNoRows → 404`，`UNIQUE` 冲突 → `409`）。
2. `pkg/server/server.go` 的 `registerOperations()` 中追加 5 行 `huma.Register`。

## 4. errorx

`pkg/errorx`：若没有 `Forbidden` 常量则新增一项 `Forbidden = "FORBIDDEN"`，与 `Unauthorized` 排在一起。

## 5. 网关鉴权改造

> 前置：`google-credential-resolvers` 必须先合入，`validateClientAuth(r, resolver)` 已存在。

1. `pkg/server/gateway_helpers.go`：
   - 把 `validateClientAuth(r, resolver)` 重写为 `(s *Server) authenticateClient(ctx context.Context, r *http.Request, resolver int32) (*db.ApiKey, error)`。
   - **Token 提取位置跟随 resolver**，复用 google-credential-resolvers 中的「客户端凭证识别」表：
     - `bearerToken` → 仅 `Authorization: Bearer <token>`。
     - `xApiKey` → 仅 `X-Api-Key`。
     - `searchKey` → 仅 URL `?key=`。
     - `googApiKey` → 仅 `X-Goog-Api-Key`。
     - `generalApiKey` / `unknown` / 其它 → 按嗅探优先级 Bearer → X-Api-Key → `?key=` → X-Goog-Api-Key 取第一个非空。
   - 任一可接受位置都没填 → `401 missing credentials`。
   - 提到 token 后调 `s.queries.GetApiKeyByKey(ctx, token)`。
   - 错误映射：`pgx.ErrNoRows → gatewayError{401, "invalid api key", Unauthorized}`，`disabled → gatewayError{403, "api key disabled", Forbidden}`，DB 错误 → 500。
2. `pkg/server/handle_gateway.go` step 4：
   - 把调用改为 `apiKey, err := h.authenticateClient(r.Context(), r, endpoint.CredentialsResolver)`（必须在 `resolveEndpoint` 之后）。
   - 之后 `insertRequest`、所有 `updateRequestOnHeader` 的 `ApiKeyID` 字段填 `pgtype.Int4{Int32: apiKey.ID, Valid: true}`。
   - 把 `apiKey.Annotations` JSONB 反序列化为 `map[string]string`，构造 `*jsx.ApiKeySummary`。

## 6. JSX hook ctx

1. `pkg/jsx/types.go`：新增 `ApiKeySummary`，并在 `SortInput` / `RewriteModelInput` / `BeforeRequestInput` / `RewriteInput` 上各加 `ApiKey *ApiKeySummary `json:"apiKey"\`` 字段。
2. `pkg/jsx/session.go`（或 hook 触发点所在文件）：扩展 `RunSortHook` / `RunRewriteModelHook` / `RunBeforeRequestHook` / `RunRewriteHook` 的输入，把 `ApiKey` 透传到 ctx。如签名只接收 `Input` 结构体则字段对应填好即可，不需要新增参数。
3. `handle_gateway.go` 调用处把 `apiKeyJS` 写入对应 input 的 `ApiKey` 字段。
4. `pkg/jsx/sdk.js` 与文档无变化（ctx 是 JSON 注入）。

## 7. OpenAPI / TS 类型

1. `mise run openapi` 重新生成 `openapi.yaml`。
2. `pnpm --dir dashboard generate-openapi` 重新生成 `dashboard/src/openapi-types.d.ts`。
3. 检查 `dashboard/src/api/index.ts` 是否需要 re-export `ApiKeyView`（schema 类型自动跟随）。

## 8. Dashboard

1. `dashboard/src/ui/icons/paths.ts` + `IconName`：补 `key` 图标（来自 `@tabler/icons-vue`）。
2. `dashboard/src/router/index.ts`：新增 `apiKeys` 路由。
3. `dashboard/src/App.vue` 的 `pageMeta`：新增 `apiKeys` 条目。
4. `dashboard/src/components/AppSidebar.vue` 的 `nav` 数组里在 `scripts` 之前插入 `apiKeys`。
5. 新建 `dashboard/src/views/ApiKeysView.vue`：参考 `ScriptsView.vue` 结构；列表列 `名称`、`Key`（明文 + 复制按钮）、操作；操作含禁用/启用、编辑、删除。
6. 新建 `dashboard/src/components/ApiKeyForm.vue`：参考 `ScriptForm.vue`，字段为 `名称` / `Key` (`Input` + 「随机生成」按钮) / `禁用` 复选框 / `Annotations`（复用 `AnnotationsEditor`）。
7. 复制反馈与生成逻辑：
   - 复制：`navigator.clipboard.writeText` + 临时态切换；
   - 生成：`crypto.getRandomValues(new Uint8Array(16))` 转 hex 拼 `sk_pt_` 前缀。

## 9. 验证

1. `go build ./...`、`pnpm --dir dashboard build`、`pnpm --dir dashboard type-check`、`pnpm --dir dashboard lint` 全过。
2. `docker compose up -d` 起依赖，跑 `mise run server` 与 `mise run web`，手测：
   - 新建一个 API Key，复制，未禁用 → 调 `/v1/chat/completions` 类网关路径 → 200 透传。
   - 把 key 禁用 → 同样请求 → 403 `FORBIDDEN`。
   - 用错的 key → 401 `UNAUTHORIZED`。
   - 不带 header → 401 `UNAUTHORIZED`。
   - 写一个简单脚本读 `ctx.apiKey.name` 并 `console.log`，确认请求详情里能看到日志。
3. 检查 `request` 表行的 `api_key_id` 已被填上。
