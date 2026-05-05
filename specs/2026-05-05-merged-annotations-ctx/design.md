# Design — Merged annotations in hook ctx

## 目标

1. 提取「model + provider + provider-model entry + api key」四层 annotations 合并逻辑成一个独立工具，整个 codebase 只在一处实现。
2. 让所有 JS hook 都能从 ctx 直接拿到合并后的 `annotations`，无需在脚本里再拼一次。
3. 让后续 TODO #2（`ah.outbound.type` / `ah.outbound.config` 选择 outbound transformer）可以直接消费同一份合并结果，不必再走一遍解析。

合并优先级：`model < provider < provider-model entry < api key`，越往后越优先（api key 最后落，覆盖前三层）。

## 模型层（DB & contract）

`model` 表当前没有 `annotations` 列，需要补齐：

- 新增 migration `013_model_annotations.sql`：`ALTER TABLE model ADD COLUMN annotations JSONB NOT NULL DEFAULT '{}'::jsonb;`
- `db/queries/model.sql` 的 `GetModelByName`、`GetModels`、`UpsertModel` 都加上 `annotations` 字段。
- `db/queries/routing.sql` 两个 routing 查询（`GetProvidersByEndpointAndModel`、`GetProvidersByEndpointTypesAndModel`）都加上 `m.annotations AS model_annotations`，避免每个候选都额外发一次 `GetModelByName`。
- 跑 `sqlc generate` 重新生成 `pkg/db/`。
- `pkg/contract/model.go` 的 `ModelView` 加 `Annotations map[string]string`，`ToModelView` / 上行写入端补齐 marshal/unmarshal。
- `dashboard/src/components/ModelForm.vue` + `ModelsView` 复用现有 `AnnotationsEditor` 组件挂上去（与 `ProviderForm` 的方式一致）。生成 `openapi.yaml` 与 dashboard 类型。

## 合并工具：`pkg/annotations`

annotations 全 codebase 已经统一为 `map[string]string`（dashboard、provider/MPE entry、api_key 一致）。新建独立小包，避免 server/jsx/llmbridge 之间形成循环依赖：

```go
package annotations

// Merge returns a new map produced by overlaying each layer in order.
// Later layers win on key conflict. Nil layers are skipped. The result
// is never nil; an empty merge yields an empty (allocated) map so JSON
// encoding produces "{}" instead of "null".
func Merge(layers ...map[string]string) map[string]string

// Decode parses JSONB bytes into map[string]string. nil/empty/"null"/"{}"
// all yield an empty map. Non-object JSON returns an error. Non-string
// values are coerced via fmt.Sprint to keep the surface flat (matches
// how dashboard's AnnotationsEditor stores everything as strings).
func Decode(raw []byte) (map[string]string, error)
```

调用约定：

- 路由查询返回的 `model_annotations` / `provider_annotations` 是 `[]byte`，先 `Decode` 再 `Merge`。
- `ProviderModelEntry.Annotations` 已经是 `map[string]string`，直接喂给 `Merge`。
- api key 的 annotations 在 `authenticateClient` 之后已经通过 `apiKeySummaryFromRow` 解码为 `map[string]string`，复用即可。
- 优先级 `model < provider < entry < apiKey`，调用形如 `Merge(modelAnno, providerAnno, entryAnno, apiKeyAnno)`。

## JS-visible 形态（`pkg/jsx/types.go`）

每个 hook 的 ctx 都加上一个顶层 `annotations` 字段，**已经合并完毕**，JS 端 `ctx.annotations["ah.outbound.type"]` 直接拿。

