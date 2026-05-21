# Design: 移除上游计价

## 概述

当前系统有两层计价：
- **模型级定价** (`model.pricing` JSONB) — 全局模型维度，保留。
- **渠道模型级定价** (`ProviderModelEntry.Pricing`) — 每个渠道的每个模型可以挂独立定价，产生 `upstream_cost`，**本次移除**。

移除后，所有成本计算统一走模型级定价，`request` 表只保留 `model_cost` / `model_cost_currency`。

## 数据库变更

新增一个 goose 迁移 `025_remove_upstream_cost.sql`：

1. **重建连续聚合 `request_overview_hourly`**：TimescaleDB 的连续聚合不支持 `ALTER`，必须 drop + recreate。新定义将 `COALESCE(upstream_cost, model_cost, 0)` 改为 `COALESCE(model_cost, 0)`，`cost_currency` 改为 `COALESCE(NULLIF(model_cost_currency, ''), '')`。
2. **DROP `upstream_cost` 和 `upstream_cost_currency` 列**：连续聚合重建后再 drop 列，避免依赖问题。

由于是 `NO TRANSACTION` 迁移（连续聚合操作必须在事务外），down 迁移需要能 re-add 列并重建旧聚合。

## 后端变更

### sqlc 查询

- `UpdateRequestOnComplete`：去掉 `upstream_cost` 和 `upstream_cost_currency` 两个参数。
- `ListRequests` / `ListRequestsBySpan`：SELECT 列表去掉 upstream_cost 相关字段。
- `ListRequestTraces`：去掉 `upstream_costs` 的 LATERAL JOIN。
- overview 查询不变（它们读连续聚合，聚合定义已变）。

### Go 类型

- `contract.ProviderModelEntry`：移除 `Pricing` 字段。
- `contract.RequestView`：移除 `UpstreamCost` / `UpstreamCostCurrency`。
- `contract.RequestTraceView`：移除 `UpstreamCosts`。
- `contract.requestLike`：移除对应字段。
- `toRequestView()`、`ToRequestView()`、`ToListRequestRowView()`、`ToListRequestsBySpanRowView()`：移除 upstream cost 赋值。
- `ToRequestTraceView()`：移除 upstream costs 解析。

### 计价逻辑

- `costsFor()`：移除 provider 侧计算（整个 `if providerID > 0` 分支），返回值从 4 个缩减为 2 个 (`modelCost`, `modelCcy`)。去掉 `providerID` 参数。
- `providerEntryPricing()`：整个函数删除。
- 所有 `costsFor()` 调用点（`handle_gateway.go:718`、`handle_unified_gateway.go:1174`）：适配新签名。
- 所有 `UpdateRequestOnCompleteParams` 构造处：去掉 `UpstreamCost` / `UpstreamCostCurrency` 字段。

## Dashboard 变更

- `ProviderModelsPanel.vue`：移除 `PricingEditor` 引用和 `pricing` 相关字段（Row type、entryToRow、emptyRow、rowsToList、comparableRow 等）。
- `RequestDetailsContent.vue`：移除「上游价」显示。
- `TracesView.vue`：移除「上游成本」列。
- `RequestsView.vue`：cost 列改为只用 `modelCost` / `modelCostCurrency`。
- 重新生成 `openapi.yaml` 和 TS 类型。
