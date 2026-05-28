# TTFT 平均时间折线图 & Decode 速度 Timeline 图

## 需求

在概览页面的"速度统计"一栏下方，新增两个图表：

1. **TTFT 平均时间折线图**：使用与现有速度统计相同的 `OverviewLineChart` 组件，显示每个时间桶内的平均 TTFT（Time To First Token）时间，单位为毫秒。分组维度与速度统计一致（共用 `speedDimension`）。

2. **Decode 速度 Timeline 图**：使用 unovis 的 `VisTimeline` 组件，每个分组（与上方折线图分组一致）显示一行，横轴为速度（tok/s），每行用一条水平条显示该分组在所有时间桶中的 decode 速度平均值的最小值到最大值区间。数据来源是现有 decode 速度折线图中的各时间桶平均值，只需在这些平均值中取 min/max。
