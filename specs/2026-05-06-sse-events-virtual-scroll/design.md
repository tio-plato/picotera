# SSE Events 虚拟滚动设计

## 范围

本次改动只涉及 Dashboard 请求详情里的响应 artifact `Events` 子视图。后端 artifact 结构、OpenAPI 契约、SSE 解析规则、聚合视图、Raw 视图和渲染视图不变。

当前 `dashboard/src/components/ResponseArtifactView.vue` 在 `Events` 视图中对 `sseEvents` 全量 `v-for`，并为每个 JSON event 创建一个 `JsonArtifactViewer`。大型 SSE 响应会一次性挂载大量 JSON tree editor，导致详情页切到 Events 时卡顿。

## 前端结构

新增 `dashboard/src/components/SSEEventsVirtualList.vue`，负责展示已解析的 `ParsedSSEEvent[]`。`ResponseArtifactView.vue` 继续负责：

- 判断响应是否为 SSE。
- 调用 `parseSSEEventsForDisplay(payload.body)`。
- 在 `Events` 子视图中处理空状态。
- 将非空事件数组传给 `SSEEventsVirtualList`。

`SSEEventsVirtualList.vue` 负责：

- 创建固定高度滚动容器，沿用当前 `max-h-[720px]`、`overflow-auto`、`pr-1` 的视觉密度。
- 使用 `@tanstack/vue-virtual` 的 `useVirtualizer` 计算可见行。
- 只渲染 `virtualizer.getVirtualItems()` 返回的可见 event 和 overscan event。
- 对 JSON event 继续使用 `JsonArtifactViewer`。
- 对非 JSON event 继续使用当前 `<pre>` 代码块样式。
- 保持当前 event 卡片头部信息：序号、event 类型、JSON/Text 标记。

这次不引入 event 选中态或左右分栏。当前已实现的是顺序卡片列表，虚拟化会保留这个交互模型，避免扩大改动面。

## 虚拟滚动实现

依赖使用 `@tanstack/vue-virtual`。实现采用容器级 row virtualizer：

```ts
const parentRef = ref<HTMLElement | null>(null)
const rowVirtualizer = useVirtualizer({
  count: () => props.events.length,
  getScrollElement: () => parentRef.value,
  estimateSize: estimateEventHeight,
  getItemKey: (index) => props.events[index]?.index ?? index,
  overscan: 4,
})
```

列表 DOM 结构使用 TanStack Virtual 的标准模式：

- 外层容器滚动。
- 内层 spacer 高度设置为 `rowVirtualizer.getTotalSize()`。
- 可见行绝对定位，通过 `transform: translateY(...)` 放到对应位置。

SSE event 的高度不固定，JSON tree 展开、字符串长度和非 JSON 文本长度都会影响行高。组件必须启用动态测量：

- 每个虚拟行根节点设置 `data-index`。
- 虚拟行根节点挂载后调用 `rowVirtualizer.measureElement(el)`。
- Vue 更新后重新测量当前可见行，保证 JSONEditor 挂载完成后的高度被记录。
- `estimateEventHeight` 根据事件类型给出稳定估算：JSON event 使用较高估算值，文本 event 根据 `data.length` 给出受上限约束的估算值。

滚动容器使用 `contain: strict` 或等价内联样式限制布局影响，减少大量行尺寸变化对外层页面的重排成本。

## 数据与解析

`parseSSEEventsForDisplay` 保持当前返回结构：

```ts
export interface ParsedSSEEvent {
  index: number
  event: string | null
  data: string
  json: unknown | null
}
```

本次计划不修改解析宽容度：

- 仍以空行分隔 event。
- 仍读取 `event:` 和 `data:`。
- 仍跳过 `[DONE]`。
- 仍对 `data` 做严格 `JSON.parse`，失败即作为 Text 展示。

## 依赖

在 `dashboard/package.json` 增加 `@tanstack/vue-virtual`，并用 pnpm 更新锁文件。该包是 TanStack Virtual v3 的 Vue adapter，底层依赖 `@tanstack/virtual-core`。

当前 npm latest 为 `3.13.24`。实现时使用 pnpm 解析的稳定 v3 版本，并把 `package.json` 约束写为 `^3.13.24`。

## 样式

视觉样式继续使用 Dashboard 现有 Tailwind token：

- 卡片：`rounded-md border border-line-soft bg-surface-0 overflow-hidden`
- header：`border-b border-line-soft bg-surface-50 px-3 py-2`
- meta：`font-mono text-xs` / `font-mono text-2xs`
- 内容区：`p-3`

虚拟化所需的定位样式只用于布局，不引入新 UI primitive，不新增颜色 token。

## 验证

实现后运行：

- `pnpm --dir dashboard type-check`
- `pnpm --dir dashboard build`

手动验证：

- 打开包含大量 SSE events 的响应 artifact。
- 切换到 `Events` 子视图时页面保持可交互。
- 滚动到顶部、中部、底部时 event 序号连续且内容正确。
- JSON event 只在进入可见区域时创建 JSON tree。
- 非 JSON event 仍以代码块显示。
