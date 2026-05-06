# 执行计划

1. 添加虚拟滚动依赖
   - 在 dashboard workspace 安装 `@tanstack/vue-virtual`。
   - 检查 `dashboard/package.json` 和 `dashboard/pnpm-lock.yaml` 只包含预期依赖更新。

2. 新建 SSE Events 虚拟列表组件
   - 新建 `dashboard/src/components/SSEEventsVirtualList.vue`。
   - 接收 `events: ParsedSSEEvent[]`。
   - 使用 `useVirtualizer` 绑定本组件的滚动容器。
   - 设置 `count`、`getScrollElement`、`getItemKey`、`estimateSize`、`overscan`。
   - 用 spacer + absolute row 布局渲染 `virtualRows`。

3. 迁移现有 event 卡片渲染
   - 将 `ResponseArtifactView.vue` 中 Events 分支的 `<article>` markup 移到 `SSEEventsVirtualList.vue`。
   - JSON event 继续渲染 `JsonArtifactViewer`。
   - Text event 继续渲染当前 `<pre>` 样式。
   - 保持序号、event 类型、JSON/Text badge 文案与当前样式。

4. 增加动态高度测量
   - 在虚拟行根节点设置 `data-index`。
   - 用模板 ref 收集当前虚拟行元素。
   - 在 mounted 和 updated 后调用 `rowVirtualizer.measureElement` 测量可见行。
   - 在 `events.length` 变化时调用 `rowVirtualizer.measure()` 并把滚动位置复位到顶部。

5. 接入响应详情 Events 子视图
   - 在 `ResponseArtifactView.vue` 引入 `SSEEventsVirtualList`。
   - 保留空状态 `没有可解析 event`。
   - 非空时用 `<SSEEventsVirtualList :events="sseEvents" />` 替换当前全量 `v-for`。

6. 验证类型与构建
   - 运行 `pnpm --dir dashboard type-check`。
   - 运行 `pnpm --dir dashboard build`。
   - 修复虚拟列表组件中的 Vue ref 类型、TanStack Virtual 类型和构建问题。

7. 手动检查
   - 使用大量 SSE events 的 artifact 打开请求详情。
   - 切换 `Events` 子视图，确认首屏只挂载可见数量的 event 卡片。
   - 滚动顶部、中部、底部，确认 event 序号、JSON/Text 展示和滚动高度正确。
