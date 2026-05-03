# Plan: Endpoint type field

## 1. 数据库迁移

1.1 新建 `db/migrations/009_endpoint_type.sql`：

```sql
-- +goose Up
ALTER TABLE endpoint ADD COLUMN endpoint_type INTEGER NOT NULL DEFAULT 1; -- general

-- +goose Down
ALTER TABLE endpoint DROP COLUMN endpoint_type;
```

1.2 启动 `docker compose up -d`，运行 `mise run server` 一次以确认迁移生效（程序启动时自动跑）。

## 2. sqlc 查询

2.1 `db/queries/endpoint.sql`：

- `GetEndpoints`：保持 `SELECT *`，无需改动 SQL（schema 自动反映）。
- `UpsertEndpoint` 改为：
  ```sql
  INSERT INTO endpoint (name, path, model_path, credentials_resolver, endpoint_type)
  VALUES ($1, $2, $3, $4, $5)
  ON CONFLICT (path) DO UPDATE
    SET model_path = $3, credentials_resolver = $4, endpoint_type = $5
  RETURNING *;
  ```

2.2 运行 `sqlc generate`。验证 `pkg/db/`：

- `Endpoint.EndpointType` 类型为 `int32`。
- `UpsertEndpointParams` 新增 `EndpointType int32` 字段，`ModelPath` 仍为 `string`。

## 3. Backend 类型与 handler

3.1 `pkg/contract/endpoint.go`：

- 新增枚举常量与转换函数：
  ```go
  const (
      EndpointType_Unknown               int32 = 0
      EndpointType_General               int32 = 1
      EndpointType_OpenAIChatCompletions int32 = 2
      EndpointType_OpenAIResponses       int32 = 3
      EndpointType_AnthropicMessages     int32 = 4
      EndpointType_AnthropicCountTokens  int32 = 5
      EndpointType_GeneralListModels     int32 = 6
  )

  func ToEndpointType(s string) int32 { /* switch */ }
  func FromEndpointType(t int32) string { /* switch */ }
  ```
- `EndpointView` 增加字段：
  ```go
  type EndpointView struct {
      Name                string `json:"name"`
      Path                string `json:"path"`
      ModelPath           string `json:"modelPath"`
      CredentialsResolver string `json:"credentialsResolver" enum:"generalApiKey,bearerToken,xApiKey,unknown"`
      EndpointType        string `json:"endpointType" enum:"general,openaiChatCompletions,openaiResponses,anthropicMessages,anthropicCountTokens,generalListModels,unknown"`
  }
  ```
- `ToEndpointView`：新增 `EndpointType: FromEndpointType(endpoint.EndpointType)`。

3.2 `pkg/server/handle_endpoint.go`：

- `handleUpsertEndpoint` 构造 `db.UpsertEndpointParams` 增加 `EndpointType: contract.ToEndpointType(input.Body.EndpointType)`。

3.3 `pkg/server/gateway_helpers.go::extractModel`：函数体起首加一行：
```go
if modelPath == "" {
    return "", &gatewayError{
        status:  http.StatusBadRequest,
        message: "endpoint has no model path configured",
        code:    errorx.ModelNotFound.Error(),
    }
}
```
（保留原本对 gjson 结果为空的处理。）

3.4 `go build ./...` 通过。

## 4. OpenAPI 与前端类型

4.1 运行 `mise run openapi` 重新生成 `openapi.yaml`。

4.2 启动 `mise run web`（或 `pnpm --dir dashboard build`）触发 `openapi-typescript`，确认 `dashboard/src/api.d.ts`：

- `EndpointView.endpointType` 字段存在，包含全部七个字面量。
- `EndpointView.modelPath` 为 `string`。

4.3 `dashboard/src/api/index.ts` 增加：

```ts
export type EndpointType = NonNullable<EndpointView['endpointType']>
export const ENDPOINT_TYPES_MODEL_ROUTED: EndpointType[] = [
  'openaiChatCompletions',
  'openaiResponses',
  'anthropicMessages',
  'anthropicCountTokens',
]
export const ENDPOINT_TYPES_DIRECT: EndpointType[] = ['general', 'generalListModels']
export const ENDPOINT_TYPE_LABELS: Record<EndpointType, string> = {
  general: '通用',
  openaiChatCompletions: 'OpenAI 聊天补全',
  openaiResponses: 'OpenAI 响应',
  anthropicMessages: 'Anthropic 消息',
  anthropicCountTokens: 'Anthropic Tokens 计数',
  generalListModels: '模型列表',
  unknown: '未知',
}
```

