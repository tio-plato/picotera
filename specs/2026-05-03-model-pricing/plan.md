## Execution Plan — 模型定价

### 阶段 1 — 数据库与共享类型

1. **新增迁移 `db/migrations/010_pricing_and_exchange_rate.sql`**
   - `CREATE TABLE exchange_rate (code TEXT PK, name TEXT NOT NULL, symbol TEXT NOT NULL, units_per_usd NUMERIC NOT NULL)`。
   - `INSERT INTO exchange_rate (code, name, symbol, units_per_usd) VALUES ('USD', 'US Dollar', '$', 1)`。
   - `ALTER TABLE model ADD COLUMN pricing JSONB NOT NULL DEFAULT '{}'::jsonb`。
   - `ALTER TABLE request ADD COLUMN model_cost NUMERIC(20, 6)`、`model_cost_currency TEXT`、`upstream_cost NUMERIC(20, 6)`、`upstream_cost_currency TEXT`（均可空）。
   - `Down`：反向操作。

2. **新增 sqlc 查询 `db/queries/exchange_rate.sql`**
   - `GetExchangeRates :many`、`GetExchangeRateByCode :one`、`UpsertExchangeRate :one`、`DeleteExchangeRate :exec`。
   - 数值字段对应到 pgx 的 `pgtype.Numeric`。

3. **修改 `db/queries/model.sql`** 让 `GetModels` / `GetModelByName` / `UpsertModel` 处理 `pricing` 字段。`UpsertModel` 多一个参数。

4. **修改 `db/queries/request.sql`**
   - `ListRequests`、`ListRequestsBySpan`、`GetRequest` SELECT 列表加入 4 列。
   - `UpdateRequestOnComplete` 参数列表追加 `model_cost = $11, model_cost_currency = $12, upstream_cost = $13, upstream_cost_currency = $14`。
   - 不修改 `UpdateRequestMetrics`（轻量心跳路径不算成本）。

5. **运行 `sqlc generate`** 重新生成 `pkg/db/`。

6. **新增 `pkg/contract/pricing.go`**
   - 定义 `Pricing`、`PricingTier`。
   - 实现 `(*Pricing) UnmarshalJSON` / `Validate()`：tiers 升序、首项 0、各字段 ≥ 0。
   - 提供 `PricingFromJSONB([]byte) (*Pricing, error)` 与 `PricingToJSONB(*Pricing) ([]byte, error)` 辅助函数（空对象 ↔ nil）。

### 阶段 2 — 后端 API 与成本计算

7. **`pkg/contract/exchange_rate.go`（新文件）**
   - `ExchangeRateView`、各 request/response 类型、4 个 `huma.Operation` 定义。
   - `ToExchangeRateView` / `FromExchangeRateView`，处理 `pgtype.Numeric` ↔ `float64`。

8. **`pkg/server/handle_exchange_rate.go`（新文件）**
   - 4 个 handler：list / get / put / delete。
   - delete 时 `code == "USD"` → `huma.Error400BadRequest`。
   - put 时 `unitsPerUsd <= 0` → 400。

9. **`pkg/contract/model.go`**
   - `ModelView` 加 `Pricing *Pricing` 字段。
   - `ToModelView`：调用 `PricingFromJSONB`。
   - 让 `PutModelRequest` 使用同一个 `ModelView`（已是）；handler 中调用 `Pricing.Validate()`。

10. **`pkg/server/handle_models.go`**
    - `handlePutModel` 校验 `Pricing`、把 JSONB 传入 `UpsertModel`。
    - `handleGetModel` / `handleListModels`：无需改动（`ToModelView` 已读取）。

11. **`pkg/contract/provider.go`**
    - `ProviderModelEntry` 加 `Pricing *Pricing` 字段（json tag `pricing,omitempty`）。
    - 在 provider upsert handler 流程中调用 `entry.Pricing.Validate()`。

