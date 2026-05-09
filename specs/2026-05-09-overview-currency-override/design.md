# Design

## Overview

概览页新增一个页面级货币选单，用于控制 `/overview` 内所有费用展示。该选单的选项与全局「主要货币」一致，并在顶部增加「跟随设置」。

「跟随设置」表示概览页使用全局 `preferences.displayCurrency` 的当前值；选择「原始货币」表示概览页强制按原始币种拆开展示；选择任一具体货币代码表示概览页只在本页面内按该币种换算费用，不修改全局设置。

## State Model

在 `dashboard/src/stores/preferences.ts` 增加：

```ts
type OverviewCurrencyOverride = 'original' | string | null

overviewCurrencyOverride: OverviewCurrencyOverride
```

语义：

- `null`：跟随全局 `displayCurrency`。
- `'original'`：概览页强制使用原始货币展示。
- 其它非空字符串：概览页使用该货币代码作为目标币种。

该字段与现有偏好一起写入 `localStorage['picotera.preferences']`。读取时只接受 `null`、`'original'` 或非空字符串；其它值使用 `null`。不对字符串做大小写转换、trim 或其它宽松规整。

## Currency Resolution

货币解析改为 provide/inject 上下文，而不是让每个页面直接读取全局偏好。新增 `dashboard/src/composables/useCurrencyContext.ts`：

```ts
export interface CurrencyContext {
  rates: ComputedRef<ExchangeRateView[]>
  byCode: ComputedRef<Map<string, ExchangeRateView>>
  targetCurrency: ComputedRef<string | null>
  convert(amount: number, fromCode: string): ConvertResult
  convertTo(amount: number, fromCode: string, targetCode: string | null): ConvertResult
  format(amount: number, code: string, opts?: CurrencyFormatOptions): string
}

export function provideCurrencyContext(targetCurrency: ComputedRef<string | null>): CurrencyContext
export function useCurrencyContext(): CurrencyContext
```

`provideCurrencyContext()` 封装现有汇率读取、格式化和换算逻辑；`convert()` 使用当前上下文的 `targetCurrency`，`convertTo()` 使用调用方显式传入的目标币种。`useCurrencyContext()` 使用 Vue `inject` 读取最近的上下文；若组件树上没有 provider，直接抛错。`App.vue` 在应用根部统一 provide 全局上下文，所有调用点都必须在该 provider 之内。

删除 `dashboard/src/composables/useCurrency.ts`。所有现有调用点改为导入 `useCurrencyContext()`，货币读取路径统一收敛到 provide/inject 上下文。

全局入口在 `dashboard/src/App.vue` 挂载一次默认上下文：

```ts
const prefs = usePreferencesStore()
provideCurrencyContext(computed(() => prefs.displayCurrency ?? null))
```

概览页在自己的组件内再提供一个页面级上下文：

```ts
const parentCurrency = useCurrencyContext()
const overviewTargetCurrency = computed(() => {
  if (prefs.overviewCurrencyOverride === 'original') return null
  return prefs.overviewCurrencyOverride ?? parentCurrency.targetCurrency.value
})
const overviewCurrency = provideCurrencyContext(overviewTargetCurrency)
const isOriginalMode = computed(() => overviewCurrency.targetCurrency.value == null)
```

概览页及其子组件里的 `MoneyDisplay`、图表聚合 helper、Sankey value formatter 都使用最近的货币上下文。选择「跟随设置」时，概览页 provider 的目标币种跟随父级上下文；选择「原始货币」或具体货币时，概览页 provider 覆盖父级上下文。

换算规则保持现有行为：

- 没有有效目标货币，返回原金额和原币种。
- 目标币种与原币种相同，返回原金额和原币种。
- 原币种或目标币种缺汇率，返回原金额和原币种。
- 两端汇率都存在时，用 `(amount / fromUnitsPerUsd) * targetUnitsPerUsd` 换算到目标币种。

## UI Placement

选单放在概览页顶部 controls bar，紧跟时间范围之后，使用现有 `Select` primitive：

- label：`货币`
- 选项 1：`跟随设置`
- 选项 2：`原始货币`
- 后续选项：与全局设置一致，来自 `useExchangeRates().rates`，label 格式保持 `CODE SYMBOL · NAME`

当选择具体货币时，概览页中以下费用区域统一使用该目标币种：

- 总费用卡片
- 费用流向 Sankey
- 费用分布 donut
- 费用时间序列

Token、请求数、追踪数、筛选条件和维度切换不受影响。

## API

不需要后端 API、OpenAPI 或数据库变更。该功能完全是 dashboard 本地偏好与展示逻辑。

## Verification

实现后运行：

```bash
pnpm --dir dashboard type-check
pnpm --dir dashboard lint
```

手动验证：

- 概览页货币默认显示「跟随设置」。
- 全局设置从「原始货币」切到具体货币时，概览页处于「跟随设置」会同步变化。
- 概览页选择「原始货币」后，再修改全局货币，概览页仍按原始币种拆开展示。
- 概览页选择具体货币后，再修改全局货币，概览页费用展示保持页面选择的币种。
- 概览页切回「跟随设置」后重新同步全局设置。
