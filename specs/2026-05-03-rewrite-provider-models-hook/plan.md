# Plan

## 1. `pkg/jsx/sdk.js`

在 `globalThis.picotera.hooks` 里加一个新的 Waterfall：

```diff
 hooks: {
   sortProviders: new Waterfall(),
   beforeRequest: new Waterfall(),
   rewriteRequest: new Waterfall(),
   rewriteModel: new Waterfall(),
+  rewriteProviderModels: new Waterfall(),
 },
```

## 2. `pkg/jsx/types.go`

新增类型（与 `contract.ProviderModelEntry` 的 JSON 形状完全一致，独立声明以避免 jsx 反向依赖 contract）：

```go
// ProviderModelEntry 镜像 contract.ProviderModelEntry 的 JSON 形状。
type ProviderModelEntry struct {
    Model             string            `json:"model"`
    UpstreamModelName string            `json:"upstreamModelName,omitempty"`
    Endpoints         []string          `json:"endpoints,omitempty"`
    Priority          int32             `json:"priority,omitempty"`
    Annotations       map[string]string `json:"annotations,omitempty"`
    Disabled          bool              `json:"disabled,omitempty"`
}

// ProviderSummary 是 rewriteProviderModels ctx.provider 的形状。
// 不包含 credentials；providerModels 是 DB 持久化的旧列表。
type ProviderSummary struct {
    ID             int32                `json:"id"`
    Name           string               `json:"name"`
    Priority       int32                `json:"priority"`
    ProviderModels []ProviderModelEntry `json:"providerModels"`
    Annotations    map[string]string    `json:"annotations"`
    Disabled       bool                 `json:"disabled"`
}

// RewriteProviderModelsInput 是 rewriteProviderModels waterfall 的 ctx。
type RewriteProviderModelsInput struct {
    Provider         ProviderSummary `json:"provider"`
    EndpointPath     string          `json:"endpointPath"`
    UpstreamResponse json.RawMessage `json:"upstreamResponse"`
}
```

## 3. `pkg/jsx/session.go`

新增方法：

```go
// RunRewriteProviderModelsHook 在后台拉取模型列表时调用。
// 入参 models 是默认聚合后的数组；返回 hook 处理后的最终数组。
// hook 没注册 / 返回 undefined / 返回非数组 → 直接返回入参。
func (s *Session) RunRewriteProviderModelsHook(in RewriteProviderModelsInput, models []ProviderModelEntry) ([]ProviderModelEntry, error)
```

实现采用与 `RunSortHook` 类似的 JS 表达式：

```js
(async () => {
  const ctx = %s;
  const initial = %s;
  const r = await picotera.hooks.rewriteProviderModels.runWaterfall(ctx, initial);
  if (typeof r === 'undefined' || r === null || !Array.isArray(r)) return null;
  return JSON.stringify(r);
})()
```

返回 `"null"` → 直接返回入参。否则 `json.Unmarshal` 进 `[]ProviderModelEntry`。Unmarshal 失败时也返回入参（脚本搞坏数据不破坏拉取流程）。

## 4. `pkg/contract/provider_endpoint.go`

改 `FetchModelsResponse`：

```diff
 type FetchModelsResponse struct {
     Body struct {
-        ProviderID int32    `json:"providerId"`
-        Models     []string `json:"models"`
+        ProviderID     int32                `json:"providerId"`
+        ProviderModels []ProviderModelEntry `json:"providerModels"`
+        RemovedModels  []string             `json:"removedModels"`
     }
 }
```

## 5. `pkg/server/handle_provider_endpoint.go`

`handleFetchModels` 在抽出 `models []string` 之后改写：

1. **解析上游响应为 `any`**：

   ```go
   var upstreamRaw any
   if jerr := json.Unmarshal(body, &upstreamRaw); jerr != nil {
       upstreamRaw = nil
   }
   ```

2. **默认聚合**：调用新增的 `aggregateProviderModels(oldList, upstreamNames)`，返回 `aggregated []contract.ProviderModelEntry` + `removed []string`。