- `Candidate`：增加 `Annotations map[string]string` 字段，承载该候选项四层合并的结果（model + provider + entry + apiKey）。
- `SortInput`：每个 candidate 自身带 `annotations`，顶层不再单独加（model 这一层在 `model` 字段里能拿到，api key 一层在 `apiKey.annotations` 里能拿到）。
- `BeforeRequestInput`、`RewriteInput`：增加顶层 `Annotations map[string]string`，等于所选 candidate 的 `Annotations`，方便脚本不用再去 candidate 上找。
- `RewriteModelInput`：增加 `Annotations map[string]string`，此时 MPE / provider 还没解析，只承载 model + apiKey 两层。
- `RewriteProviderModelsInput`：增加 `Annotations map[string]string`，承载 model + provider + apiKey 三层（没有具体 entry）。`fetch-models` 流程没有 model 上下文，传 model 层 `{}`。
- `SortInput.Model` / 各 input 的 `Model` 当前是 `any` / 永远是 `nil`，本次顺手填成一个 `ModelSummary{ Name, Annotations }`，让 JS 也能单独看 model 这一层（其它层 candidate / apiKey 已经带）。

## Server 层接线（`pkg/server`）

新建 `pkg/server/annotations.go`（或塞进 `gateway_helpers.go`）封装两件事：

```go
// candidateAnnotationsBuilder pins the request-scoped layers (model and
// apiKey) so the per-candidate loop only needs to feed in the (provider,
// mpe entry) pair. apiKey is the highest-priority layer and applies to
// every candidate produced under this request.
type candidateAnnotationsBuilder struct {
    modelAnno   map[string]string
    apiKeyAnno  map[string]string
}

func newCandidateAnnotationsBuilder(modelAnnoRaw []byte, apiKeyAnno map[string]string) (*candidateAnnotationsBuilder, error)

// merge returns the per-candidate merged annotations for the given
// (provider, mpe entry) layers in addition to the pinned model + apiKey
// layers. Order: model < provider < entry < apiKey.
func (b *candidateAnnotationsBuilder) merge(providerAnnotationsRaw []byte, entryAnnotations map[string]string) (
    merged, providerDecoded map[string]string,
)
```

`handle_gateway.go` 与 `handle_unified_gateway.go` 在构建 candidate 列表时调用这个 builder：

- `model_annotations` 来自路由查询新增的列。
- `apiKeyAnno` 来自 `apiKeySummaryFromRow(apiKey).Annotations`（已是 `map[string]string`）。
- 每个 row 的 provider/entry 通过 `merge` 得到合并结果。
- 把合并结果同时塞进：
  - JS 可见的 `Candidate.Annotations`
  - 非 JS 可见的 `providerSidecar.annotations`（unified handler 已经有这个 sidecar 结构；path-based handler 也加一份），方便 TODO #2 不再做第二次解析。
- 选中 candidate 调 `RunBeforeRequestHook` / `RunRewriteHook` 时把这份 map 放到 input 顶层 `annotations`。

`provider_models[].annotations` 的解码：现状是路由 SQL 直接 `elem -> 'annotations'` 当 JSONB 取出来，一路传到 candidate 再到 JS。改造为：candidate 构建时 `annotations.Decode` 一次，得到 `map[string]string`，存到 `Candidate.MPE.Annotations`（类型从 `json.RawMessage` 改为 `map[string]string`）。`Candidate.Provider.Annotations` 同理改成 `map[string]string`，与 `ApiKeySummary` 风格一致。这样 JS 端访问统一，不再需要 `JSON.parse`。

## 复用给 TODO #2 的接口

- `pkg/annotations` 的 `Merge` / `Decode` 是直接消费点。
- `providerSidecar.annotations` 已经是 merged 后的 `map[string]string`，TODO #2 在 unified handler 选中 candidate 后：
  ```go
  outboundType := side.annotations["ah.outbound.type"]
  outboundCfgJSON := side.annotations["ah.outbound.config"]
  ```
  即可直接用，无需再走一次解析路径。
- TODO #3 不动 `llmbridge.outboundFor` 的签名；TODO #2 会扩展它接收 `outboundType` 与 `Config`，本次 spec 不规划。

## 不动的事

- 不引入嵌套 annotations / 不支持非 string 值（dashboard 也只发 string，保持一致）。
- 不动 `RewriteHook` 的 body 处理与 retry 协议。
- 不改 `ApiKeySummary.Annotations` 形态。
- 不缓存 builder 跨请求；annotations 量小，每请求构造即可。
