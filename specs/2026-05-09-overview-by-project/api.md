# 概览页按项目维度区分 — API

无新增 operation，三个现有 operation 接收新参数、返回新字段。

## 共同变更：`OverviewCommonRequest`

新增 query 参数：

| 字段 | 类型 | 约束 | 说明 |
|---|---|---|---|
| `projectId` | `int32` | `omitempty` `minimum=1` | 按项目过滤；缺省或 0 不过滤 |

应用于：
- `GET /api/picotera/overview/summary`
- `GET /api/picotera/overview/distribution`
- `GET /api/picotera/overview/series`

## `GET /overview/summary`

响应体 `OverviewSummaryView.breakdown[]` 中每个 `OverviewBreakdownRowView` 新增字段：

| 字段 | 类型 | 说明 |
|---|---|---|
| `projectId` | `int32` | 项目 id；`0` 表示未关联项目 |

旧字段不变。

## `GET /overview/distribution`

`dimension` query 参数 enum 扩展：

```
apiKey, model, upstreamModel, provider, project
```

当 `dimension=project` 时，`OverviewDistributionRowView.key` 是项目 id 的字符串形式（例如 `"3"`），未关联项目为空字符串 `""`。前端通过 `dimensionLabel('project', key)` 转成项目名 / 「未关联」。

## `GET /overview/series`

`dimension` query 参数 enum 扩展：

```
none, apiKey, model, upstreamModel, provider, project
```

当 `dimension=project` 时，`OverviewSeriesGroupView.key` 与 `OverviewSeriesPointView.groupKey` 是项目 id 的字符串形式。

## 错误行为

- `projectId < 1` 但显式传入：Huma 校验拒绝 400。
- `projectId` 指向不存在的项目：返回空数据集（与现有 `apiKeyId` / `providerId` 行为一致，不返回 404）。
