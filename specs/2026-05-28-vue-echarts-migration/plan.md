# Plan: vue-echarts 图表迁移

## Step 1: 安装依赖

在 `dashboard/` 下安装 `echarts` 和 `vue-echarts`：

```bash
pnpm --dir dashboard add echarts vue-echarts
```

## Step 2: 添加图表色盘 CSS 变量

编辑 `dashboard/src/index.css`：

- 在 `@theme` 块中添加 `--color-chart-0` 到 `--color-chart-9`（light 主题默认值）
- 在 `solarized-light`、`solarized-dark`、`dark` 三个 `:root[data-theme]` 块中各添加对应的覆盖值

具体色值见 `design.md` 的色盘表。

## Step 3: 创建 ECharts 注册文件

创建 `dashboard/src/components/charts/echarts.ts`：

- 从 `echarts/core` 按需注册 `CanvasRenderer`、`LineChart`、`PieChart`、`SankeyChart`、`CustomChart`、`GridComponent`、`TooltipComponent`、`DatasetComponent`
- 导出 `use` 后的类型定义（`ComposeOption` 联合类型），供 `v-chart` 的 `:option` 类型约束使用

## Step 4: 更新 colors.ts

重写 `dashboard/src/components/charts/colors.ts`：

- `groupColor(index)` 改为读取 `--color-chart-{index}` CSS 变量（index < 10 时）
- index >= 10 时，从 `--color-chart-{index % 10}` 的 OKLCH 色调出发做旋转
- 新增 `getChartColors()` 函数返回当前主题下的 10 色数组（`string[]`），通过 `getComputedStyle` 读取 CSS 变量，供 ECharts option 的 `color` 属性使用
- 新增 `getThemeAxisStyle()` 函数返回坐标轴/网格样式所需的颜色（ink-muted、ink-faint、line 等），通过 `getComputedStyle` 读取

## Step 5: 迁移 OverviewAreaStack

重写 `dashboard/src/components/charts/OverviewAreaStack.vue`：

- 导入 `vue-echarts` 的 `VChart` 和 `./echarts`
- 保持 props 接口不变（`groups`, `buckets`, `points`, `height?`, `valueFormat?`, `bucketFormat?`）
- 保持 `hiddenKeys`、`toggleSeries`、`isolateSeries` 逻辑不变
- 用 computed 构建 ECharts option：
  - `xAxis`: category 类型，`data` 为格式化后的 bucket 标签
  - `yAxis`: value 类型，axisLabel 使用 `valueFormat` 或 `compactNumber`
  - `series`: 每个 visibleGroup 对应一个 `type: 'line'` series，带 `areaStyle`、`stack: 'total'`、`smooth: true`
  - `tooltip`: trigger 'axis'，自定义 formatter
  - `grid`: margin 与现有一致
  - `color`: 从 visibleColors 读取
- 保持底部 legend tag 列表（`<ul>` + `Tag` 组件），与现有完全一致

## Step 6: 迁移 OverviewLineChart

重写 `dashboard/src/components/charts/OverviewLineChart.vue`：

- 与 AreaStack 类似，但 series 不设 `areaStyle` 和 `stack`
- `connectNulls: true` 替代 unovis 的 `interpolateMissingData`
- 其余 props、交互、legend 保持一致

## Step 7: 迁移 OverviewDonut

重写 `dashboard/src/components/charts/OverviewDonut.vue`：

- `type: 'pie'`，`radius: ['60%', '72%']`
- `data` 数组中每项带 `itemStyle.color` 使用 `groupColor(originalIndex)`
- 过滤掉 `hiddenKeys` 中的项
- `emphasis.scaleSize: 4`
- tooltip formatter 显示 label、formatted value、百分比
- `label: false`（不在图上显示文字）
- 如有 `centralLabel` / `centralSubLabel`，使用 `graphic` 组件在中心绘制文字
- 保持底部 legend tag 列表

## Step 8: 迁移 OverviewSankey

重写 `dashboard/src/components/charts/OverviewSankey.vue`：

- `type: 'sankey'`
- `data` (nodes) + `links` 直接映射
- 节点颜色：`__other__` 节点用 `var(--color-ink-faint)`，其余按 `groupColor(node.layer)`
- `nodeGap: 14`、`nodeWidth: 14`
- `label.show: true`，显示节点名称
- tooltip formatter 保持现有逻辑（link 显示 source → target + value，node 显示 label + total）
- 高度固定 288px

## Step 9: 迁移 OverviewSpeedTimeline

重写 `dashboard/src/components/charts/OverviewSpeedTimeline.vue`：

- 使用 `type: 'boxplot'` series，水平布局
- `yAxis`: category 类型，`data` 为各组的 label（倒序排列使第一组在顶部）
- `xAxis`: value 类型，axisLabel 使用 `valueFormat`
- 每个数据项编码为 boxplot 五值 `[min, min, median, max, max]`，退化为 min-max 范围显示
- 每个数据项通过 `itemStyle.color` 设置对应的 `groupColor`
- tooltip formatter 显示分组名 + min — max 范围
- 保持 "暂无数据" 空状态

## Step 10: 主题响应

在各图表组件中：

- 使用 `usePreferencesStore()` 的 `theme` 作为 watch 依赖
- theme 变化时重新读取 CSS 变量、重建 option，触发 ECharts 更新
- `v-chart` 组件的 `:option` 绑定 computed，自动响应

## Step 11: 移除 unovis 依赖

```bash
pnpm --dir dashboard remove @unovis/ts @unovis/vue
```

## Step 12: 更新文档

- `dashboard/CLAUDE.md`：将 Charts 段落中的 `@unovis/vue` 改为 `vue-echarts`
- `dashboard/DESIGN_SYSTEM.md`：如有 unovis 相关引用，更新为 ECharts
- `dashboard/package.json`：确认 unovis 已移除、echarts + vue-echarts 已添加

## Step 13: 类型检查与 lint

```bash
pnpm --dir dashboard type-check
pnpm --dir dashboard lint
```

确保无类型错误和 lint 违规。
