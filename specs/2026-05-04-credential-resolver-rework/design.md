# 设计：凭证解析重构

## 背景

当前网关在 `pkg/server/gateway_helpers.go` 内实现了三组凭证逻辑：

- `extractClientToken` —— 严格按 `endpoint.credentials_resolver` 取一个位置，只有 `generalApiKey` 才会扫描全部位置。
- `applyCredentials` —— 把上游凭证按解析器写入新请求；`generalApiKey` 时会模仿客户端的位置或同时写三个 header。
- `buildUpstreamRequest` —— 抹去四个凭证 header，再调用 `applyCredentials`；不转发客户端 URL 的 search query。

读、写两侧都被同一个 `credentials_resolver` 字段绑死，而 provider 与 provider_endpoint 之间没有任何方式对发送格式进行覆盖；而且客户端 search query 全部丢失，对部分上游（例如 Gemini 的 `?alt=sse`）是个限制。

## 总体方案

引入“读侧优先 + 全位回退”、“写侧由 endpoint+provider_endpoint 共同决定”、“透传非凭证 query”三条规则：

1. **读侧 (extractClientToken)**：先从 `endpoint.credentials_resolver` 指定位置读取；若空，则按固定优先级回退 (Bearer → X-Api-Key → `?key=` → X-Goog-Api-Key)。`generalApiKey` / `unknown` 行为不变 —— 它们本身就走全扫描。
2. **写侧 (applyCredentials)**：使用“有效发送解析器” = `provider_endpoint.credentials_resolver`（若非 `unknown`），否则 fallback 到 `endpoint.credentials_resolver`。语义和今天完全一致；只是多了一层覆盖。
3. **Search query 处理 (buildUpstreamRequest)**：构造上游请求时，把客户端 URL 上的 query 参数合并到上游 URL（去掉 `key`），冲突时上游 URL 自带的键胜出；写凭证仍然作用于上游 URL 的 query（仅当解析器是 `searchKey`）。

## 数据模型变更

新增迁移 `db/migrations/012_provider_endpoint_credentials_resolver.sql`：

```sql
ALTER TABLE provider_endpoint
  ADD COLUMN credentials_resolver INTEGER NOT NULL DEFAULT 0;
```

`0` 即 `CredentialsResolver_Unknown`，语义为 “继承 endpoint 的解析器”。复用现有常量集，不需要新枚举。

## 后端改动

### `pkg/server/gateway_helpers.go`

- `extractClientToken(r, resolver)`：保留签名，调整实现。当 resolver 命中具体位置但该位置为空时，按 `pickFirst(bearer, xApi, query, goog)` 回退。`generalApiKey` / `unknown` 走原 `pickFirst` 流程不变。
- `applyCredentials(req, credentials, resolver, sourceRequest)`：实现保持不变；调用方传入“有效发送解析器”即可。
- 新增辅助函数 `mergeClientQuery(upstreamURL *url.URL, clientRawQuery string)`：把 `clientRawQuery` 解析为 `url.Values`，删除 `key`，再把上游 URL 自带的键值复制覆盖到 client 副本上，最后写回 `upstreamURL.RawQuery`。
- `buildUpstreamRequest`：
  - 接收一个新参数 `sendResolver int32`，该值由调用方根据 endpoint + provider_endpoint 计算。
  - 在 `http.NewRequestWithContext` 之后，调用 `mergeClientQuery` 合并 query。
  - 调用 `applyCredentials` 时使用 `sendResolver`。

### `pkg/server/handle_gateway.go`

- `providerSidecar` 增加 `sendResolver int32` 字段。
- 在构造 `sidecar` 时填入 `effectiveSendResolver(endpoint.CredentialsResolver, row.SendCredentialsResolver)`，规则：`row.SendCredentialsResolver != Unknown ? row.SendCredentialsResolver : endpoint.CredentialsResolver`。
- 调用 `buildUpstreamRequest` 时把 `side.sendResolver` 透传进去。

### `pkg/server/handle_provider_endpoint.go::handleFetchModels`

`/fetch-models` 路径同样需要按 `effectiveSendResolver` 计算。`pe.CredentialsResolver` 已经从 sqlc 拿到，加一行 fallback 计算即可。

### `db/queries/provider_endpoint.sql`

- `UpsertProviderEndpoint`：列、`VALUES` 与 `DO UPDATE` 都加上 `credentials_resolver`。
- `GetProviderEndpoint`、`ListProviderEndpoints`：`SELECT *` 自动带新列，无需改 SQL。

### `db/queries/routing.sql::GetProvidersByEndpointAndModel`

`SELECT pe.credentials_resolver AS send_credentials_resolver` 加入返回列。

### `pkg/contract/provider_endpoint.go`

`ProviderEndpointView` 增加：

```go
CredentialsResolver string `json:"credentialsResolver" enum:"unknown,generalApiKey,bearerToken,xApiKey,searchKey,googApiKey"`
```

`unknown` 语义 = 继承。`To*` / `From*` 用现有的 `FromCredentialsResolver` / `ToCredentialsResolver` 工具。

## 前端改动

### `dashboard/src/components/ProviderEndpointsPanel.vue`

- 现有列表里的每行加一个 `Select` 凭证解析器，绑定到 `drafts` 旁的 `resolverDrafts` 状态；保存逻辑在 `saveDraft`、`addBinding` 与 `PUT` 的 body 同步带上 `credentialsResolver`（默认 `unknown`）。
- 新增绑定表单加一个 `Field` 选项，默认 `unknown` 显示为 “继承端点设置”。
- 选项与 `EndpointForm.vue` 复用，但首项必须是 `unknown` → “继承端点设置”。

### OpenAPI 类型

`mise run openapi && pnpm --dir dashboard generate-openapi` 重新生成。

## 行为对照表

| 场景 | 当前 | 改后 |
|------|------|------|
| 客户端用 Bearer，endpoint=bearerToken | 读到 | 读到 |
| 客户端用 ?key=，endpoint=bearerToken | 401 | 读到（回退命中 query） |
| 上游写凭证，endpoint=bearerToken | Authorization | Authorization |
| 上游写凭证，endpoint=bearerToken + provider_endpoint=xApiKey | Authorization | X-Api-Key |
| 客户端 URL `?alt=sse&key=…` 转发 | 不转发，丢 alt 与 key | 转发 alt，丢 key |
| 上游 URL 自带 `?key=fixed`，客户端 `?key=client` | 上游被 client 覆盖（searchKey 时） | searchKey 时凭证覆盖 fixed；其它解析器下 client 被忽略 |

## 不在范围内

- 不改 `pkg/jsx` 的 `ProviderSummary` / `Candidate` 形状；hooks 仍只看 endpoint 的 `credentialsResolver`。
- 不引入新 `CredentialsResolver` 常量。
- 不动现有 OpenAPI operation ID 或路径。
