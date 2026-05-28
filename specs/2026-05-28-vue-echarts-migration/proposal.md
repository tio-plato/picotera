# Proposal: vue-echarts 替换 unovis 图表

## 目标

将 dashboard 中现有的 5 个基于 `@unovis/vue` + `@unovis/ts` 的图表组件全部迁移到 `vue-echarts`（Apache ECharts 的 Vue 封装）。

## 色盘要求

为每个主题（light / dark / solarized-light / solarized-dark）新设计一套 **8 色以上**的色盘，专供图表系列颜色使用。新色盘通过 CSS 变量暴露，图表组件通过 `colors.ts` 读取，与现有主题切换机制集成。

## 现有图表组件

| 组件 | 图表类型 | 位置 |
|------|---------|------|
| `OverviewAreaStack` | 堆叠面积图 | `src/components/charts/OverviewAreaStack.vue` |
| `OverviewLineChart` | 多系列折线图 | `src/components/charts/OverviewLineChart.vue` |
| `OverviewDonut` | 环形图 | `src/components/charts/OverviewDonut.vue` |
| `OverviewSankey` | 桑基图 | `src/components/charts/OverviewSankey.vue` |
| `OverviewSpeedTimeline` | 水平范围条形图 | `src/components/charts/OverviewSpeedTimeline.vue` |

## 保留的交互行为

- 系列切换（点击图例 tag 切换单个系列可见性）
- 系列隔离（右键点击图例 tag，仅显示该系列）
- 自定义 tooltip（与现有格式一致）
- 颜色分配（按系列索引从色盘取色，索引稳定）

## 约束

- 不引入额外 UI 库，图表外的 legend tag 仍使用本地 `Tag` 组件
- Props 接口保持不变，`OverviewView.vue` 不需要修改数据层代码
- 移除所有 unovis 依赖
