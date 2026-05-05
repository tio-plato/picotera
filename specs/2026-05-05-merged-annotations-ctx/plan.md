# Plan — Merged annotations in hook ctx

按顺序执行；每一步都跑 `go build ./...` / `pnpm --dir dashboard type-check` 兜底，必要处再跑 `go test ./pkg/server/...`。

## 1. 新建 `pkg/annotations` 包

- 新文件 `pkg/annotations/annotations.go`：实现 `Merge` 和 `Decode`，签名见 `api.md`。
- 新文件 `pkg/annotations/annotations_test.go`，覆盖：
  - `Decode(nil)` / `[]byte("")` / `[]byte("null")` / `[]byte("{}")` 都返回空 map + nil。
  - 非 object 顶层 JSON 返回 error。
  - 非 string 值（数字 / bool / null）被 `fmt.Sprint` 化。
  - `Merge` 四层（model/provider/entry/apiKey）按顺序覆盖。
  - `Merge()` / 全 nil 输入返回非 nil 空 map。

## 2. DB schema 与 sqlc

1. 新建 `db/migrations/013_model_annotations.sql`：
   ```sql
   -- +goose Up
   ALTER TABLE model ADD COLUMN annotations JSONB NOT NULL DEFAULT '{}'::jsonb;
   -- +goose Down
   ALTER TABLE model DROP COLUMN annotations;
   ```
2. 改 `db/queries/model.sql`：`GetModelByName` / `GetModels` 显式列出列（含 `annotations`）；`UpsertModel` 新增 `annotations` 参数（位置加在 `pricing` 之后），`ON CONFLICT` 子句也更新它。
3. 改 `db/queries/routing.sql`：两个 routing 查询的 SELECT 列表加 `m.annotations AS model_annotations`。
4. 跑 `mise run sqlc-generate`（如无则 `sqlc generate`），确认 `pkg/db/` 生成里 `Model.Annotations`、`GetProvidersByEndpointAndModelRow.ModelAnnotations`、`GetProvidersByEndpointTypesAndModelRow.ModelAnnotations` 都出现。

## 3. Contract 层（`pkg/contract/model.go`）

1. `ModelView` 加 `Annotations map[string]string` 字段（json 标签 `annotations`，`omitempty` 不加，保持 `{}`）。
2. `ToModelView`：用 `pkg/annotations.Decode(model.Annotations)` 解码并写入。
3. 新增上行 `FromModelView`（参考 `provider.go` 的 `ProviderView -> db.UpsertProviderParams` 风格）：把 `Annotations` marshal 成 `[]byte`（空 map 也写 `{}`）。`PutModel` handler 调用它得到 `db.UpsertModelParams`。
4. 改 `pkg/server/handle_models.go` 中 `PutModel` 调用，把新增字段送到 sqlc。

## 4. 跑通 backend 编译

`go build ./...`；新增字段不破坏现有 `db.Model` 用法，确认无回归即可。

## 5. JSX 类型与 SDK 形状

1. 改 `pkg/jsx/types.go`：
   - 把 `ProviderSummary.Annotations` 由 `json.RawMessage` 改成 `map[string]string`。
   - 把 `CandidateMPE.Annotations` 由 `json.RawMessage` 改成 `map[string]string`。
   - `Candidate` 新增 `Annotations map[string]string` 字段（merged）。
   - 新增 `type ModelSummary struct { Name string; Annotations map[string]string }`。
   - `SortInput.Model` / `BeforeRequestInput.Model` / `RewriteInput.Model` / `RewriteProviderModelsInput` 字段类型由 `any` 改成 `*ModelSummary`（保持指针，方便未来扩展且兼容 `nil`）。
   - `RewriteModelInput`、`BeforeRequestInput`、`RewriteInput`、`RewriteProviderModelsInput` 各加顶层 `Annotations map[string]string`（json 标签 `annotations`）。
2. 不改 SDK 公共 API（`Run*Hook` 签名保持），新增字段全部走 input 结构体。

## 6. Server 层接线

1. 新建 `pkg/server/annotations.go`，实现 `candidateAnnotationsBuilder`：
   ```go
   type candidateAnnotationsBuilder struct {
       modelAnno  map[string]string
       apiKeyAnno map[string]string
   }
   func newCandidateAnnotationsBuilder(modelAnnoRaw []byte, apiKeyAnno map[string]string) (*candidateAnnotationsBuilder, error)
   func (b *candidateAnnotationsBuilder) merge(providerAnnoRaw []byte, entryAnno map[string]string) (
       merged, providerDecoded map[string]string,
   )
   ```
   `merge` 同时返回 provider 层解码结果，避免 caller 再跑一次 `Decode`。合并顺序：`model < provider < entry < apiKey`。
