## Design — 模型定价

### 数据模型

#### 1. `exchange_rate` 表（新增）

```sql
CREATE TABLE exchange_rate (
  code TEXT PRIMARY KEY,         -- ISO 4217 货币代码，如 USD / CNY / EUR / JPY
  name TEXT NOT NULL,            -- 显示名，如 "美元" / "人民币"
  symbol TEXT NOT NULL,          -- 货币符号，如 "$" / "¥"
  units_per_usd NUMERIC NOT NULL -- 1 USD 等于多少单位的该币种（USD 自身固定为 1）
);
```

迁移随后插入一行 `('USD', 'US Dollar', '$', 1)` 作为默认基准。USD 不可删除（前端按 code === 'USD' 锁定按钮；后端在 delete 处理函数里返回 400）。

换算公式：
```
amount_in_target = amount_in_source * (units_per_usd[target] / units_per_usd[source])
```

#### 2. 定价数据结构（共享 JSON 形状）

定价同时挂在 `model.pricing` 和 `ProviderModelEntry.pricing`，使用同一份 JSON：

```json
{
  "currency": "USD",
  "tiers": [
    {
      "minInputTokens": 0,
      "input": 3,
      "output": 15,
      "cacheRead": 0.3,
      "cacheWrite": 3.75,
      "cacheWrite1h": 6,
      "implicitCacheRead": 0
    },
    {
      "minInputTokens": 200000,
      "input": 6,
      "output": 30,
      "cacheRead": 0.6,
      "cacheWrite": 7.5,
      "cacheWrite1h": 12,
      "implicitCacheRead": 0
    }
  ]
}
```

约束：
- 所有价格以 `currency` 为单位、**per 1M tokens** 表示。
- `tiers` 至少包含一项；普通定价等价于「单 tier，`minInputTokens = 0`」。
- `tiers` 按 `minInputTokens` 升序保存。
- 每个 tier 的 6 个价格字段都必须存在（缺省为 0），便于前端算式简单。
- `pricing` 字段缺失或 `tiers` 为空数组表示「未定价」。

#### 3. 模型表：新增 `pricing JSONB`

```sql
ALTER TABLE model ADD COLUMN pricing JSONB NOT NULL DEFAULT '{}'::jsonb;
```

`{}` 表示未定价。

#### 4. Provider 内嵌的上游条目：扩展 `ProviderModelEntry`

`ProviderModelEntry`（`pkg/contract/provider.go`）增加 `Pricing *Pricing`，序列化为 `pricing` 字段。沿用 `provider_models` JSONB 数组，无需 SQL 迁移。

#### 5. 请求表：新增 4 列保存成本快照

```sql
ALTER TABLE request
  ADD COLUMN model_cost NUMERIC(20, 6),
  ADD COLUMN model_cost_currency TEXT,
  ADD COLUMN upstream_cost NUMERIC(20, 6),
  ADD COLUMN upstream_cost_currency TEXT;
```

四列均可为 `NULL`：
- `(model_cost, model_cost_currency)`：用 `model.pricing` 计算出的金额与原始货币代码。两者要么都为 `NULL`（无定价或缺 token 数），要么同时非空。
- `(upstream_cost, upstream_cost_currency)`：用对应 `provider.providerModels[]` 条目的 `pricing` 计算出的同义快照。

META 与 UPSTREAM 两类记录都会写这 4 列。存原始货币、不在落库时换算——展示侧负责按用户偏好换算。

精度选择：`NUMERIC(20, 6)` 容纳到小数点后 6 位、整数部分 14 位（最大约 9.99e13），覆盖在大模型大批量场景下的最高定价 × 最大 token 用量。

### 计费逻辑（后端）

成本在请求完成时由后端计算并落库。这样：
- 历史请求的成本不会因后期改价或定义阶梯被改动。
- 前端列表和详情不需要在客户端反向找定价——直接读快照即可。
- 多份成本（模型价 / 上游价）天然并行存储。

触发时机：`updateRequestOnComplete`（response 写入完毕、token 数已知）。该 SQL 同时服务 META 元请求与 UPSTREAM 子请求——两类记录都有 `model` 与 `provider_id`，因此使用同一段计算逻辑即可：
- 用 `request.model` 反查 `model.pricing` → 写入 `model_cost / model_cost_currency`。
- 用 `request.providerId` 取出 provider，再在其 `providerModels[]` 中找 `model == request.model` 的条目 → 写入 `upstream_cost / upstream_cost_currency`。

META 行最终归属于成功（或最后一次尝试）的那个 provider，因而其 `upstream_cost` 反映该 provider 的定价；UPSTREAM 行各自反映尝试时的 provider。任一侧缺定价或缺必要 token 数时，该侧两列保持 `NULL`。

#### 选档算法

```
function pickTier(tiers, inputTokens):
    候选 = tiers 中所有 minInputTokens ≤ inputTokens 的项
    return 候选中 minInputTokens 最大的一项；若无则返回 null
```

#### 单次请求成本

```
cost(tier, request):
    return (
        tier.input             * (request.inputTokens      ?? 0) +
        tier.output            * (request.outputTokens     ?? 0) +
        tier.cacheRead         * (request.cacheReadTokens  ?? 0) +
        tier.cacheWrite        * (request.cacheWriteTokens ?? 0) +
        tier.cacheWrite1h      * 0 +     // 暂未在 request 中追踪
        tier.implicitCacheRead * 0       // 暂未在 request 中追踪
    ) / 1_000_000
```

四舍五入到 6 位小数。`cacheWrite1h` 与 `implicitCacheRead` 暂无对应的 token 计数列；保留字段但贡献为 0，后续可在 `request` 表加列后启用。