3. **跑 hook**：

   ```go
   sess, err := s.jsxEngine.NewSession(ctx, fmt.Sprintf("fetch-models:%d:%d", input.Body.ProviderID, time.Now().UnixNano()))
   if err != nil {
       return nil, huma.Error502BadGateway("failed to load js hooks: " + err.Error())
   }
   defer sess.Close()

   providerSummary := jsx.ProviderSummary{
       ID:             provider.ID,
       Name:           provider.Name,
       Priority:       provider.Priority,
       ProviderModels: contractToJsxEntries(oldList),
       Annotations:    annotations,
       Disabled:       provider.Disabled,
   }
   upstreamRawJSON, _ := json.Marshal(upstreamRaw)   // nil → "null"

   processed, herr := sess.RunRewriteProviderModelsHook(jsx.RewriteProviderModelsInput{
       Provider:         providerSummary,
       EndpointPath:     input.Body.EndpointPath,
       UpstreamResponse: upstreamRawJSON,
   }, contractToJsxEntries(aggregated))
   if herr != nil {
       status := http.StatusBadGateway
       if errors.Is(herr, jsx.ErrHookTimeout) {
           status = http.StatusServiceUnavailable
       }
       return nil, huma.NewError(status, herr.Error())
   }
   ```

4. **后处理**：把 `processed []jsx.ProviderModelEntry` 转回 `[]contract.ProviderModelEntry` 后净化（trim `model`；丢弃 `model == ""` 条目）+ 再调用一次 `dedupProviderModelsByPair` → `final`。老条目的 `endpoints` / `priority` / `annotations` / `disabled` 由默认聚合阶段原样写进 hook 入参，hook 返回什么就是什么；不再做额外 merge。

5. 返回：

   ```go
   out.Body.ProviderID = input.Body.ProviderID
   out.Body.ProviderModels = final
   out.Body.RemovedModels = removed
   ```

新增辅助函数（同包）：

```go
// 默认聚合规则实现（见 design.md 的伪代码）。
func aggregateProviderModels(
    old []contract.ProviderModelEntry,
    upstreamNames []string,
) (aggregated []contract.ProviderModelEntry, removed []string)

// 按 (Model, UpstreamModelName) 严格去重，保留首次出现条目的所有字段。
func dedupProviderModelsByPair(in []contract.ProviderModelEntry) []contract.ProviderModelEntry

// JSON tag 完全一致 → 用 json.Marshal + json.Unmarshal 实现两边转换，避免手写字段映射。
func contractToJsxEntries(in []contract.ProviderModelEntry) []jsx.ProviderModelEntry
func jsxToContractEntries(in []jsx.ProviderModelEntry) []contract.ProviderModelEntry
```

## 6. 接入 Server

`Server` 已经有 `jsxEngine` 字段；`handleFetchModels` 现在是值方法，签名不变。`provider.ProviderModels` 已经是 `[]byte` JSON，需要 unmarshal 到 `[]contract.ProviderModelEntry` 才能跑聚合。

## 7. Frontend：`dashboard/src/api.d.ts`

跑 `mise run openapi` + `pnpm --dir dashboard build` 让 openapi-typescript 重新生成 `FetchModelsResponse` 类型。新字段 `providerModels` / `removedModels` 自动出现。

## 8. Frontend：`dashboard/src/components/ProviderModelsPanel.vue`

### 8.1 类型 / 状态扩展

`fetchSummary` 改成：

```ts
type MissingRow = { uid: number; modelName: string; upstreamModelName: string }

const fetchSummary = ref<{
  added: number
  missing: MissingRow[]
  removedHint: string[]
} | null>(null)

const pendingDeletions = ref<Record<number, boolean>>({})    // key = uid
```

### 8.2 helper 改造

`emptyRow` / `entryToRow` 保持不动——服务端返回的是完整 `ProviderModelEntry`，直接走现有 `entryToRow(entry)` 的路径。

新增 `pairKey(model, upstream)` 工具：

```ts
function pairKey(model: string, upstream: string): string {
  return `${model}\u0000${upstream ?? ''}`
}
```

`rowsToList` 末尾按 `(modelName, upstreamModelName)` 去重，保留首次出现。

### 8.3 `fetchFromUpstream` 重写