2. `handle_gateway.go`（顺序与现有代码保持一致）：
   1. `authenticateClient` 成功后，`apiKeyAnno := apiKeySummaryFromRow(apiKey).Annotations`。这一份 map 在整个请求生命周期内不变。
   2. `extractModel` 之后、`RunRewriteModelHook` 之前，调 `GetModelByName(modelName)`：成功则 `Decode` 出 model annotations；`pgx.ErrNoRows` 时按 `{}` 处理。得到初始 `modelAnno`。
   3. `RunRewriteModelHook` 的 ctx：`Annotations: Merge(modelAnno, apiKeyAnno)`，`Model: &jsx.ModelSummary{Name: originalModelName, Annotations: modelAnno}`，`ApiKey: apiKeyJS`（保持现状）。如果 hook 改写了 model，重新读一次 `GetModelByName(newModel)` 刷新 `modelAnno`。
   4. 路由查询返回后，`providers[0].ModelAnnotations` 覆盖 `modelAnno`（同一查询里所有行都相同；以路由查询结果为准，避免两次读之间状态漂移）。
   5. 用 `newCandidateAnnotationsBuilder(modelAnno, apiKeyAnno)` 建 builder。逐行调 `builder.merge(row.ProviderAnnotations, decodeEntry(row.Annotations))` 得到 `merged` 与 `providerDecoded`；`entryDecoded` 单独用 `annotations.Decode(row.Annotations)` 拿到。
   6. `providerSidecar` 结构体新增 `annotations map[string]string` 字段，存 merged。
   7. `jsx.Candidate.Provider.Annotations = providerDecoded`，`MPE.Annotations = entryDecoded`，`Candidate.Annotations = merged`。
   8. `RunSortHook` 顶层 `Annotations: Merge(modelAnno, apiKeyAnno)`，`Model: &ModelSummary{...}`。
   9. `RunBeforeRequestHook` / `RunRewriteHook` 顶层 `Annotations: cand.Annotations`（已含 apiKey 层），`Model: &ModelSummary{Name: originalModelName, Annotations: modelAnno}`。
3. `handle_unified_gateway.go`：同样改造（model 拉取走相同的 `GetModelByName`，apiKey annotations 取自 `apiKeyJS.Annotations`），并给 `providerSidecar` 加 `annotations` 字段。
4. `handle_provider_endpoint.go` 中调用 `RunRewriteProviderModelsHook` 的「fetch models」流程没有具体 model 上下文：传 `Model: nil`、`Annotations: Merge(providerAnno, apiKeyAnnoIfAuthed)`。该流程当前如未鉴权 api key，则只 merge provider 这一层。

## 7. JSON 兼容性自检

`pkg/jsx/types.go` 中的 `Annotations map[string]string` 在零值时序列化成 `null`。在 Go 端构造时统一用 `annotations.Merge()`（永不返回 nil），保证 JS 总是看到 `{}` 而非 `null`。给 `ProviderSummary.Annotations` 等字段做同样保证。

## 8. Dashboard

1. `dashboard/src/components/ModelForm.vue`：参照 `ProviderForm` 的 annotations 编辑块，挂上 `<AnnotationsEditor v-model="form.annotations" />`，初值为 `{}`。
2. `dashboard/src/views/ModelsView.vue`：表格 / 详情若展示 annotations 摘要则补一栏（参考 ProvidersView）。最低限度仅保证字段不丢失。
3. `pnpm --dir dashboard generate-openapi`、`pnpm --dir dashboard type-check`、`pnpm --dir dashboard lint`、`pnpm --dir dashboard build`。
4. `mise run openapi` 重新生成 `openapi.yaml`。

## 9. 测试

- `go test ./pkg/annotations/...` — 上面的单元测试。
- `go test ./pkg/server/...` — 跑现有套件，重点关注 `handle_unified_gateway_test.go` 是否仍过；如其断言里硬编码 annotations 形态（`json.RawMessage` 风格），改为 `map[string]string` 形态。
- 如有 `pkg/jsx/engine_test.go` 涉及 annotations 的断言，更新为新形态。

## 10. 自查

- 通读 design.md、api.md、plan.md，删除任何犹豫词。
- 提交前再过一遍 `git diff` 确认没漏掉 sqlc / openapi 的回写。
