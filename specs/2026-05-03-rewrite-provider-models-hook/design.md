# Design

## 现状

`POST /api/picotera/provider-endpoints/fetch-models` 的处理逻辑（`pkg/server/handle_provider_endpoint.go::handleFetchModels`）：

1. 取 provider、provider-endpoint、endpoint 三条 row。
2. 用 provider 凭据 + endpoint resolver 拼出请求头，向 `pe.UpstreamUrl` 发 GET。
3. 把响应 body 喂给 `parseModelsResponse` 抽出 `[]string`（兼容 `data[].id` / `data[].name` / 顶层数组的 `id` / `name`）。
4. 直接把 `[]string` 返给前端。

前端 `dashboard/src/components/ProviderModelsPanel.vue::fetchFromUpstream` 拿到 `string[]` 后：

- 在本地 `rows` 内对比模型名（按 `row.modelName`），新增上游有但本地无的；
- 用本地有但上游没有的名字组成 `missing[]` 弹窗，让用户勾选删除。

完全不接入 jsx，没有 hook 介入聚合逻辑。

## 目标

- 后端在拉到上游原始响应 + 解析出模型名之后，构造一个"老 vs 新"的默认聚合结果 `[]ProviderModelEntry`（完整 `model` / `upstreamModelName` / `endpoints` / `priority` / `annotations` / `disabled` 字段）。
- 起一个 jsx Session 调用新 Waterfall hook `rewriteProviderModels`，让脚本对聚合结果再加工一次。
- HTTP 响应改成返回 `[]ProviderModelEntry`（完整类型：老条目保留全部字段，新增条目仅含 `model`）。前端拿这个结果直接调 `entryToRow` 生成新 `Row`，并对现有 `rows` 做 diff。

## Hook：`rewriteProviderModels`

新增 Waterfall，与 `rewriteModel` / `rewriteRequest` 命名风格一致。

### ctx 形状

```ts
type RewriteProviderModelsCtx = {
  provider: {
    id: number,
    name: string,
    priority: number,
    providerModels: ProviderModelEntry[],   // DB 里持久化的旧列表（聚合前）
    annotations: Record<string, string>,
    disabled: boolean,
  },
  endpointPath: string,                      // 本次拉取所用的 provider-endpoint 路径
  upstreamResponse: any,                     // 上游原始响应解析后的 JSON（透传）
}
```

`provider` 子集复用 `ProviderView` 的字段语义；`credentials` 出于安全考虑不暴露给脚本。

### waterfall value

Waterfall 入参 / 出参与 DB / API 的 `ProviderModelEntry` 类型完全一致，含 `model` / `upstreamModelName` / `endpoints` / `priority` / `annotations` / `disabled` 全部字段。提案里写的 `upstreamModel` 是简写，实际 JSON 字段名是 `upstreamModelName`（与 `pkg/contract/provider.go` 现有约定对齐）。

```ts
type ProviderModelEntry = {
  model: string,
  upstreamModelName?: string,
  endpoints?: string[],
  priority?: number,
  annotations?: Record<string, string>,
  disabled?: boolean,
}

picotera.hooks.rewriteProviderModels.tap(name, fn)
// fn: (ctx, models: ProviderModelEntry[]) => ProviderModelEntry[] | undefined
```

- 入参 `models` 是默认聚合规则跑完之后的数组（已去重）。老条目命中保留分支时所有字段原样保留，新增条目仅含 `model`。
- return 数组 → 替换成新数组；return undefined / 非数组 → 沿用上一 tap 的值。
- 最终值由 Go 拿到，再做一次去重 + 字段净化（trim `model`、丢弃空 `model` 条目，其他字段透传）。

## 默认聚合规则（Go 侧）

输入：

- `old []ProviderModelEntry`：DB `provider.providerModels`。
- `upstreamNames []string`：从上游响应抽出的模型名列表（沿用现有 `parseModelsResponse`）。

伪代码：

```
upstreamSet := set(upstreamNames)
result := []
seenActual := map[string]bool

// 1) 老列表里 actualUpstream 仍在上游里的保留；不在的丢掉。
for e in old:
    actual := e.upstreamModelName or e.model
    if actual in upstreamSet:
        result.append(e)
        seenActual[actual] = true

// 2) 上游里没出现在 seenActual 的，作为新条目追加 {model: name}（省略 upstreamModelName）。
for name in upstreamNames:
    if name not in seenActual:
        result.append({model: name})

// 3) 按 (model, upstreamModelName) 精确去重，保留首次出现。
result = dedupByPair(result)
```

注意：

- "实际上游模型"用 `upstreamModelName` 非空时取 `upstreamModelName`，否则取 `model`，与现有 `gateway` 路径上 `candidateUpstreamModel` 的语义一致。
- 老列表里每条的 `endpoints` / `priority` / `annotations` / `disabled` / 自定义 `model` 名都按原样保留，仅"是否在新列表里"决定保留 / 删除。
- 新增条目仅写 `model` 字段（`upstreamModelName` 省略），前端把它映射成 `Row` 时 `upstreamModelName` 默认为空字符串。

## 去重

`dedupByPair([])`：以 `(entry.Model, entry.UpstreamModelName)` 字符串对作为 key，保留首次出现，丢弃后续重复。其他字段（`endpoints` / `priority` / `annotations` / `disabled`）不参与 key，重复时只保留首条。

去重在三个地方触发：

1. **后端聚合后**（hook 入参之前）。
2. **后端 hook 返回后**（防止脚本制造重复）。同步去除 `model == ""` 的条目。
3. **前端**：`fetchFromUpstream` 把响应合并进 `rows` 时，对合并后的列表按 `(modelName, upstreamModelName)` 去重；`rowsToList` 保存前再做一次。

