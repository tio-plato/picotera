# Plan: Google credential resolvers

## 1. Contract 层枚举扩展

文件：`pkg/contract/endpoint.go`

- 新增常量 `CredentialsResolver_SearchKey int32 = 4`、`CredentialsResolver_GoogApiKey int32 = 5`。
- `ToCredentialsResolver` 增加 `case "searchKey"` / `case "googApiKey"` 分支。
- `FromCredentialsResolver` 增加对应整型分支，返回 `"searchKey"` / `"googApiKey"`。
- `EndpointView.CredentialsResolver` 的 `enum` 标签改为 `enum:"generalApiKey,bearerToken,xApiKey,searchKey,googApiKey,unknown"`。

## 2. `validateClientAuth` 与 resolver 绑定

文件：`pkg/server/gateway_helpers.go`、`pkg/server/handle_gateway.go`

签名改为 `validateClientAuth(r *http.Request, resolver int32) error`，按 resolver 决定可接受位置：

```go
hasBearer := strings.HasPrefix(r.Header.Get("Authorization"), "Bearer ")
hasXApi  := r.Header.Get("X-Api-Key") != ""
hasQuery := r.URL.Query().Get("key") != ""
hasGoog  := r.Header.Get("X-Goog-Api-Key") != ""

switch resolver {
case contract.CredentialsResolver_BearerToken:
    if hasBearer { return nil }
case contract.CredentialsResolver_XApiKey:
    if hasXApi { return nil }
case contract.CredentialsResolver_SearchKey:
    if hasQuery { return nil }
case contract.CredentialsResolver_GoogApiKey:
    if hasGoog { return nil }
default: // GeneralApiKey / Unknown / 其它
    if hasBearer || hasXApi || hasQuery || hasGoog { return nil }
}
return &gatewayError{ status: 401, message: "missing credentials", code: errorx.Unauthorized.Error() }
```

`handle_gateway.go` 中调用点（当前在 line 126 的 `validateClientAuth(r)`）调整为先 `resolveEndpoint` 再 `validateClientAuth(r, endpoint.CredentialsResolver)`。需检查现有顺序：当前代码已是先 resolve endpoint 再 validate，仅多传一个参数。

## 3. 凭证注入：`setCredentialsHeaders` → `applyCredentials`

文件：`pkg/server/gateway_helpers.go`

替换函数签名为：

```go
func applyCredentials(req *http.Request, credentials string, resolver int32, sourceRequest *http.Request)
```

实现：

- `credentials == ""` 直接 return。
- `switch resolver`：
  - `BearerToken`：`req.Header.Set("Authorization", "Bearer "+credentials)`。
  - `XApiKey`：`req.Header.Set("X-Api-Key", credentials)`。
  - `SearchKey`：
    ```go
    q := req.URL.Query()
    q.Set("key", credentials)
    req.URL.RawQuery = q.Encode()
    ```
  - `GoogApiKey`：`req.Header.Set("X-Goog-Api-Key", credentials)`。
  - `GeneralApiKey`：
    - `sourceRequest != nil` 时按以下优先级嗅探（命中即 return）：
      1. `Authorization: Bearer …` → 写 Authorization。
      2. `X-Api-Key` 非空 → 写 X-Api-Key。
      3. URL 查询参数 `key` 非空 → 走 `SearchKey` 分支同款 `q.Set` 逻辑。
      4. `X-Goog-Api-Key` 非空 → 写 X-Goog-Api-Key。
      5. 都无：走"三头兜底"。
    - `sourceRequest == nil` 时直接走"三头兜底"。
    - 三头兜底：`req.Header.Set("Authorization", "Bearer "+credentials)` + `req.Header.Set("X-Api-Key", credentials)` + `req.Header.Set("X-Goog-Api-Key", credentials)`。

## 4. `buildUpstreamRequest` 调整

文件：`pkg/server/gateway_helpers.go`

- 头部剥离 lower 列表追加 `"x-goog-api-key"`（与 `authorization / x-api-key / host / content-length` 同列）。
- 末尾 `setCredentialsHeaders(req.Header, creds, resolver, original)` 改为 `applyCredentials(req, creds, resolver, original)`。

## 5. fetch-models 调用点同步

文件：`pkg/server/handle_provider_endpoint.go:91`

- `setCredentialsHeaders(req.Header, provider.Credentials, endpoint.CredentialsResolver, nil)` 改为 `applyCredentials(req, provider.Credentials, endpoint.CredentialsResolver, nil)`。

## 6. 重新生成 OpenAPI 与 TS 类型

```
mise run openapi
pnpm --dir dashboard generate-openapi
```

确认 `dashboard/src/openapi-types.d.ts` 中 `EndpointView.credentialsResolver` 联合类型新增 `"searchKey" | "googApiKey"`。

## 7. 前端表单

文件：`dashboard/src/components/EndpointForm.vue`

`<Select v-model="form.credentialsResolver">` 中追加：

```vue
<option value="searchKey">Search Key (?key=)</option>
<option value="googApiKey">X-Goog-Api-Key</option>
```

`EndpointsView.vue` 的 `Tag` 渲染保持现状（`generalApiKey` → ok，其它 → muted）。

## 8. 校验

- `go build ./...` 通过。
- `pnpm --dir dashboard type-check` 与 `pnpm --dir dashboard build` 通过。
- 客户端校验：
  1. endpoint=`searchKey`，客户端只带 `Authorization: Bearer` → 401。
  2. endpoint=`searchKey`，客户端带 `?key=…` → 放行。
  3. endpoint=`googApiKey`，客户端只带 `X-Api-Key` → 401。
  4. endpoint=`generalApiKey`，客户端带任一种 → 放行。
- 上游凭证注入：
  1. endpoint=`generalApiKey`，客户端 `?key=…` → 上游 URL `key=<creds>` 被覆盖、三头不带凭证。
  2. endpoint=`generalApiKey`，客户端 `X-Goog-Api-Key` → 上游 `X-Goog-Api-Key=<creds>` 被覆盖且未泄漏客户端值。
  3. endpoint=`searchKey`，fetch-models（nil source）→ 上游 URL `key=<creds>` 被设置。
  4. endpoint=`googApiKey`，fetch-models → 上游 `X-Goog-Api-Key=<creds>` 被设置。
  5. endpoint=`generalApiKey`，fetch-models → 三头同时写入 `Authorization`、`X-Api-Key`、`X-Goog-Api-Key`。
