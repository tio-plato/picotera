# 移除上游计价

移除系统中关于"上游计价"（provider-level pricing）的所有部分：

1. 移除 `ProviderModelEntry.Pricing` 字段 — 渠道模型上不再挂定价。
2. 移除 `request` 表上的 `upstream_cost` / `upstream_cost_currency` 列。
3. 移除所有引用上游计价的后端逻辑（`providerEntryPricing()`、`costsFor()` 的 provider 侧计算等）。
4. 移除 Dashboard 中显示上游成本的 UI（RequestDetailsContent 的上游价、TracesView 的上游成本列、RequestsView 中 fallback 到 upstream cost 的逻辑）。
5. 移除 `ProviderModelsPanel.vue` 中每个模型行内的 `PricingEditor`（model-level 的定价保留，仅移除 provider-model-level 的）。
6. `request_overview_hourly` 连续聚合目前 `COALESCE(upstream_cost, model_cost, 0)` 优先取上游价，改为只取 `model_cost`。
7. 所有原来使用上游计价的地方统一改为使用模型级别（`model.pricing`）计价。
