# Plan

## 1. 后端 contract

`pkg/contract/endpoint.go`：

- 增加常量 `EndpointType_ExaSearch int32 = 9`。
- `ToEndpointType`：增加 `case "exaSearch": return EndpointType_ExaSearch`。
- `FromEndpointType`：增加 `case EndpointType_ExaSearch: return "exaSearch"`。
- `EndpointView` 的 `EndpointType` 字段 `enum:"..."` 标签尾部追加 `,exaSearch`。

## 2. 后端 upsert 校验

`pkg/server/handle_endpoint.go::handleUpsertEndpoint`：

- 在把 view 转换成 sqlc 入参之前增加一段：
  - 当 `req.Body.EndpointType == "exaSearch"` 且 `req.Body.ModelPath != ""` 时，返回 `huma.Error400BadRequest("exaSearch endpoint must have empty modelPath")`（与文件内已有 400 形式一致；以实际现存写法为准）。
- 其它代码路径不动。

## 3. OpenAPI / 前端类型同步

- 运行 `mise run openapi`，确认 `openapi.yaml` 中 `EndpointView.endpointType.enum` 多了 `exaSearch`。
- 运行 `pnpm --dir dashboard generate-openapi`，确认 `dashboard/src/openapi-types.d.ts` 同步。

## 4. 前端字面量

`dashboard/src/api/index.ts`：

- `ENDPOINT_TYPE_LABELS` 中追加一行 `exaSearch: 'Exa 搜索'`。
- `ENDPOINT_TYPES_MODEL_ROUTED` 保持不变。

## 5. 前端表单 `EndpointForm.vue`

- `<script setup>` 中引入 `watch`（如未引入）。
- 新增：

  ```ts
  const isModelPathLocked = computed(() => form.value.endpointType === 'exaSearch')
  watch(
    () => form.value.endpointType,
    (t) => {
      if (t === 'exaSearch') form.value.modelPath = ''
    },
  )
  ```

- 模板里 `<Input v-model="form.modelPath" ... />` 改为：
  - 增加 `:disabled="isModelPathLocked"`。
  - `placeholder` 改成动态绑定：
    ```vue
    :placeholder="isModelPathLocked ? 'Exa 搜索端点不解析模型' : '可选，留空表示该端点不解析模型'"
    ```

## 6. 前端列表 `EndpointsView.vue`

- `endpointTypeVariant`：在 `ENDPOINT_TYPES_MODEL_ROUTED.includes(t)` 分支之后、`general` 分支之前，增加：
  ```ts
  if (t === 'exaSearch') return 'more'
  ```
  让样式与默认 `more` 分支一致，但显式标注，便于以后调色。

## 7. 校验

- `go build ./...`。
- `pnpm --dir dashboard type-check`。
- `pnpm --dir dashboard lint`。
- 手工：在本地建一条 `endpointType=exaSearch`、`path=/api/search`、`credentialsResolver=xApiKey` 的端点；绑定一个 dummy provider；
  - 确认表单中模型字段路径被禁用并自动清空。
  - 确认列表里类型列显示 "Exa 搜索"。
  - 用 `curl` 试着把 `modelPath` 设成非空，PUT `/api/picotera/endpoints`，应被后端 400 拒绝。
  - 向 `/api/search` 发任意 JSON body，确认网关按 no-model 端点路径执行，meta + upstream request 行的 `model` / `upstream_model` 为 NULL，上游收到的请求体未被改写。