## 5. 前端 UI

5.1 `dashboard/src/components/EndpointForm.vue`：

- 表单 state 加 `endpointType: EndpointType`，默认 `props.endpoint?.endpointType ?? 'general'`。
- `modelPath` 仍是 `string`，submit 前根据类型决定校验：
  - 模型路由型：trim 后必填，空时阻止提交并 `error.value = '该端点类型必须填写模型字段路径'`。
  - 直传/列表型：允许空串，原样提交（落库即空字符串）。
- 模板新增两块：
  - `<Field label="类型">` 包含 `<Select v-model="form.endpointType">`，使用 `ENDPOINT_TYPE_LABELS` 渲染六个有效项 + `unknown`（仅在原始数据是 unknown 时可见，新增表单不显示）。
  - 模型字段路径的 `<Field>` 根据类型动态切换 `required` 与 placeholder（`可选` / `例如 body.model`）。
- PUT 请求体新增 `endpointType`，其它字段不变。

5.2 `dashboard/src/views/EndpointsView.vue`：

- 表头加 `<Th>类型</Th>` 列（位置：路径、名称、**类型**、模型字段、凭证解析、actions）。
- 行渲染：`<Tag :variant="endpointTypeVariant(e.endpointType)">{{ ENDPOINT_TYPE_LABELS[e.endpointType] }}</Tag>`，其中 `endpointTypeVariant` 把模型路由型映射 `accent`、直传型映射 `muted`、`unknown` 映射 `more`。
- "模型字段" 列：`<span class="font-mono text-ink-faint">{{ e.modelPath || '—' }}</span>`。

5.3 `dashboard/src/components/ProviderModelsPanel.vue`：

- `load()` 内并行追加 `GET /endpoints`，存储到 `endpoints.value: EndpointView[]`。
- 计算属性 `groupedFetchSources`：
  ```ts
  const epByPath = new Map(endpoints.value.map((e) => [e.path, e]))
  const listModels: string[] = []
  const general: string[] = []
  for (const pe of providerEndpoints.value) {
    const t = epByPath.get(pe.endpointPath)?.endpointType
    if (t === 'generalListModels') listModels.push(pe.endpointPath)
    else if (t === 'general') general.push(pe.endpointPath)
  }
  return { listModels, general }
  ```
- `Select` 模板替换为 `<optgroup>` 结构（仅当对应组非空才渲染 optgroup）：
  ```html
  <Select v-model="fetchEndpointPath" ...>
    <option value="" disabled>{{ ... }}</option>
    <optgroup v-if="groupedFetchSources.listModels.length" label="列表模型端点">
      <option v-for="p in groupedFetchSources.listModels" :key="p" :value="p">{{ p }}</option>
    </optgroup>
    <optgroup v-if="groupedFetchSources.general.length" label="通用端点">
      <option v-for="p in groupedFetchSources.general" :key="p" :value="p">{{ p }}</option>
    </optgroup>
  </Select>
  ```
- 默认选中：`fetchEndpointPath.value = listModels[0] ?? general[0] ?? ''`，覆盖原来的 `providerEndpoints[0]`。
- 当两组都为空时，`Select` 的 placeholder option 文案改为 `无可用列表模型 / 通用端点`，按钮禁用（`!fetchEndpointPath`）。

5.4 `dashboard/src/ui/Select.vue`：检查是否原生支持嵌套 `<optgroup>`。如果当前 `<Select>` 仅 wrap 原生 `<select>` + `<slot>`，则零改动；否则补一个透传 slot 的实现。

## 6. 验证

6.1 `go build ./...` 通过；`mise run server` 启动无错（迁移落地）。

6.2 `pnpm --dir dashboard type-check` 与 `pnpm --dir dashboard lint` 通过；`pnpm --dir dashboard build` 成功。

6.3 手动验证：

- 旧端点行升级后 GET 应返回 `endpointType: "general"` 与原 `modelPath`。
- 新建端点切到 `openaiChatCompletions` 时，模型字段路径未填会被表单阻止提交；切到 `general` 时可留空，提交后 `modelPath` 为 `""`。
- `EndpointsView` 类型列正确渲染、不同类型的 Tag 颜色一致；`modelPath` 为空时单元格显示 `—`。
- 进入某个 provider 的模型面板：fetch 来源 `Select` 仅出现当前 provider 已绑定且类型为 `generalListModels` / `general` 的端点；分两组显示。
- 网关请求模型路由型端点（已配 `modelPath`）行为不变；调用 `modelPath` 为空的端点返回 400 + `model_not_found` + `endpoint has no model path configured`。
