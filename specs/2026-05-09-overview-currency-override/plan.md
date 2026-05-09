# Plan

1. 修改 `dashboard/src/stores/preferences.ts`
   - 新增 `OverviewCurrencyOverride = 'original' | string | null` 类型。
   - 在 `DEFAULTS` 中加入 `overviewCurrencyOverride: null as OverviewCurrencyOverride`。
   - 在 `load()` 中读取 `parsed.overviewCurrencyOverride`，接受 `'original'` 或非空字符串，其它值回落为 `null`。
   - 在 `persist()` 中写入 `overviewCurrencyOverride`。
   - 将 `overviewCurrencyOverride` 加入 `watch([...])` 依赖列表。
   - 从 store return 中导出 `overviewCurrencyOverride`。

2. 新增 `dashboard/src/composables/useCurrencyContext.ts`
   - 定义 `CurrencyContext`、`CurrencyFormatOptions`、`ConvertResult`。
   - 定义 `currencyContextKey: InjectionKey<CurrencyContext>`。
   - 实现 `createCurrencyContext(targetCurrency: ComputedRef<string | null>)`，封装 `useExchangeRates()`、`rates`、`byCode`、`format()`、`convertTo()`、`convert()`。
   - 实现 `provideCurrencyContext(targetCurrency)`，创建上下文并 `provide(currencyContextKey, ctx)`。
   - 实现 `useCurrencyContext()`：`inject(currencyContextKey)`，缺失时直接抛错，不再回落到默认上下文。
   - 换算规则保持现有 `useCurrency()` 行为：缺目标币种、同币种、任一汇率缺失时返回原金额；两端汇率存在时换算。

3. 删除 `dashboard/src/composables/useCurrency.ts`
   - 将 `ConvertResult` 类型导出迁移到 `useCurrencyContext.ts`。
   - 后续步骤把所有调用点从 `useCurrency()` 改为 `useCurrencyContext()`。

4. 在应用根部提供全局货币上下文
   - 在 `dashboard/src/App.vue` 引入 `provideCurrencyContext` 与 `usePreferencesStore`。
   - 用 `computed(() => prefs.displayCurrency ?? null)` 调用 `provideCurrencyContext(...)`。
   - 保持 `PreferencesMenu.vue` 继续负责修改全局 `prefs.displayCurrency`。

5. 修改 `dashboard/src/ui/MoneyDisplay.vue`
   - 从 `useCurrency()` 切换为 `useCurrencyContext()`。
   - 保持 props 和输出不变，使其自动读取最近的货币 provider。

6. 修改 `dashboard/src/views/OverviewView.vue` 的 script
   - 引入 `usePreferencesStore`。
   - 引入 `useCurrencyContext` 与 `provideCurrencyContext`。
   - 先读取父级上下文：`const parentCurrency = useCurrencyContext()`。
   - 新增 `overviewCurrencyValue` computed，用 `''` 表示「跟随设置」，`'original'` 表示「原始货币」，其它非空字符串写入 `prefs.overviewCurrencyOverride`。
   - 新增 `overviewTargetCurrency` computed：`prefs.overviewCurrencyOverride === 'original'` 时返回 `null`；否则返回 `prefs.overviewCurrencyOverride ?? parentCurrency.targetCurrency.value`。
   - 调用 `const ccy = provideCurrencyContext(overviewTargetCurrency)`，让概览页模板和子组件就近读取页面级货币上下文。
   - 将 `isOriginalMode` 改为基于 `ccy.targetCurrency`。
   - 将概览页内所有费用换算逻辑继续调用 `ccy.convert(...)`；此时 `ccy` 已经是页面级上下文。
   - 将费用标题和 `formatCurrencyCompact` 调用中的展示币种统一读取 `ccy.targetCurrency.value`。

7. 修改 `dashboard/src/views/OverviewView.vue` 的 template
   - 在顶部 controls bar 的时间范围后新增「货币」选单。
   - 第一项为 `<option value="">跟随设置</option>`。
   - 第二项为 `<option value="original">原始货币</option>`。
   - 后续项复用 `ccy.rates`，格式与全局设置一致：`{{ r.code }} {{ r.symbol }} · {{ r.name }}`。
   - 确认费用卡片、Sankey、donut、series 在 override 模式下只渲染一个目标币种视图，在跟随全局且全局为原始货币时仍按原始币种拆开展示。
   - 确认选择「原始货币」时，即使全局设置为具体货币，概览页仍按原始币种拆开展示。

8. 检查其它读取点
   - 仅 `dashboard/src/views/TracesView.vue` 直接调用 `useCurrency()`，改为 `useCurrencyContext()`。
   - `RequestsView.vue`、`ModelsView.vue`、`RequestDetailsContent.vue` 等只通过 `MoneyDisplay` 间接展示金额，改完 `MoneyDisplay` 即自动跟随根部上下文，无需单独修改。
   - 这些概览页外调用点通过 `App.vue` 的根部 provider 读取全局上下文，展示语义保持不变。
   - 不把概览页 override 泄露到概览页外的路由或侧滑面板。

9. 验证
   - 运行 `pnpm --dir dashboard type-check`。
   - 运行 `pnpm --dir dashboard lint`。
   - 在浏览器手动检查默认跟随、原始货币覆盖、具体货币覆盖、切回跟随四条交互路径。
