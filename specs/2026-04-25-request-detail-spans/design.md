# Design — Request Detail Spans

## 背景

请求记录已采用 meta/upstream 两段式模型：每次客户端请求生成一条 meta 请求（`type=0`，`span_id=自身 id`），每次上游尝试生成一条 upstream 请求（`type=1`，`span_id=meta.id`）。失败重试会让一个 meta 对应多条 upstream。

当前请求列表页面 `RequestsView.vue` 不区分 type，meta 与 upstream 行平铺展示，详情面板 `RequestDetailsPanel.vue` 仅显示单条记录，无法体现 span 关系。

## 目标

- 列表默认只显示 meta（`type=0`），通过显式切换查看 upstream 或全部。
- 点击一行进入侧滑面板，面板顶部以横排卡片同时展示该 meta 及其所有子请求（meta + upstream\[\]），点击卡片切换下方详情视图。

## 整体方案

1. **列表筛选**：在 `RequestsView` 顶部筛选区新增「类型」选择，默认 `meta`，可选 `upstream`、`all`。值为 `all` 时不传 `type`，否则传对应数字 (`0` / `1`)。
2. **后端按 span 列举**：新增 `ListRequestsBySpan` 查询，按 `span_id = ?` 取出 meta 自身（`id = span_id`）及其子 upstream，按 `created_at` 升序返回。新增 API `GET /requests/{id}/spans` 返回 `RequestView[]`。
3. **详情面板改造**：`RequestDetailsPanel` 接收 `requestId`，初始化时调用新接口拉一次 span 列表。顶部用横向滚动的卡片轨道渲染：第一张卡片是 meta，其余依序为 upstream（按时间排序，标号 `#1`、`#2`...）。点击卡片切换 `selectedId`，下半部分仍渲染当前选中条目的字段（沿用原 sections）。

## 接口与数据流

- 后端：增加 `ListRequestsBySpan` sqlc 查询；handler `handleListRequestsBySpan` 用 path `id`（即 meta id）去查 `WHERE span_id = $1 ORDER BY created_at ASC`；返回值复用 `RequestView`。
- 前端：详情面板初次加载只调用一次 `/requests/{id}/spans`，本地切换不再回查；如选中条目仍处于 `pending` 状态，提供刷新按钮重拉。

## UI 设计

- 卡片轨道：`flex gap-2 overflow-x-auto`，每张卡片 `min-w-44`，包含：
  - 角标：`META` 或 `#n`（upstream 索引）
  - 主行：provider 名（meta 阶段未确定时显示 `—`）
  - 副行：状态码徽章 + 耗时
  - 选中态：`ring` + `bg-accent-faint`
- 详情主体：保留现有 sections（基本信息 / 性能 / Token / 错误）。新增「Span 关系」字段：显示当前条目的 type、span_id、parent_span_id。
- meta 卡片始终位于首位，即使 upstream 为空也能显示。

## 关键决策

- **不引入树形组件**：层级仅两层（meta → upstream），横排卡片足够，无需 tree。
- **一次性拉取**：span 数量小（通常 1–3），整段一次拉完比为每张卡片单点查询更简单。
- **保留 SidePanel**：复用 `useSidePanel`，避免引入新的 Modal 组件。

## 兼容性

- 已存在的旧 `request` 数据（`type` 默认 1）会落入 upstream 视图；不在默认列表展示，符合迁移后语义。
- 列表 type 筛选的「all」与现有 `Type=-1` 默认值兼容，无需改后端。
