# Plan: 端点"模型列表"类型

## Step 1: 新增端点类型常量和转换函数

**文件**: `pkg/contract/endpoint.go`

1. 添加常量 `EndpointType_ModelList int32 = 10`。
2. 在 `ToEndpointType` 中添加 `case "modelList": return EndpointType_ModelList`。
3. 在 `FromEndpointType` 中添加 `case EndpointType_ModelList: return "modelList"`。
4. 更新 `EndpointView.EndpointType` 字段的 `enum` tag，追加 `modelList`。

## Step 2: 端点 upsert 校验

**文件**: `pkg/server/handle_endpoint.go`

在 `handleUpsertEndpoint` 中，追加一条校验规则（与 `exaSearch` 校验并列）：当 `endpointType == "modelList"` 且 `modelPath != ""` 时返回 400 错误。

## Step 3: provider_endpoint 绑定限制

**文件**: `pkg/server/handle_provider_endpoint.go`

在 `handleUpsertProviderEndpoint` 中，在执行 upsert 前查询目标端点 `GetEndpointByPath`。若端点存在且 `EndpointType == EndpointType_ModelList`，返回 400："modelList endpoint cannot have provider bindings"。

## Step 4: 新增 sqlc 查询

**文件**: `db/queries/routing.sql`

添加查询 `ListAvailableModelNames`：

```sql
-- name: ListAvailableModelNames :many
SELECT DISTINCT m.name
FROM model AS m
WHERE m.disabled = FALSE
  AND EXISTS (
    SELECT 1
    FROM provider AS p
    CROSS JOIN LATERAL jsonb_array_elements(p.provider_models) AS elem
    JOIN provider_endpoint AS pe ON pe.provider_id = p.id
    WHERE p.provider_models @> jsonb_build_array(jsonb_build_object('model', m.name))
      AND elem ->> 'model' = m.name
      AND p.disabled = FALSE
      AND COALESCE((elem ->> 'disabled')::boolean, false) = false
      AND pe.upstream_url <> ''
      AND p.credentials <> ''
      AND (
        elem -> 'endpoints' IS NULL
        OR jsonb_typeof(elem -> 'endpoints') <> 'array'
        OR jsonb_array_length(elem -> 'endpoints') = 0
        OR elem -> 'endpoints' @> to_jsonb(ARRAY[pe.endpoint_path])
      )
  )
ORDER BY m.name;
```

然后运行 `sqlc generate`。

## Step 5: 网关分发和 handleModelList

**文件**: `pkg/server/handle_gateway.go`

在 `gatewayHandler.ServeHTTP` 中，端点路由匹配成功（step 1）后，检查 `endpoint.EndpointType == contract.EndpointType_ModelList`：若是，则调用 `h.handleModelList(w, r, endpoint)` 并 return，跳过标准网关流程。

**新文件**: `pkg/server/handle_model_list.go`

实现 `handleModelList` 方法：

```go
func (h *gatewayHandler) handleModelList(w http.ResponseWriter, r *http.Request, endpoint db.Endpoint) {
    // 1. 检查 Method
    if r.Method != http.MethodGet && r.Method != http.MethodHead {
        writeGatewayError(w, http.StatusNotFound, "route not found", errorx.RouteNotFound.Error())
        return
    }

    // 2. 关闭 body
    r.Body.Close()

    // 3. 认证
    _, err := h.authenticateClient(r.Context(), r, endpoint.CredentialsResolver)
    if err != nil {
        handleGatewayErr(w, err)
        return
    }

    // 4. 查询可用模型
    names, err := h.queries.ListAvailableModelNames(r.Context())
    if err != nil {
        writeGatewayError(w, http.StatusInternalServerError, "failed to query models", errorx.InternalError.Error())
        return
    }

    // 5. 构建响应
    type entry struct {
        ID     string `json:"id"`
        Object string `json:"object"`
    }
    data := make([]entry, len(names))
    for i, n := range names {
        data[i] = entry{ID: n, Object: "model"}
    }

    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(http.StatusOK)
    json.NewEncoder(w).Encode(map[string]any{
        "object": "list",
        "data":   data,
    })
}
```

## Step 6: 重新生成 OpenAPI 规范和 TS 类型

1. 运行 `mise run openapi`。
2. 运行 `pnpm --dir dashboard generate-openapi`。

## Step 7: Dashboard 前端

**文件**: `dashboard/src/api/index.ts`

1. 在 `ENDPOINT_TYPE_LABELS` 中添加 `modelList: '模型列表'`。

**文件**: `dashboard/src/components/EndpointForm.vue`

1. 更新 `isModelPathLocked` 计算属性：当 `endpointType` 为 `modelList` 或 `exaSearch` 时锁定。
2. 更新 `endpointType` watcher：当类型变为 `modelList` 时清空 `modelPath`。
3. 更新 `modelPath` 字段的 placeholder：当为 `modelList` 时显示"模型列表端点不解析模型"。

## Step 8: 验证

1. 运行 `go build ./cmd/picotera` 确认编译通过。
2. 运行 `pnpm --dir dashboard type-check` 确认前端类型正确。
3. 运行 `pnpm --dir dashboard lint` 确认 lint 通过。
