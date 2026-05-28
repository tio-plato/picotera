# 设计

## 1. TTFT 平均时间折线图

### 后端

现有 `request_speed_hourly` 连续聚合仅存储 `prefill_time_sum`（TTFT 毫秒总和）和 `prefill_token_sum`（输入 token 总和），缺少请求计数字段，无法计算平均 TTFT。

新增一个 goose migration（028），删除并重建 `request_speed_hourly`，增加 `prefill_request_count` 列：

```sql
COUNT(CASE WHEN input_tokens >= 50 AND ttft_ms >= 500 THEN 1 END) AS prefill_request_count
```

修改 `ListOverviewSpeedSeries` 查询，新增返回字段：

```sql
COALESCE((SUM(prefill_time_sum) / NULLIF(SUM(prefill_request_count), 0))::float8, 0)::float8 AS avg_ttft
```

`avg_ttft` 单位为毫秒。

后端 handler（`handle_overview.go`）在组装 points 时新增 `avgTtft` metric，逻辑与 `prefillSpeed` / `decodeSpeed` 一致。

### 前端

在 `OverviewView.vue` 中新增 `seriesAvgTtft` computed，从 `speedSeriesData` 的 points 中过滤 `metric === 'avgTtft'`。

在模板中速度统计两张折线图下方，新增一个 `DataCard`，内部放 `OverviewLineChart`，`valueFormat` 格式化为毫秒（如 `123 ms`、`1.2 s`）。

## 2. Decode 速度 Timeline 图

### 前端

数据完全来自现有 `seriesDecodeSpeed` computed（已有的 decode 速度折线图数据），不需要新的后端查询。

在 `OverviewView.vue` 中新增 computed，对每个 group 遍历所有 bucket 的 decode speed 值，取 min/max：

```ts
// 每个 group 一条 timeline datum
interface TimelineDatum {
  row: string       // group label
  x: number         // min speed
  length: number    // max - min
  color: string     // group color
}
```

新建 `OverviewSpeedTimeline.vue` 组件，使用 `VisXYContainer` + `VisTimeline` + `VisAxis`：
- X 轴：速度（tok/s）
- 每行一个分组，显示 row label
- 每行一条水平条，从 min 到 max
- 配色与折线图中对应分组一致

在模板中 TTFT 折线图下方放置该 Timeline 图。

## 布局

速度统计区域最终布局（从上到下）：

1. 维度选择器（已有）
2. Prefill 速度 + Decode 速度折线图（已有，2 列）
3. TTFT 平均时间折线图（新增，单列全宽）
4. Decode 速度范围 Timeline 图（新增，单列全宽）
