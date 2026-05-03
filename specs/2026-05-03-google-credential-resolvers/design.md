# Design: Google credential resolvers

## 背景

当前 `endpoint.credentials_resolver` 支持三种枚举：`generalApiKey / bearerToken / xApiKey`，对应 `Authorization: Bearer` 与 `X-Api-Key` 两种凭证位置。Google Gemini 等上游使用 `?key=` 查询参数或 `X-Goog-Api-Key` 请求头投递凭证，需要扩展凭证识别与注入路径以接入它们。

本次同时把这两种位置纳入 `generalApiKey` 的"嗅探—回写"逻辑，并新增两种专用解析器：

- `searchKey` —— 上游凭证固定写入 URL 查询参数 `key`。
- `googApiKey` —— 上游凭证固定写入请求头 `X-Goog-Api-Key`。

## 数据模型

### 枚举扩展（`pkg/contract/endpoint.go`）

```go
const (
    CredentialsResolver_Unknown       int32 = 0
    CredentialsResolver_GeneralApiKey int32 = 1
    CredentialsResolver_BearerToken   int32 = 2
    CredentialsResolver_XApiKey       int32 = 3
    CredentialsResolver_SearchKey     int32 = 4
    CredentialsResolver_GoogApiKey    int32 = 5
)
```

`ToCredentialsResolver` / `FromCredentialsResolver` 增加 `searchKey` / `googApiKey` 字符串映射；`EndpointView.CredentialsResolver` 的 `enum` 标签同步扩展为 `generalApiKey,bearerToken,xApiKey,searchKey,googApiKey,unknown`。

无数据库迁移：`credentials_resolver` 仍是整型列，存量行不受影响。

## 网关层（`pkg/server/gateway_helpers.go`）

### `validateClientAuth`

签名调整为 `validateClientAuth(r *http.Request, resolver int32) error`，按 endpoint 的 `credentialsResolver` 决定可接受的凭证位置：

| resolver        | 接受的客户端凭证位置                                        |
| --------------- | ----------------------------------------------------------- |
| `generalApiKey` | `Authorization: Bearer` / `X-Api-Key` / `?key=` / `X-Goog-Api-Key` 任一 |
| `bearerToken`   | 仅 `Authorization: Bearer`                                  |
| `xApiKey`       | 仅 `X-Api-Key`                                              |
| `searchKey`     | 仅 URL 查询参数 `key=`                                      |
| `googApiKey`    | 仅 `X-Goog-Api-Key`                                         |
| 其它（含 `unknown`） | 同 `generalApiKey`，作为兜底                            |

未命中时返回原有 401 + `errorx.Unauthorized`。调用点在 `handle_gateway.go` 中 `resolveEndpoint` 之后、即 `endpoint.CredentialsResolver` 已可用的位置。

### 凭证注入：从 `setCredentialsHeaders` 重构为 `applyCredentials`

当前函数仅修改 `http.Header`，无法表达 `?key=` 这种 URL 改写。重构签名如下：

```go
func applyCredentials(req *http.Request, credentials string, resolver int32, sourceRequest *http.Request)
```

调用方在已构造好 `*http.Request`（含上游 URL 与 header）后调用。函数职责：

- `BearerToken`：设置 `Authorization: Bearer <creds>`。
- `XApiKey`：设置 `X-Api-Key: <creds>`。
- `SearchKey`：在 `req.URL.Query()` 上 `Set("key", creds)` 并回写 `req.URL.RawQuery`（`Set` 语义保证已存在的 `key` 被覆盖，与 Google 上游 URL 中预置的 `key=API_KEY` 占位也兼容）。
- `GoogApiKey`：设置 `X-Goog-Api-Key: <creds>`。
- `GeneralApiKey`：
  - `sourceRequest != nil` 时按嗅探优先级单点回写。
  - `sourceRequest == nil` 或四种位置全部缺失时，统一走"三头兜底"：同时写 `Authorization: Bearer <creds>`、`X-Api-Key: <creds>`、`X-Goog-Api-Key: <creds>`，不写 `?key=` 查询参数。

嗅探优先级（自上而下命中即返回）：

1. `Authorization: Bearer` → 写 `Authorization: Bearer <creds>`。
2. `X-Api-Key` → 写 `X-Api-Key: <creds>`。
3. URL 查询参数 `key` → 在上游 URL 上 `Set("key", creds)`。
4. `X-Goog-Api-Key` → 写 `X-Goog-Api-Key: <creds>`。
5. 都没命中：兜底同时写三种 header（`Authorization: Bearer` + `X-Api-Key` + `X-Goog-Api-Key`）。

### `buildUpstreamRequest`：剥离请求头白名单

复制原始请求头时新增剥离项：除 `authorization / x-api-key / host / content-length` 之外，还需要剥离 `x-goog-api-key`，避免客户端凭证泄漏到上游。客户端的 `?key=` 由于 picotera 不复制原始 URL（上游 URL 来自 provider 配置，仅做 `{name}` 占位替换），不会传递；`applyCredentials` 内部走 `Set` 语义，逻辑闭合。

调用顺序：先 `buildUpstreamRequest` 构造 `req`，再 `applyCredentials(req, creds, resolver, original)`。`applyCredentials` 在内部修改 `req.URL`，与 `substitutePathVars` 后续无依赖。

### `handle_provider_endpoint.go::handleFetchModels`

`setCredentialsHeaders(req.Header, provider.Credentials, endpoint.CredentialsResolver, nil)` 调用替换为 `applyCredentials(req, provider.Credentials, endpoint.CredentialsResolver, nil)`。`searchKey` 与 `googApiKey` 在 `nil sourceRequest` 时仍按确定性规则注入（query 或 goog header），不走兜底分支。

## 前端

### `EndpointForm.vue`

`<Select v-model="form.credentialsResolver">` 增加两个 `<option>`：

```vue
<option value="searchKey">Search Key (?key=)</option>
<option value="googApiKey">X-Goog-Api-Key</option>
```

无新表单字段。

### `EndpointsView.vue`

列表中 `Tag` 的 variant 仍按 `credentialsResolver === 'generalApiKey' ? 'ok' : 'muted'` 渲染；新枚举落到 `muted`，无需特殊处理。

### OpenAPI 类型

`pkg/contract/endpoint.go` 中 `EndpointView.CredentialsResolver` 的 `enum` 字符串扩展后，依次执行 `mise run openapi` 与 `pnpm --dir dashboard generate-openapi`，`dashboard/src/openapi-types.d.ts` 自动包含新枚举字面量。

## 风险与权衡

- **`X-Goog-Api-Key` 头白名单遗漏**：若不在 `buildUpstreamRequest` 剥离列表中，客户端 `X-Goog-Api-Key` 会原样泄漏到上游，绕过 picotera 的凭证替换。剥离后由 `applyCredentials` 单点写入。
- **`searchKey` 在 nil sourceRequest 下的行为**：fetch-models / 内部调用没有 source request，但 `searchKey` 与 `googApiKey` 的注入位置是确定的，不走"双写兜底"路径，行为可预测。
- **`generalApiKey` 嗅探顺序固定**：客户端若同时携带多种凭证，`Authorization` 优先于 `X-Api-Key` 优先于 `?key=` 优先于 `X-Goog-Api-Key`。现实中很少同时存在，无需暴露策略开关。
