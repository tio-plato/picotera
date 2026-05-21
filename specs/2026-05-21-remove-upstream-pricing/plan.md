# Plan: 移除上游计价

## Step 1: 数据库迁移

新建 `db/migrations/025_remove_upstream_cost.sql`。

**Up:**
1. `DROP` continuous aggregate policy。
2. `DROP MATERIALIZED VIEW request_overview_hourly`。
3. `ALTER TABLE request DROP COLUMN upstream_cost, DROP COLUMN upstream_cost_currency`。
4. `CREATE MATERIALIZED VIEW request_overview_hourly` — 与 019 相同结构，但：
   - `cost_currency` 改为 `COALESCE(NULLIF(model_cost_currency, ''), '')`。
   - `cost` 改为 `SUM(COALESCE(model_cost, 0))::numeric(20, 6)`。
5. 恢复 `materialized_only = false` 和 continuous aggregate policy（同 019 参数）。

**Down:**
1. Drop policy + view。
2. `ALTER TABLE request ADD COLUMN upstream_cost NUMERIC(20, 6), ADD COLUMN upstream_cost_currency TEXT`。
3. Recreate 旧版 view（还原 019 的 COALESCE 逻辑）。
4. 恢复 policy。

使用 `-- +goose NO TRANSACTION`。

## Step 2: sqlc 查询

编辑 `db/queries/request.sql`：

- `ListRequests`：SELECT 列表删除 `r.upstream_cost, r.upstream_cost_currency`。
- `ListRequestsBySpan`：SELECT 列表删除 `r.upstream_cost, r.upstream_cost_currency`。
- `ListRequestTraces`：删除整个 `upstream_costs` LATERAL JOIN（lines 88-104），将 `COALESCE(upstream_costs.costs, '[]'::jsonb)::jsonb AS upstream_costs` 从 SELECT 列表删除。
- `UpdateRequestOnComplete`：SET 子句删除 `upstream_cost = $14, upstream_cost_currency = $15`，同时调整参数编号。

运行 `sqlc generate` 重新生成 `pkg/db/`。

## Step 3: Go contract 类型

编辑 `pkg/contract/provider.go`：
- `ProviderModelEntry`：删除 `Pricing *Pricing` 字段。

编辑 `pkg/contract/request.go`：
- `RequestView`：删除 `UpstreamCost` 和 `UpstreamCostCurrency` 字段。
- `RequestTraceView`：删除 `UpstreamCosts` 字段。
- `requestLike`：删除 `UpstreamCost` 和 `UpstreamCostCurrency` 字段。
- `toRequestView()`：删除 upstream cost 的赋值代码块（lines 173-179）。
- `ToRequestView()`：删除 `UpstreamCost` / `UpstreamCostCurrency` 赋值。
- `ToListRequestRowView()`：同上。
- `ToListRequestsBySpanRowView()`：同上。
- `ToRequestTraceView()`：删除 `upstreamCosts` 解析和赋值。

## Step 4: Go 计价逻辑

编辑 `pkg/server/pricing.go`：
- 删除 `providerEntryPricing()` 函数（lines 107-123）。

编辑 `pkg/server/gateway_helpers.go`：
- `costsFor()` 改为只计算 model cost：
  - 移除 `providerID` 参数。
  - 移除 provider 侧的 `if providerID > 0` 分支。
  - 返回值从 `(modelCost, modelCcy, upstreamCost, upstreamCcy)` 缩减为 `(modelCost pgtype.Numeric, modelCcy pgtype.Text)`。

编辑 `pkg/server/handle_gateway.go`：
- `streamSuccess()`（line 718）：适配 `costsFor` 新签名（去掉 providerID，只接收 2 个返回值）。
- 所有 `UpdateRequestOnCompleteParams` 构造处：去掉 `UpstreamCost` / `UpstreamCostCurrency` 字段。

编辑 `pkg/server/handle_unified_gateway.go`：
- `unifiedStreamSuccess()`（line 1174）：同上适配。
- 所有 `UpdateRequestOnCompleteParams` 构造处：去掉 upstream 字段。

## Step 5: Dashboard — 移除 ProviderModelsPanel 中的上游定价

编辑 `dashboard/src/components/ProviderModelsPanel.vue`：
- `Row` type：删除 `pricing` 字段。
- `ComparableModel` type：删除 `pricing` 字段。
- `entryToRow()`：删除 `pricing` 赋值。
- `emptyRow()`：删除 `pricing` 赋值。
- `comparablePricing()` 函数：删除。
- `comparableRow()`：删除 `pricing`。
- `comparableEntry()`：删除 `pricing`。
- `comparableModelSortKey()`：删除 `JSON.stringify(value.pricing)` 段。
- `rowsToList()`：删除 pricing 相关赋值。
- 模板中删除 `<Field label="定价">` + `<PricingEditor>` 块。
- 删除 `PricingEditor` 和 `Pricing` 类型的 import。

## Step 6: Dashboard — 移除请求/Trace 视图中的上游成本显示

编辑 `dashboard/src/components/RequestDetailsContent.vue`：
- 删除「上游价」的 `<Field>` 块（lines 329-336）。
- 条件 `v-if` 只保留 `selected.modelCost != null`。

编辑 `dashboard/src/views/TracesView.vue`：
- columns 中删除 `{ key: 'upstreamCosts', header: '上游成本', align: 'right' }`。
- 删除 `#cell-upstreamCosts` template slot。

编辑 `dashboard/src/views/RequestsView.vue`：
- cost 列的 `MoneyDisplay`：`amount` 改为 `row.modelCost ?? null`，`currency` 改为 `row.modelCostCurrency || ''`。

## Step 7: 重新生成 OpenAPI 和 TS 类型

1. `mise run openapi` — 重新生成 `openapi.yaml`。
2. `pnpm --dir dashboard generate-openapi` — 重新生成 TS 类型。

## Step 8: 验证

1. `go build ./cmd/picotera` — 编译通过。
2. `go test ./pkg/server/...` — 测试通过（`pricing_test.go` 不受影响，它只测 `computeCost`）。
3. `pnpm --dir dashboard type-check` — TS 类型检查通过。
4. `pnpm --dir dashboard lint` — lint 通过。
