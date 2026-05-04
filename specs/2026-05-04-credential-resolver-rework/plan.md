# 执行计划

## 1. 数据库迁移

- 新建 `db/migrations/012_provider_endpoint_credentials_resolver.sql`：
  - Up：`ALTER TABLE provider_endpoint ADD COLUMN credentials_resolver INTEGER NOT NULL DEFAULT 0;`
  - Down：`ALTER TABLE provider_endpoint DROP COLUMN credentials_resolver;`

## 2. SQL 查询调整

- `db/queries/provider_endpoint.sql`：
  - `UpsertProviderEndpoint` 的列、`VALUES`、`DO UPDATE SET` 都加上 `credentials_resolver`。
- `db/queries/routing.sql`：
  - `GetProvidersByEndpointAndModel` 的 SELECT 列表加 `pe.credentials_resolver AS send_credentials_resolver`。
- 跑 `sqlc generate` 重新生成 `pkg/db/`。

## 3. 后端凭证逻辑

`pkg/server/gateway_helpers.go`：

1. 改 `extractClientToken`：在具体解析器分支返回前判空，若空则回落到 `pickFirst(bearer, xApi, query, goog)`。
2. 新增 `effectiveSendResolver(endpointResolver, peResolver int32) int32`：`peResolver != Unknown ? peResolver : endpointResolver`。
3. 新增 `mergeClientQuery(upstreamURL *url.URL, clientRawQuery string)`：解析 client query，删除 `key`，把 upstream 自带 query 覆盖到 client 副本上，写回 `RawQuery`。
4. 改 `buildUpstreamRequest` 签名加上 `sendResolver int32`，调用前合并 query，再调用 `applyCredentials(req, creds, sendResolver, original)`。

## 4. 网关串联

`pkg/server/handle_gateway.go`：

1. `providerSidecar` 加 `sendResolver int32`，构造 sidecar 时填入 `effectiveSendResolver(endpoint.CredentialsResolver, row.SendCredentialsResolver)`。
2. 调用 `buildUpstreamRequest` 时传入 `side.sendResolver`。

`pkg/server/handle_provider_endpoint.go::handleFetchModels`：

1. 用 `effectiveSendResolver(endpoint.CredentialsResolver, pe.CredentialsResolver)` 替代直接传 `endpoint.CredentialsResolver`。

## 5. 契约

`pkg/contract/provider_endpoint.go`：

1. `ProviderEndpointView` 加 `CredentialsResolver` 字段（`unknown` 列入 enum 首项）。
2. `ToProviderEndpointView`：`FromCredentialsResolver(pe.CredentialsResolver)`。
3. `FromProviderEndpointView`：`ToCredentialsResolver(view.CredentialsResolver)` 写入 `UpsertProviderEndpointParams`。

## 6. OpenAPI / TS 类型

- `mise run openapi`
- `pnpm --dir dashboard generate-openapi`

## 7. Dashboard

`dashboard/src/components/ProviderEndpointsPanel.vue`：

1. 复用一个 `RESOLVER_OPTIONS` 常量数组（首项 `unknown` 标签为 “继承端点设置”），其余沿用 `EndpointForm.vue` 中文标签。
2. `drafts` 旁加 `resolverDrafts: Record<string, string>`，从 `pe.credentialsResolver` 初始化。
3. 列表行渲染一个 `Select`（`size="sm"`），与 URL 输入并列，复用 `saveDraft`：在保存时同时比较 URL / resolver 是否变化，任一变化即 PUT。
4. 新增表单加 `Field 凭证类型` 默认 `unknown`，`addBinding` 把 `credentialsResolver` 加进 PUT body。

## 8. 联调验证

`pnpm --dir dashboard build && pnpm --dir dashboard type-check && pnpm --dir dashboard lint`。

`go build ./...` 通过。

手测脚本（用 curl）：

- 客户端发 Bearer 但 endpoint=`bearerToken`，正常 200。
- 客户端发 `?key=` 但 endpoint=`bearerToken`，应 200（回退）。
- endpoint=`bearerToken`，provider_endpoint=`xApiKey`，确认上游收到 `X-Api-Key`、没有 `Authorization`。
- 客户端 URL 带 `?alt=sse&key=secret`：上游 URL `?alt=sse` 出现，`key=` 不出现。
- 上游 URL 自带 `?key=fixed`，endpoint=`searchKey`，确认 `?key=` 是凭证而非 fixed（凭证后写）。