## HTTP API 变化

`POST /api/picotera/provider-endpoints/fetch-models` 响应改造：

```diff
 type FetchModelsResponse struct {
     Body struct {
-        ProviderID int32    `json:"providerId"`
-        Models     []string `json:"models"`
+        ProviderID     int32                `json:"providerId"`
+        ProviderModels []ProviderModelEntry `json:"providerModels"`
     }
 }
```

请求结构不变。前端是唯一消费方，跟着改即可。

为了让前端容易给出"缺失提示"，响应再额外带一个 `removedModels []string`，列出**默认聚合规则**判定为应删除的"实际上游模型"名（即老列表里出现、上游列表里没有的 actualUpstream）。这是给前端 UI 的便捷字段；不参与 hook 处理。

```go
type FetchModelsResponse struct {
    Body struct {
        ProviderID     int32                `json:"providerId"`
        ProviderModels []ProviderModelEntry `json:"providerModels"`
        RemovedModels  []string             `json:"removedModels"`
    }
}
```

`removedModels` 是**默认规则**的删除集，不受脚本影响——脚本可能加回这些条目。前端按"脚本最终结果 vs 当前 rows"重新 diff，`removedModels` 仅供 UI 展示提示文案使用。

## 流程

```
handleFetchModels:
  1. 取 provider / provider-endpoint / endpoint
  2. fetch upstream → raw bytes
  3. parseModelsResponse(raw) → upstreamNames []string
  4. 把 raw bytes 再 json.Unmarshal 成 any，作为 hook ctx.upstreamResponse
  5. defaultAggregate(provider.providerModels, upstreamNames) → models, removedNames
  6. 起 jsx session：jsxEngine.NewSession(ctx, "fetch-models:<providerID>:<ts>")
  7. session.RunRewriteProviderModelsHook(ctx, models) → models'
  8. dedupByPair + 净化 → finalModels
  9. 返回 { providerId, providerModels: finalModels, removedModels }
```

session 创建失败 / hook 抛错 / hook 超时 → 503/502 + 包含错误信息的 huma error；不落 meta request（meta request 只用于 gateway 链路，这里是后台管理操作）。脚本执行的日志走现有 `appendLog` 通道，但本接口暂不上报 artifacts——日志只能从 server stdout 看到。如果需要把脚本日志透给前端，单独再做。

## Frontend diff（`ProviderModelsPanel.vue`）

`fetchFromUpstream` 改写为：

```
const { data } = await api.POST('/api/picotera/provider-endpoints/fetch-models', { body: ... })
const serverList = data.providerModels        // ProviderModelEntry[]
const serverRemoved = data.removedModels       // string[]，仅展示用

// pair = `${model}\u0000${upstreamModelName ?? ''}`
const rowPairs = new Set(rows.map(r => pair(r.modelName, r.upstreamModelName)))
const serverPairs = new Set(serverList.map(e => pair(e.model, e.upstreamModelName ?? '')))

// 1) 服务端有、本地无 → 自动追加为新 row。entryToRow 直接复用现有逻辑
//    （已经处理 endpoints/priority/annotations/disabled 全字段）。
for entry in serverList:
    if pair(entry.model, entry.upstreamModelName ?? '') not in rowPairs:
        rows.push(entryToRow(entry))

// 2) 本地有、服务端无 → 收集到 fetchSummary.missing，prompt 用户勾选删除。
//    展示文案用 row.modelName（带可选 → row.upstreamModelName 显示）。
missing := rows.filter(r => !serverPairs.has(pair(r.modelName, r.upstreamModelName)))

// 3) 合并后再按 (modelName, upstreamModelName) 去重。
```

服务端返回的 `ProviderModelEntry` 直接喂给现有 `entryToRow`，所有字段（`endpoints` / `priority` / `annotations` / `disabled`）一并带过来；脚本对这些字段的改写也会反映到前端 `Row`。`rowsToList` 内再做一次去重保险。

`fetchSummary` 结构变成：

```ts
{
  added: number,
  missing: { uid: number, modelName: string, upstreamModelName: string }[],
  removedHint: string[],   // 来自 serverRemoved，仅展示
}
```

`pendingDeletions` 改为按 `uid` 索引（替代原来的按 modelName，避免 modelName 重复时的歧义）。

## 失败模式

| 情况 | 行为 |
|---|---|
| upstream 5xx / 解析失败 | 维持现有 huma error 路径（4xx/5xx）|
| jsx session 创建失败（脚本编译挂） | 502 + `failed to load js hooks`，前端弹错 |
| hook 超时 | 503 + `jsx: hook timeout` |
| hook 返回非数组 | Go 侧视作"沿用入参"，不报错 |
| hook 返回数组里有非对象 / `model` 字段非 string / `model` 为空 | 该条目丢弃，不报错 |
| hook 返回数组里某条目其他字段（`endpoints` / `priority` 等）类型不对 | json.Unmarshal 失败 → 整次 hook 视作 "沿用入参"，不报错 |
| upstream 响应非 JSON | hook ctx.upstreamResponse 为 null，仍正常执行；脚本可自行兜底 |

## 不涉及

- DB schema（`provider.providerModels` JSONB 字段不动）。
- gateway 链路（`handle_gateway.go` 不动）。
- 现有四个 hook（sortProviders / beforeRequest / rewriteRequest / rewriteModel）。
- `parseModelsResponse` 的解析策略——继续沿用现有 `data[].id` / `data[].name` 优先级。
- 配置项（沿用 `JSHookTimeout` / `JSMemoryLimit`）。
