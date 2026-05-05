# API — Merged annotations in hook ctx

## Go：`pkg/annotations`

```go
package annotations

func Merge(layers ...map[string]string) map[string]string
func Decode(raw []byte) (map[string]string, error)
```

- `Merge` 永远返回非 nil map（即使所有 layer 都为 nil），这样 JSON 编码出 `{}` 而非 `null`。
- `Decode(nil)` / `Decode([]byte("null"))` / `Decode([]byte("{}"))` 都返回空 map + nil error。
- 非 object 顶层 JSON（数组、字符串、数字…）返回 error。
- 非 string 值在解码时通过 `fmt.Sprint` 字符串化，与 dashboard 行为一致。

## REST API（`pkg/contract/model.go`）

`ModelView` 增加 `annotations`：

```jsonc
{
  "name": "claude-sonnet-4-6",
  "title": "...",
  "developer": "anthropic",
  "series": "claude",
  "disabled": false,
  "pricing": { "...": "..." },
  "annotations": {
    "ah.outbound.type": "openrouter",
    "ah.outbound.config": "{\"baseUrl\":\"...\"}"
  }
}
```

Operations：
- `GET /api/picotera/models` — body 中每个 `ModelView` 携带 `annotations`。
- `GET /api/picotera/models/{name}` — 同上。
- `PUT /api/picotera/models` — body 接受 `annotations`，省略时按 `{}` 写入。
- `POST /api/picotera/models/delete` — 不变。

写入 / 上行端：`PutModel` handler 把 `Annotations` marshal 成 JSONB 后传给 `UpsertModel`。

## SQL 形状变更（`db/queries/`）

- `model.sql`：`GetModelByName` / `GetModels` / `UpsertModel` 每行多一个 `annotations` 字段；`UpsertModel` 多一个绑定参数（最后一位）。
- `routing.sql`：两个 routing 查询新增列 `m.annotations AS model_annotations`，类型 JSONB。

## JS-visible ctx（`pkg/jsx/types.go`）

新增 / 调整字段：

```ts
type ModelSummary = {
  name: string;
  annotations: Record<string, string>;
};

type ApiKeySummary = { /* unchanged */ };

type ProviderSummary = {
  id: number;
  name: string;
  priority: number;
  // CHANGED: was json.RawMessage; now decoded map.
  annotations: Record<string, string>;
  providerModels?: ProviderModelEntry[];
  disabled: boolean;
};

type CandidateMPE = {
  modelName: string;
  providerId: number;
  endpointPath: string;
  upstreamModelName: string;
  priority: number;
  // CHANGED: was json.RawMessage; now decoded map.
  annotations: Record<string, string>;
};

type Candidate = {
  provider: ProviderSummary;
  mpe: CandidateMPE;
  // NEW: merged map (model < provider < mpe entry, later wins).
  annotations: Record<string, string>;
};
```

每个 hook ctx 顶层都加一个 `annotations` 字段（合并优先级：`model < provider < entry < apiKey`，越往后越优先）：

| Hook                    | `ctx.annotations` 含义                                       |
| ----------------------- | ------------------------------------------------------------ |
| `rewriteModel`          | model + apiKey（此时 candidate 还未解析）                    |
| `sortProviders`         | model + apiKey；每个 candidate 自带各自合并好的 `annotations` |
| `rewriteProviderModels` | model + provider + apiKey（此时没有具体 entry；fetch-models 流程下 model 层为 `{}`） |
| `beforeRequest`         | 该 candidate 的合并结果（model + provider + entry + apiKey） |
| `rewriteRequest`        | 同 `beforeRequest`                                            |

ModelSummary 暴露在 `SortInput.model` / `BeforeRequestInput.model` / `RewriteInput.model` / `RewriteProviderModelsInput`（如适用）；当前这些字段是 `any` / `nil`，本次填成 `ModelSummary` 实例。

api key 的 annotations 已经在 `ApiKeySummary.annotations`（`map[string]string`）暴露过，本次不改它的形态，只是把它纳入合并管线。
