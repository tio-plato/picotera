# Design

## 背景

`provider.provider_models` 当前是一个 JSONB 对象：键为本地模型名，值为该 provider 在该模型下的上游配置。

```jsonc
{
  "claude-sonnet-4-6": { "upstreamModelName": "anthropic/claude-sonnet-4", "priority": 0, "endpoints": ["/v1/messages"], "annotations": {}, "disabled": false },
  "gpt-5":            { "upstreamModelName": "gpt-5", "priority": 10 }
}
```

由于以本地模型名为键，**同一个 provider + 同一个本地模型** 只能配置一条上游映射。当用户希望对同一个内部模型挂多条上游（不同的 `upstreamModelName`、不同的 endpoints 子集、不同的优先级用作 fallback）时，该结构无法表达。

## 目标

把 `provider_models` 改造为**数组**，数组元素携带原有所有字段，并新增 `model` 字段保存本地模型名；允许同一 `model` 出现多次。

```jsonc
[
  { "model": "claude-sonnet-4-6", "upstreamModelName": "anthropic/claude-sonnet-4", "priority": 10, "endpoints": ["/v1/messages"] },
  { "model": "claude-sonnet-4-6", "upstreamModelName": "claude-sonnet-4-6-20250929", "priority": 0 },
  { "model": "gpt-5", "upstreamModelName": "gpt-5", "priority": 10 }
]
```

## 数据模型与迁移

### 列结构

`provider.provider_models` 列类型保持 JSONB；存储形态由对象改为数组。

### 索引

继续使用 GIN `jsonb_path_ops` 索引（对数组同样有效），用于 `@>` 包含查询：
查找 `model = X` 的 provider 时，`provider_models @> '[{"model":"X"}]'` 会命中索引。

### 迁移 (008_provider_models_array.sql)

**Up**：原地转换全部已有数据，不丢数据。

```sql
UPDATE provider
   SET provider_models = COALESCE(
     (
       SELECT jsonb_agg(jsonb_build_object('model', k) || v)
       FROM jsonb_each(provider_models) AS x(k, v)
       WHERE jsonb_typeof(v) = 'object'
     ),
     '[]'::jsonb
   )
 WHERE jsonb_typeof(provider_models) = 'object';
```

`jsonb_build_object('model', k) || v` 把模型名作为 `model` 字段并入原条目；如果原条目自带 `model` 字段（不应该存在），并集运算右侧 `v` 会覆盖左侧，因此显式以 `v || jsonb_build_object('model', k)` 顺序确保 key 优先用 jsonb_each 的键：

```sql
SELECT jsonb_agg(v || jsonb_build_object('model', k))
```

最终采用后者（key 来自 record 键，最权威）。

**Down**：把数组聚回对象（key 取数组元素的 `model` 字段；当同一 model 出现多次时，**保留最后一条**，这是 `jsonb_object_agg` 的固有语义，文档化为已知精度损失）。

```sql
UPDATE provider
   SET provider_models = COALESCE(
     (
       SELECT jsonb_object_agg(
                elem ->> 'model',
                elem - 'model'
              )
         FROM jsonb_array_elements(provider_models) AS elem
         WHERE elem ? 'model'
     ),
     '{}'::jsonb
   )
 WHERE jsonb_typeof(provider_models) = 'array';
```

GIN 索引由 006 创建、且其表达式与列相同，无需重建。

## 路由查询

`db/queries/routing.sql` 中 `GetProvidersByEndpointAndModel` 改为展开数组并按 `model` 过滤：

```sql
SELECT
  sqlc.arg('model_name')::text AS model_name,
  p.id AS provider_id,
  pe.endpoint_path,
  COALESCE(elem ->> 'upstreamModelName', '')::text AS upstream_model_name,
  COALESCE((elem ->> 'priority')::int, 0)::int AS priority,
  (COALESCE(elem -> 'annotations', '{}'::jsonb))::jsonb AS annotations,
  p.name AS provider_name,
  p.credentials AS provider_credentials,
  p.priority AS provider_priority,
  pe.upstream_url,
  p.annotations AS provider_annotations
FROM provider AS p
JOIN provider_endpoint AS pe ON pe.provider_id = p.id
JOIN model AS m ON m.name = sqlc.arg('model_name')::text
CROSS JOIN LATERAL jsonb_array_elements(p.provider_models) AS elem
WHERE pe.endpoint_path = sqlc.arg('endpoint_path')::text
  AND p.provider_models @> jsonb_build_array(jsonb_build_object('model', sqlc.arg('model_name')::text))
  AND elem ->> 'model' = sqlc.arg('model_name')::text
  AND p.disabled = FALSE
  AND m.disabled = FALSE
  AND COALESCE((elem ->> 'disabled')::boolean, false) = false
  AND (
    elem -> 'endpoints' IS NULL
    OR jsonb_typeof(elem -> 'endpoints') <> 'array'
    OR jsonb_array_length(elem -> 'endpoints') = 0
    OR elem -> 'endpoints' @> to_jsonb(ARRAY[pe.endpoint_path])
  );
```