12. **`pkg/contract/request.go`**
    - `RequestView` 增加 `ModelCost *float64` / `ModelCostCurrency string` / `UpstreamCost *float64` / `UpstreamCostCurrency string`。
    - 扩展 `requestLike`、3 个 `To*View` 函数读取新列（`pgtype.Numeric` → `*float64`）。

13. **`pkg/server/pricing.go`（新文件）**
    - `computeCost(p *contract.Pricing, inputTokens, outputTokens, cacheReadTokens, cacheWriteTokens *int32) (amount pgtype.Numeric, currency pgtype.Text, ok bool)`。
    - 用 `math/big.Rat` 累加 6 种类型 × token；其中 `cacheWrite1h` / `implicitCacheRead` 当前传 0。
    - `inputTokens == nil` → `ok=false`（无法选档）。
    - 量化到 6 位小数后写入 `pgtype.Numeric`。

14. **`pkg/server/handle_gateway.go` / `gateway_helpers.go`**
    - 抽出 helper `(s *Server) costsFor(ctx, model, providerID, tokens) (modelCost, modelCcy, upstreamCost, upstreamCcy)`：
      - `s.queries.GetModelByName(ctx, model)` 取模型定价。
      - 在 provider 的 `providerModels[]` 中找 `model == request.model` 的条目取上游定价（provider 已在内存里）。
      - 两次调用 `computeCost`；任一侧 `ok=false` 时该侧两列均为 `pgtype.Numeric{Valid:false}` / `pgtype.Text{Valid:false}`。
    - 每个调用 `updateRequestOnComplete` 的位置（UPSTREAM 子请求完成、META 元请求最终汇总）都先调 `costsFor` 再把结果填进 `UpdateRequestOnCompleteParams` 的 4 个新字段。

15. **`pkg/server/server.go`**
    - 在 `registerOperations` 注册 4 个 exchange rate operation。

16. **运行 `mise run openapi`** 重新生成 `openapi.yaml`。

### 阶段 3 — 前端基础设施

17. **`pnpm --dir dashboard generate-openapi`** 同步类型。

18. **`dashboard/src/stores/preferences.ts`**
    - 增加 `displayCurrency: string | null`，默认 `null`。
    - 更新 STORAGE schema 与 load/save 逻辑。

19. **`dashboard/src/stores/exchangeRates.ts`（新文件）**
    - Pinia store：`rates: ExchangeRateView[]`、`fetch()`、`upsert(rate)`、`remove(code)`。
    - 暴露 `byCode = computed(() => Map<string, ExchangeRateView>)`。

20. **`dashboard/src/composables/useCurrency.ts`（新文件）**
    - 接入 `usePreferencesStore` 与 `useExchangeRatesStore`。
    - `convert(amount, fromCode) → { amount, currency, converted: boolean, original: { amount, currency } }`：换算失败时返回原币、`converted=false`。
    - `format(amount, code, opts?)`：`Intl.NumberFormat` + 货币符号。

21. **`dashboard/src/ui/MoneyDisplay.vue`（新组件）**
    - props：`amount: number | null | undefined`、`currency: string | null | undefined`、`fallback?: string`。
    - 调用 `useCurrency.format` 渲染；如果 `converted`，`title` 写原始金额。
    - `amount == null` 或 `currency` 为空时渲染 `fallback`。
    - 在 `src/ui/index.ts` 导出。

22. **`dashboard/src/App.vue`**
    - 启动时调用 `useExchangeRatesStore().fetch()`。
    - `pageMeta` 加 `rates: { title: '汇率', hint: '管理币种与换算' }`。

### 阶段 4 — 汇率管理 UI

23. **`dashboard/src/router/index.ts`**
    - 新路由 `{ path: '/rates', name: 'rates', component: () => import('@/views/RatesView.vue') }`。

24. **`dashboard/src/components/AppSidebar.vue`**
    - 在导航中加入「汇率」入口（图标 `currency-dollar`）；如该 icon 未在 `src/ui/icons/paths.ts` 注册，需补上。