```ts
async function fetchFromUpstream() {
  if (!fetchEndpointPath.value) { error.value = '请选择一个端点作为来源'; return }
  fetching.value = true
  error.value = ''
  fetchSummary.value = null
  pendingDeletions.value = {}
  const { data, error: err } = await api.POST('/api/picotera/provider-endpoints/fetch-models', {
    body: { providerId: props.providerId, endpointPath: fetchEndpointPath.value },
  })
  fetching.value = false
  if (err) { error.value = err.message ?? '拉取模型失败'; return }

  const serverList = (data.providerModels ?? []) as ProviderModelEntry[]
  const removedHint = (data.removedModels ?? []) as string[]

  const rowPairs = new Set(rows.value.map(r => pairKey(r.modelName, r.upstreamModelName)))
  const serverPairs = new Set(serverList.map(e => pairKey(e.model, e.upstreamModelName ?? '')))

  let added = 0
  for (const entry of serverList) {
    const key = pairKey(entry.model, entry.upstreamModelName ?? '')
    if (!rowPairs.has(key)) {
      rows.value.push(entryToRow(entry))
      rowPairs.add(key)
      added++
    }
  }

  rows.value.sort((a, b) => {
    const cmp = a.modelName.localeCompare(b.modelName)
    if (cmp !== 0) return cmp
    return b.priority - a.priority
  })

  const missing: MissingRow[] = rows.value
    .filter(r => !serverPairs.has(pairKey(r.modelName, r.upstreamModelName)))
    .map(r => ({ uid: r.uid, modelName: r.modelName, upstreamModelName: r.upstreamModelName }))

  fetchSummary.value = { added, missing, removedHint }
}
```

### 8.4 `applyDeletions` 改 uid 索引

```ts
function applyDeletions() {
  const toDelete = new Set(
    Object.entries(pendingDeletions.value)
      .filter(([, v]) => v)
      .map(([k]) => Number(k))
  )
  if (!toDelete.size) {
    fetchSummary.value = null
    pendingDeletions.value = {}
    return
  }
  rows.value = rows.value.filter(r => !toDelete.has(r.uid))
  fetchSummary.value = null
  pendingDeletions.value = {}
}
```

### 8.5 模板：missing 区域

把现有 `v-for="name in fetchSummary.missing"` 改成 `v-for="row in fetchSummary.missing" :key="row.uid"`，label 文案：`{{ row.modelName }}<span v-if="row.upstreamModelName"> → {{ row.upstreamModelName }}</span>`，`v-model="pendingDeletions[row.uid]"`，`:id="`del-${row.uid}`"`。

可选：如果 `removedHint.length`，在 summary 卡片顶部多一行小字说明"默认规则建议删除：…"。

## 9. `pkg/jsx/engine_test.go`

新增四个用例：

- `TestSession_Hooks_RewriteProviderModels_Passthrough`：脚本不 return，期望返回入参 `[]ProviderModelEntry` 不变。
- `TestSession_Hooks_RewriteProviderModels_Replace`：脚本 `return [{model: 'a'}, {model: 'b', upstreamModelName: 'B', priority: 7, annotations: {x: 'y'}}]`，期望按数组顺序返回，所有字段保留。
- `TestSession_Hooks_RewriteProviderModels_NonArray`：脚本 `return 42`，期望 fallback 到入参。
- `TestSession_Hooks_RewriteProviderModels_FieldTypeMismatch`：脚本 `return [{model: 'a', priority: 'bad'}]`，期望 unmarshal 失败时 fallback 到入参（不报错）。

## 10. 验证

- `go build ./...`
- `go test ./pkg/jsx/...`
- `mise run openapi`（响应字段变了，需要更新 openapi.yaml；提交里包含）
- `pnpm --dir dashboard build`（让 `src/api.d.ts` 重新生成）
- `pnpm --dir dashboard lint`
- 手动 smoke：
  1. `docker compose up -d` → `mise run server` → `mise run web`。
  2. 配一对 provider + provider-endpoint，老 `providerModels` 留两条：`{model: 'foo'}`、`{model: 'my-mini', upstreamModelName: 'mini'}`。
  3. 模拟上游返回 `{data: [{id: 'foo'}, {id: 'gpt-4o'}]}`（用一个临时 mock URL 或现成的 OpenAI-style 端点）。
  4. 期望响应：`providerModels` 含原始 `foo` 条目（`endpoints` 等保留）+ 新增 `{model: 'gpt-4o'}`，`removedModels: ['mini']`。前端弹窗提示 `my-mini → mini` 是否删除。
  5. 在 `/api/picotera/scripts` 注入：
     ```js
     picotera.hooks.rewriteProviderModels.tap('drop-mini', (ctx, models) =>
       models.filter(m => !m.model.includes('gpt-4o')))
     ```
     重新拉取，期望 `gpt-4o` 不在响应里；`removedModels` 仍是 `['mini']`（不受 hook 影响）。

## 11. 提交分块

1. `feat(jsx): add rewriteProviderModels waterfall and ProviderModelEntry mirror type`
2. `feat(server): aggregate provider models on fetch and run rewriteProviderModels hook`
3. `feat(dashboard): consume providerModels response and pair-based diff in ProviderModelsPanel`
4. `chore(openapi): regenerate spec for fetch-models response shape`
5. `test(jsx): cover rewriteProviderModels`