要点：

- `@> jsonb_build_array(jsonb_build_object('model', $1))` 命中 GIN，先快速筛掉无关 provider；再用 `jsonb_array_elements` 展开拿到具体 entry。
- 输出列名与字段类型不变，调用方（gateway）零改动；同一 (provider, model) 多 entry 时返回多行，gateway 已有的 sort + retry 逻辑天然处理。
- 排序按 `provider_priority + entry_priority` 降序（保留现有逻辑）；并列顺序未定义，业务上以优先级为准。

## Go contract

```go
type ProviderModelEntry struct {
    Model             string            `json:"model"`
    UpstreamModelName string            `json:"upstreamModelName,omitempty"`
    Endpoints         []string          `json:"endpoints,omitempty"`
    Priority          int32             `json:"priority,omitempty"`
    Annotations       map[string]string `json:"annotations,omitempty"`
    Disabled          bool              `json:"disabled,omitempty"`
}

type ProviderView struct {
    // ...
    ProviderModels []ProviderModelEntry `json:"providerModels"`
    // ...
}
```

`CreateProviderRequest.Body.ProviderModels` / `UpsertProviderRequest.Body.ProviderModels` 同步换成切片。
`ToProviderView` / `FromProviderView` 把 jsonb 与切片互转；空切片 marshal 为 `[]`，解 nil 数据库值时保护为空切片。

handler `handleCreateProvider` / `handleUpsertProvider` 把 `nil` 切片在落库前规范化为 `[]ProviderModelEntry{}`，避免写出 `null`。

## 前端类型与组件

### 类型

`openapi-typescript` 重新生成后，`ProviderView.providerModels` 为 `ProviderModelEntry[]`，`ProviderModelEntry` 增加 `model: string`。

### `ProviderModelsPanel.vue`

- `Row` 字段不变，`modelName` 直接对应 entry.model；新增行/编辑行允许同名，不再做 `已存在` 校验。
- `rowsFromProvider`：直接 `provider.providerModels.map(entryToRow)`；保持稳定排序（按 `model` 字典序，再按 `priority` 降序，确保保存后视觉一致）。
- `rowsToObject` 重命名为 `rowsToList`，输出 `ProviderModelEntry[]`：丢弃 `model` 为空的行；其他字段按现有 omit-empty 规则保留。
- 「从上游拉取」逻辑：
  - 「本地缺失上游模型」按 entry.model 去重判断；
  - 「上游缺失本地模型」按 entry.model 去重展示；勾选删除时删掉所有同名行（明确告知用户：会删除该 model 下的全部上游配置）。

### `ModelsView.vue`

`upstreamIndex` 由 `Object.entries(provider.providerModels)` 改为遍历数组：每条 entry 产出一条 `Upstream`，按 `entry.model` 聚合。`upstreamSet` / orphan 检测同步用 entry.model。

### `ModelUpstreamsPanel.vue`

`Upstream.providerId` 不再唯一，改用 `(providerId, index)` 复合键 `key="${u.providerId}:${i}"`。

### `ProvidersView.vue`

`modelNames(p)` 改为 `Array.from(new Set(p.providerModels.map((e) => e.model)))`，去重展示。

### `ProviderForm.vue`

新建/编辑 provider 时，默认 `providerModels: []`；编辑时透传现有数组，不在此处修改。

## 兼容性与回退

- 旧 JSONB 对象数据通过 Up 迁移转换为数组；不丢数据。
- 回退时 Down 把数组按 `model` 聚回对象；同一 model 多 entry 仅保留最后一条，是预期的精度损失。
- 没有外部调用方依赖 `providerModels` 的对象形态（dashboard 是唯一前端，OpenAPI 由本仓库重新生成）。
- JS hook 看到的 `mpe` shape 不变（仍由 routing 行的列直接构造），无需调整。

## 不引入第三方依赖

无新依赖。