25. **`dashboard/src/components/RateForm.vue`（新文件）**
    - 字段：`code`（编辑时锁定）、`name`、`symbol`、`unitsPerUsd`。
    - 调 `api.PUT('/api/picotera/exchange-rates')`，成功后 `useExchangeRatesStore().fetch()`。

26. **`dashboard/src/views/RatesView.vue`（新文件）**
    - 使用 `DataTable` 列出 rates；操作列含编辑、删除（USD 行禁用删除按钮）。
    - 顶部「新增」按钮 → `panel.open(RateForm, ...)`。

### 阶段 5 — 定价编辑器与表单接入

27. **`dashboard/src/components/PricingEditor.vue`（新文件）**
    - props：`modelValue: Pricing | null`，`emit('update:modelValue', ...)`。
    - 内部 state：本地深拷贝；任何修改都 emit 新对象。
    - 顶部货币 `Select`（来自 `useExchangeRatesStore().rates`）。
    - 「档位」表格：`minInputTokens`、6 个 `Input type="number"`、删除按钮（首行不可删）。
    - 「+ 增加阶梯」按钮在末尾追加；新阶梯 `minInputTokens` 默认为前一行 +1。
    - 「移除定价」按钮：把 `modelValue` emit 为 `null`。

28. **`dashboard/src/components/ModelForm.vue`**
    - `form.pricing = props.model?.pricing ?? null`。
    - `<Field label="定价"><PricingEditor v-model="form.pricing" /></Field>`。
    - submit body 带上 `pricing`。

29. **`dashboard/src/components/ProviderModelsPanel.vue`** 或 `ProviderForm.vue` 中负责 entry 编辑的位置
    - 每个 entry 编辑界面加入 `PricingEditor`，绑定到 entry 的 `pricing`。

### 阶段 6 — 模型/请求页成本展示

30. **`dashboard/src/views/ModelsView.vue`**
    - 新增「价格」`Th`/`Td`：
      - 未定价 → `—`。
      - 单 tier → 输入 / 输出 两个 `MoneyDisplay`，加「per 1M」副标。
      - 多 tier → `<Tag>分级 N</Tag>`，hover 显示档位概览。

31. **`dashboard/src/views/RequestsView.vue`**
    - 新增「成本」列：`<MoneyDisplay :amount="r.upstreamCost ?? r.modelCost" :currency="r.upstreamCostCurrency ?? r.modelCostCurrency" />`。

32. **`dashboard/src/components/RequestDetailsPanel.vue`**
    - 在「Token」section 之后追加「成本」section：
      - 模型价：`<MoneyDisplay :amount="selected.modelCost" :currency="selected.modelCostCurrency" />`。
      - 上游价：`<MoneyDisplay :amount="selected.upstreamCost" :currency="selected.upstreamCostCurrency" />`。
      - 两者都缺时整个 section 隐藏。

### 阶段 7 — 偏好菜单接入

33. **`dashboard/src/components/PreferencesMenu.vue`**
    - 新增「主要货币」section：
      - 选项 = `[{ value: null, label: '原始货币' }, ...rates.map(r => ({ value: r.code, label: `${r.code} ${r.symbol}` }))]`。
      - 用 `Select`。
      - 绑定 `prefs.displayCurrency`。

### 阶段 8 — 验收

34. `pnpm --dir dashboard type-check` 通过。
35. `pnpm --dir dashboard lint` 通过。
36. `pnpm --dir dashboard build` 通过。
37. 手动验证：
    - 新增 USD/CNY 汇率，删除 USD 被拒。
    - 模型 / 上游配置 6 类价格 + 多档；前端读写、列表概要正常。
    - 触发请求；META 与 UPSTREAM 两行的 4 个成本列均被写入；列表与详情读取展示。
    - 切换偏好币种，列表与详情金额随汇率换算；切回「原始货币」恢复原值。
