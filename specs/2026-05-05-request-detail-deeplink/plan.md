# 请求详情 Deeplink 执行计划

## 1. 拆分详情组件

1. 新建 `dashboard/src/components/RequestDetailsContent.vue`。
2. 从 `RequestDetailsPanel.vue` 移动详情加载、span 排序、tab 状态、格式化函数和正文模板到 `RequestDetailsContent.vue`。
3. 让 `RequestDetailsContent.vue` 接收 `requestId: string` 与 `providers?: ProviderView[]`。
4. 在内容组件内部处理加载中、空结果和错误状态，避免依赖 `SidePanel` 的 `#error` slot。
5. 修改 `RequestDetailsPanel.vue`，只渲染：
   - `SidePanel title="请求详情"`
   - `kicker=requestId`
   - `RequestDetailsContent`
   - `close` 事件透传

## 2. 增加全屏详情视图

1. 新建 `dashboard/src/views/RequestDetailView.vue`。
2. 使用 `useRoute` 读取 `requestId`，只接受字符串参数。
3. 使用 `useProvidersMap` 加载 providers。
4. 渲染全屏详情布局，正文复用 `RequestDetailsContent.vue`。
5. 增加返回按钮，执行 `router.replace({ name: 'requests', query: route.query })`。

## 3. 配置路由和应用 chrome

1. 在 `dashboard/src/router/index.ts` 增加 `/requests/:requestId` 路由，命名为 `requestDetail`。
2. 在 `dashboard/src/App.vue` 增加 `requestDetail` 的页面标题和提示文案。
3. 修改 `dashboard/src/components/AppSidebar.vue` 的 active 判断，把 `requestDetail` 映射为 `requests`，确保请求导航项高亮。

## 4. 请求列表 URL 同步

1. 修改 `RequestsView.vue` 的 `openDetails`：
   - 已打开同一请求时关闭面板并 replace 回 `requests`。
   - 打开新请求时打开 `RequestDetailsPanel` 并 replace 到 `requestDetail`。
   - 两种路径都保留当前 query。
2. 修改 `rowSelected`，继续使用 `request:${id}` key。
3. 增加对 `panel.activeKey` 或 `panel.state` 的 watcher：
   - 当当前路由是 `requestDetail` 且面板不再是该 request key 时，replace 回 `requests`。
   - watcher 只响应面板关闭，不处理全屏直达页面。
4. 修改 `syncParentSpanFilterToQuery`，保留当前 route name 和 params，避免在详情路径下更新 query 时跳回列表。

## 5. 验证

1. 运行 `pnpm --dir dashboard type-check`。
2. 运行 `pnpm --dir dashboard lint`。
3. 手动验证：
   - 从 `/requests` 点击一行，面板打开，URL 变为 `/requests/:requestId`。
   - 关闭面板，URL 回到 `/requests`。
   - 带 `parentSpanId` query 点击和关闭时 query 保留。
   - 直接访问 `/requests/:requestId` 展示全屏详情。
   - 全屏详情返回按钮回到 `/requests` 并保留 query。
   - 请求导航项在 `/requests/:requestId` 下保持高亮。