#### 渲染规则

- `RequestView` 增加 `modelCost? / modelCostCurrency? / upstreamCost? / upstreamCostCurrency?` 4 个字段。
- 请求列表显示一个金额：优先 `upstreamCost`，其次 `modelCost`，否则 `—`。列表展示的就是 META 行自身的成本，不需要聚合下属 UPSTREAM。
- 请求详情面板新建「成本」section，逐项展示：模型价 / 上游价；若两者都有则并排呈现。
- 货币：根据用户偏好（见下）渲染原始或换算金额。

### 前端货币偏好

`stores/preferences.ts` 增加：

```ts
displayCurrency: string | null  // null 表示原始货币
```

新组合式函数 `useCurrency`（`composables/useCurrency.ts`）：
- 暴露当前所有 `exchange_rate` 行（响应式）。
- 提供 `convert(amount, fromCode)`：根据 `displayCurrency` 与 USD 中转换算（缺失汇率时回退到原始币种）。
- 提供 `format(amount, fromCode, opts)`：按 4 位小数 + 货币符号渲染（小金额合理处理）。

成本本身由后端落库；前端只负责换算与展示，不重新计算金额。

加载汇率：在 `App.vue` 启动时一次性 `GET /api/picotera/exchange-rates`，存入新的 Pinia store `useExchangeRatesStore`；CRUD 操作完成后调用 `refresh()`。

### 前端组件

#### `PricingEditor.vue`（新组件）

挂在 `components/`，被 `ModelForm.vue` 和 `ProviderForm.vue` 中既有的 `ProviderModelsPanel` / 上游条目编辑流程复用。Props：
- `modelValue: Pricing | null`（v-model 兼容）
- `availableCurrencies: ExchangeRateView[]`

UI：
- 顶部：货币下拉（来自 `availableCurrencies`，默认 USD）。
- 「档位」列表：每行 6 个数字输入 + `minInputTokens`；首行 `minInputTokens` 锁定为 0、不可删除。
- 「+ 增加阶梯」按钮在最末追加；阶梯数量 ≥ 2 时启用「阶梯定价」标签。
- 完全清空（用户主动删除全部档位）→ `modelValue` 设为 `null`。

#### `RatesView.vue`（新视图）

`/rates` 路由 + `AppSidebar` 入口（图标使用 `currency-dollar`）。表格列：code / name / symbol / units_per_usd / 操作。USD 行禁用删除。新增/编辑通过侧滑面板 `RateForm.vue`。

#### `MoneyDisplay.vue`（小展示组件）

封装「展示金额 + 原币提示」逻辑。Props：
- `amount: number | null`
- `currency: string`
- `fallback?: string`（默认 `'—'`）

行为：
- 用 `useCurrency.format` 渲染。
- 如果发生了换算（即 `currency` 与首选币种不同），用 `title` 显示原始金额。

#### Preferences 菜单更新

`PreferencesMenu.vue` 增加「主要货币」section：下拉选择「原始货币」或任一 `exchange_rate.code`。

#### 模型列表与请求页改动

- `ModelsView.vue`：新增「价格」列。规则：单 tier 时显示「输入 / 输出 per 1M」；多 tier 时显示「分级 N」徽章；未定价显示 `—`。
- `ProvidersView.vue` / `ProviderModelsPanel.vue`：上游条目编辑界面里加入 `PricingEditor`。
- `RequestsView.vue`：新增「成本」列；直接读 `upstreamCost` / `modelCost` 字段，套 `MoneyDisplay`。
- `RequestDetailsPanel.vue`：新增「成本」section，分别展示后端落库的模型价与上游价。

### 后端

#### `exchange_rate` 全 CRUD

5 个 Huma operation：list / get / put（upsert）/ delete。USD 在 delete 中拒绝。

#### `pricing` 字段流转

- `ModelView` 增加 `Pricing *Pricing`，`PutModel` 接受、`ToModelView` 输出。
- `ProviderModelEntry.Pricing *Pricing`，已有的 provider upsert 路径直接带过去（结构变更不需要数据迁移；旧条目反序列化时该字段为 `nil`）。

#### Pricing 类型

新增 `pkg/contract/pricing.go`，定义 `Pricing`、`PricingTier`，并实现 JSON marshal/unmarshal 时校验：
- `Tiers` 升序、首项 `MinInputTokens == 0`、所有数值 ≥ 0。
- 校验失败返回 400。

#### 成本计算器

新增 `pkg/server/pricing.go`：
- `computeCost(p *Pricing, inputTokens, outputTokens, cacheReadTokens, cacheWriteTokens int32) (amount big.Rat, currency string, ok bool)`。
- `ok=false` 表示「无法计费」（`p == nil`、tiers 为空、或 inputTokens 没传无法选档）。
- 内部用 `math/big.Rat` 累加，最后量化为 `pgtype.Numeric` 的 6 位小数表示。

在 `updateRequestOnComplete` 调用前，handler 调用此函数算出 `(modelCost, modelCostCurrency)` 与 `(upstreamCost, upstreamCostCurrency)` 一起写入。

无 Go 测试要求。

### 不在本次范围

- `request` 表新增 `cache_write_1h_tokens` / `implicit_cache_read_tokens` 列（未来工作）。
- 后端聚合统计、报表、按时间段汇总成本（未来工作）。
- 多基准币种、历史汇率快照（不必要）。

### 第三方依赖

无新增。汇率换算是简单除/乘；`Pricing` JSON 用标准 `encoding/json`；前端格式化用 `Intl.NumberFormat`。
