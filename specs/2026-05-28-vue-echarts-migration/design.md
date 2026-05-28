# Design: vue-echarts 图表迁移

## 技术选型

使用 `vue-echarts` v7 + `echarts` v5。vue-echarts 提供 `<v-chart>` 组件，接受 ECharts option 对象，自动处理响应式更新和销毁。

按需引入 ECharts 模块以控制包体积：通过 `echarts/core` + 逐个注册所需的渲染器、系列、组件。创建 `src/components/charts/echarts.ts` 统一注册并导出 `use` 后的 ECharts 实例。

## 图表类型映射

| unovis 组件 | ECharts 系列类型 | 说明 |
|---|---|---|
| VisArea (stacked) | `type: 'line'` + `areaStyle` + `stack` | 堆叠面积图 |
| VisLine | `type: 'line'` | 多系列折线图 |
| VisDonut | `type: 'pie'` + `radius: ['60%', '72%']` | 环形图 |
| VisSankey | `type: 'sankey'` | 桑基图 |
| VisTimeline (horizontal range) | `type: 'boxplot'` | 水平箱线图，用 boxplot 展示 min-max 范围 |

## 色盘设计

### 架构

在 `index.css` 的每个主题块中新增 `--color-chart-0` 到 `--color-chart-9` 共 10 个 CSS 变量。`colors.ts` 中的 `groupColor(index)` 函数读取这些变量；当 index >= 10 时，基于已有色盘色调做旋转生成扩展色。

### 色盘原则

- 10 色覆盖 OKLCH 色相环的主要区间，相邻色相差 ≥ 30°
- Light 主题：L ≈ 0.55-0.65，C ≈ 0.14-0.20，在白底上保证对比
- Dark 主题：L ≈ 0.70-0.78，C ≈ 0.14-0.20，在深色底上保证可读
- Solarized 主题：色调偏暖（solarized-light）或偏冷（solarized-dark），饱和度适度降低以匹配 Solarized 调性
- 前 4 色与现有语义色（accent / ok / warn / err）的色调保持一致，确保向后兼容

### 各主题色盘

**Light** — 冷蓝基调、高饱和

| 索引 | 色调描述 | OKLCH |
|------|---------|-------|
| 0 | 蓝（accent） | `oklch(0.54 0.19 262)` |
| 1 | 绿（ok） | `oklch(0.62 0.15 155)` |
| 2 | 琥珀（warn） | `oklch(0.65 0.15 80)` |
| 3 | 红（err） | `oklch(0.58 0.19 25)` |
| 4 | 青 | `oklch(0.60 0.14 195)` |
| 5 | 紫 | `oklch(0.55 0.18 300)` |
| 6 | 粉 | `oklch(0.62 0.16 350)` |
| 7 | 橄榄 | `oklch(0.60 0.14 120)` |
| 8 | 靛蓝 | `oklch(0.52 0.17 235)` |
| 9 | 棕橙 | `oklch(0.60 0.15 55)` |

**Dark** — 提亮以在深底上可读

| 索引 | 色调描述 | OKLCH |
|------|---------|-------|
| 0 | 蓝 | `oklch(0.72 0.18 262)` |
| 1 | 绿 | `oklch(0.74 0.16 155)` |
| 2 | 琥珀 | `oklch(0.78 0.15 80)` |
| 3 | 红 | `oklch(0.72 0.19 25)` |
| 4 | 青 | `oklch(0.74 0.14 195)` |
| 5 | 紫 | `oklch(0.70 0.18 300)` |
| 6 | 粉 | `oklch(0.74 0.16 350)` |
| 7 | 橄榄 | `oklch(0.72 0.14 120)` |
| 8 | 靛蓝 | `oklch(0.68 0.17 235)` |
| 9 | 棕橙 | `oklch(0.74 0.15 55)` |

**Solarized Light** — 暖调、略低饱和

| 索引 | 色调描述 | OKLCH |
|------|---------|-------|
| 0 | 金（accent） | `oklch(0.62 0.14 85)` |
| 1 | 绿 | `oklch(0.58 0.13 135)` |
| 2 | 橙 | `oklch(0.64 0.14 65)` |
| 3 | 红 | `oklch(0.56 0.17 25)` |
| 4 | 青 | `oklch(0.58 0.12 195)` |
| 5 | 紫 | `oklch(0.54 0.15 290)` |
| 6 | 洋红 | `oklch(0.58 0.14 345)` |
| 7 | 橄榄 | `oklch(0.56 0.12 120)` |
| 8 | 靛 | `oklch(0.52 0.14 230)` |
| 9 | 棕 | `oklch(0.58 0.13 50)` |

**Solarized Dark** — 冷调、适度饱和

| 索引 | 色调描述 | OKLCH |
|------|---------|-------|
| 0 | 蓝 | `oklch(0.70 0.14 235)` |
| 1 | 绿 | `oklch(0.72 0.14 140)` |
| 2 | 琥珀 | `oklch(0.76 0.14 75)` |
| 3 | 红 | `oklch(0.68 0.17 25)` |
| 4 | 青 | `oklch(0.72 0.12 195)` |
| 5 | 紫 | `oklch(0.66 0.15 290)` |
| 6 | 洋红 | `oklch(0.70 0.14 345)` |
| 7 | 橄榄 | `oklch(0.68 0.12 120)` |
| 8 | 靛 | `oklch(0.64 0.14 210)` |
| 9 | 棕 | `oklch(0.72 0.14 55)` |

## ECharts 按需注册模块

```ts
// src/components/charts/echarts.ts
import { use } from 'echarts/core'
import { CanvasRenderer } from 'echarts/renderers'
import { LineChart, PieChart, SankeyChart, BoxplotChart } from 'echarts/charts'
import {
  GridComponent,
  TooltipComponent,
  LegendComponent,
  DatasetComponent,
} from 'echarts/components'

use([
  CanvasRenderer,
  LineChart,
  PieChart,
  SankeyChart,
  BoxplotChart,
  GridComponent,
  TooltipComponent,
  LegendComponent,
  DatasetComponent,
])
```

## 组件接口

所有 5 个图表组件保持现有 props 接口不变。内部实现从 unovis 切换为 ECharts option 构建。图例交互（toggle/isolate）由组件自身管理 `hiddenKeys` 状态，通过修改 ECharts option 中的 series 可见性来实现，与现有行为一致。

### OverviewSpeedTimeline 实现方案

使用 `type: 'boxplot'` 系列。Y 轴为 category（各分组 label），X 轴为 value（速度值），水平布局。每个数据项编码为 boxplot 五值 `[min, min, median, max, max]`（将 Q1/median/Q3 都设为同一值即可退化为仅显示 min-max 范围的箱体）。

## 主题集成

ECharts 全局不设主题。每个图表组件读取 CSS 变量构建 option 时的颜色。当 `data-theme` 属性变化时，CSS 变量自动更新，ECharts 组件通过 `watch` 或 computed option 响应变化。

ECharts 的文字/轴线/网格等非系列元素颜色从现有 `--color-ink-muted`、`--color-ink-faint`、`--color-line` 等 token 读取，保持与 dashboard 整体风格一致。使用 `getComputedStyle` 在运行时读取 CSS 变量值。
