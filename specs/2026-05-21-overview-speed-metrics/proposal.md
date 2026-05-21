# Overview Speed Metrics

在概览页面新增输出速度统计功能，包含两个指标：

- **Prefill 速度**：输入 tokens / TTFT（tokens/秒）
- **Decode 速度**：输出 tokens / (总时间 - TTFT)（tokens/秒）

## 过滤规则

- 输入 tokens < 200 的请求不参与 prefill 速度计算
- 输出 tokens < 200 的请求不参与 decode 速度计算
- TTFT < 2 秒的请求不参与 prefill 速度计算
- (总时间 - TTFT) < 2 秒的请求不参与 decode 速度计算

## 聚合方式

- 物化视图存 SUM（分子分母），查询时 SUM 再除，得到加权平均
- 权重为各请求的对应时间（ttft 或 decode 时间），时间长的请求贡献更大
- 维度与现有概览页 tokens/费用指标一致（按 model、upstreamModel、provider 等分组，按时间桶聚合）

## 可视化

- 在概览页面新增两个折线图（非堆叠面积图）
- 一个按模型（model）维度聚合
- 一个按上游（upstreamModel）维度聚合
- 使用 Unovis `VisLine` 绘制

## 计算方式

- 新建 `request_speed_hourly` continuous aggregate，按 1 小时分桶
- 物化 prefill/decode 的分子（token SUM）和分母（time SUM），CASE WHEN 含阈值过滤
- 查询时 `SUM(token_sum) / (SUM(time_sum) / 1000.0)` 得到加权平均 tokens/sec
- WHERE 过滤 + GROUP BY 维度分组 + HAVING 排除无数据的桶
