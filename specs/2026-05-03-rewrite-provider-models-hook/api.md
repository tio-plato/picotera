# API：rewriteProviderModels + fetch-models 响应改造

## JS Hook：`rewriteProviderModels`

后台拉取 provider 模型列表时调用。Go 侧先按"默认聚合规则"算出新 `providerModels` 数组，然后把这个数组喂给 waterfall；脚本可以删/加/改条目，最终结果由前端做 diff，不直接落库。

### 注册

```js
picotera.hooks.rewriteProviderModels.tap(name, fn, priority?)
```

### 回调签名

```ts
function (ctx: Ctx, models: ProviderModelEntry[]) => ProviderModelEntry[] | undefined
```

waterfall 的输入 / 输出元素类型与 DB / API 里的 `ProviderModelEntry` 完全一致（`model` / `upstreamModelName` / `endpoints` / `priority` / `annotations` / `disabled` 全字段）。提案中写的 `upstreamModel` 是简写，实际 JSON 字段名是 `upstreamModelName`。

#### ctx 字段

| 字段 | 类型 | 说明 |
|---|---|---|
| `provider` | ProviderSummary | **只读**，DB 里的渠道信息（不含 credentials） |
| `endpointPath` | string | **只读**，本次拉取所用的 provider-endpoint 路径 |
| `upstreamResponse` | any | **只读**，上游 `/models` 响应的解析后 JSON。响应非 JSON 时为 `null` |

```ts
type ProviderSummary = {
  id: number,
  name: string,
  priority: number,
  providerModels: ProviderModelEntry[],
  annotations: Record<string, string>,
  disabled: boolean,
}

type ProviderModelEntry = {
  model: string,
  upstreamModelName?: string,
  endpoints?: string[],
  priority?: number,
  annotations?: Record<string, string>,
  disabled?: boolean,
}
```

#### waterfall 输入

`models` 是默认聚合规则跑完之后的数组：

- 老条目里 actualUpstream（`upstreamModelName || model`）仍出现在新拉取列表里的保留，所有字段原样带上。
- 老条目里 actualUpstream 不在新拉取列表里的删除。
- 新拉取列表里没出现在老 actualUpstream 集合里的，作为 `{model: name}` 追加（`upstreamModelName` 省略）。
- 已按 `(model, upstreamModelName)` 精确去重。

#### 返回语义

- 返回数组 → 替换 value，作为下一个 tap 的输入；最终值由 Go 接收。
- 返回 undefined / 非数组 → 沿用上一 tap 的值。
- 返回数组里某条目不是对象 / `model` 字段非 string / `model` 为空 → Go 侧丢弃该条目。
- 返回数组里某条目其他字段（`endpoints` / `priority` 等）类型不对 → 整次 hook 视作“沿用入参”，不报错。
- 返回数组里有 `(model, upstreamModelName)` 重复条目 → Go 侧再做一次去重。

### 示例

```js
picotera.hooks.rewriteProviderModels.tap('hide-preview', (ctx, models) => {
  return models.filter(m => !m.model.includes('-preview'))
})

picotera.hooks.rewriteProviderModels.tap('alias-new', (ctx, models) => {
  const oldActuals = new Set(ctx.provider.providerModels.map(e => e.upstreamModelName || e.model))
  return models.map(m => {
    if (oldActuals.has(m.upstreamModelName || m.model)) return m
    return { model: m.model + '-alias', upstreamModelName: m.model }
  })
})

// 脚本可以改写任意字段（比如给新增条目默认加一条标注）
picotera.hooks.rewriteProviderModels.tap('tag-new', (ctx, models) => {
  const oldKeys = new Set(
    ctx.provider.providerModels.map(e => `${e.model}\u0000${e.upstreamModelName || ''}`),
  )
  return models.map(m => {
    const k = `${m.model}\u0000${m.upstreamModelName || ''}`
    if (oldKeys.has(k)) return m
    return { ...m, annotations: { ...(m.annotations || {}), source: 'auto-fetched' } }
  })
})

picotera.hooks.rewriteProviderModels.tap('chat-only', (ctx, models) => {
  if (ctx.endpointPath !== '/v1/chat/completions') return
  return models.filter(m => /chat|gpt|claude/i.test(m.model))
})
```

## HTTP API 改造：`POST /api/picotera/provider-endpoints/fetch-models`

### Request（不变）

```json
{
  "providerId": 1,
  "endpointPath": "/v1/chat/completions"
}
```

### Response（变更）

```diff
- {
-   "providerId": 1,
-   "models": ["gpt-4o", "gpt-4o-mini"]
- }
+ {
+   "providerId": 1,
+   "providerModels": [
+     { "model": "gpt-4o" },
+     { "model": "my-mini", "upstreamModelName": "gpt-4o-mini", "priority": 5 }
+   ],
+   "removedModels": ["legacy-model"]
+ }
```

字段说明：

| 字段 | 类型 | 说明 |
|---|---|---|
| `providerId` | int | 不变 |
| `providerModels` | ProviderModelEntry[] | 默认聚合 + `rewriteProviderModels` hook 处理后的最终模型列表，完整 `ProviderModelEntry` 形式，已按 `(model, upstreamModelName)` 去重 |
| `removedModels` | string[] | 默认聚合规则判定应删除的"实际上游模型"名（老列表里有、上游里没有）。**仅供前端 UI 提示文案使用**，不反映脚本最终决策 |

`providerModels` 数组里的对象就是完整的 `ProviderModelEntry`：默认聚合保留下来的老条目原样带上 `endpoints` / `priority` / `annotations` / `disabled` 等字段；新增条目仅含 `model`；脚本可以任意改写所有字段，前端拿到后直接作为 `Row` 使用。

### 错误码

| 状态 | 触发条件 |
|---|---|
| 404 | provider / provider-endpoint / endpoint 任一不存在 |
| 422 | 上游响应无法解析出模型列表 |
| 502 | 上游连接失败 / 上游 4xx 5xx / jsx session 创建失败 |
| 503 | hook 超时（`jsx: hook timeout`） |
| 500 | 其他内部错误（marshal / DB 失败等） |
