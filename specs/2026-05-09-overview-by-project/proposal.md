# 概览页按项目维度区分

给概览页面分布统计、用量统计加上按项目维度区分的功能。

## 用户澄清后的细节

- 顶部控件加「项目」筛选下拉，与现有 密钥/请求模型/上游模型/渠道 筛选并列。
- 分布统计 dimension 增加「项目」选项（环形图 Token 分布 / 费用分布 按项目切分）。
- 用量统计 dimension 增加「项目」选项（堆叠区域图 Token / 费用 / 请求数 / 追踪数 按项目切分）。
- Sankey 图层级中加入项目层：
  - `tokensIn` / `costIn`：`provider → upstreamModel → model → apiKey → project`
  - `tokensOut` / `costOut`：`project → apiKey → model → upstreamModel → provider`
- 后端通过修改连续聚合 `request_overview_hourly`，把 `project_id` 加入 SELECT 与 GROUP BY，作为新维度列。DROP & RECREATE，不保留旧物化数据（policy 会按 35 天 lookback 重新物化）。
- 旧数据 `project_id IS NULL` → cagg 中保留 NULL → 维度 CASE 输出空字符串 → 前端沿用现有「未知」标签。
- 严格输入：`projectId` 为 0 视为不过滤，其它非法值由 Huma `minimum:"1"` 拒绝。
- 不引入额外的 totalProjects 卡片，不动现有 4 个 bento 卡。
