# API: Overview Speed Metrics

## 无新增 Endpoint

速度数据嵌入现有 `GET /api/picotera/overview/series` 响应。

## 新增 metric 值

`OverviewSeriesPointView.metric` 字段（string 类型）新增两个值：

| metric | 含义 | 单位 |
|--------|------|------|
| `prefillSpeed` | Prefill 速度（输入 tokens / TTFT） | tokens/sec |
| `decodeSpeed` | Decode 速度（输出 tokens / decode 时间） | tokens/sec |

与现有 metric 值并列：

| 现有 metric | 含义 |
|-------------|------|
| `tokens` | Token 数量 |
| `requests` | 请求数 |
| `traces` | 追踪数 |
| `cost` | 费用 |

## 响应示例

```json
{
  "points": [
    {
      "metric": "prefillSpeed",
      "bucketAt": "2026-05-21T08:00:00Z",
      "groupKey": "claude-sonnet-4-20250514",
      "value": 1523.45,
      "currency": ""
    },
    {
      "metric": "decodeSpeed",
      "bucketAt": "2026-05-21T08:00:00Z",
      "groupKey": "claude-sonnet-4-20250514",
      "value": 89.12,
      "currency": ""
    }
  ]
}
```

## 聚合逻辑

- 物化视图存 SUM（分子分母），查询时 `SUM(token_sum) / (SUM(time_sum) / 1000.0)` 得到加权平均 tokens/sec
- 权重为各请求的对应时间，时间越长的请求贡献越大
- 过滤条件（per-request，不参与计算）：
  - Prefill：`input_tokens >= 200 AND ttft_ms >= 2000`
  - Decode：`output_tokens >= 200 AND (time_spent_ms - ttft_ms) >= 2000`
- 某桶/分组无满足条件的请求时，不出对应 metric 的点（前端显示为空）

## 维度支持

`dimension` 参数支持所有现有值：`none`, `model`, `upstreamModel`, `provider`, `apiKey`, `project`。

- `dimension = 'none'` 时 group_key 为 `''`（全局聚合）
- 其它维度按对应字段分组

## 过滤器

复用现有 `OverviewCommonRequest` 的全部过滤器：`range`, `apiKeyId`, `model`, `upstreamModel`, `providerId`, `projectId`。
