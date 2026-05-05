# 请求详情 Deeplink 设计

## 目标

请求详情支持 URL 直达。用户在 `/requests` 列表中点击一条请求时，地址栏使用 `router.replace` 更新为 `/requests/:requestId`，当前页面保持列表布局并打开右侧或弹窗式详情面板。关闭详情后，地址栏使用 `router.replace` 还原为 `/requests`，并保留当前筛选 query。

用户直接访问 `/requests/:requestId` 时，页面展示同一份请求详情内容的全屏版本。全屏版本不渲染请求列表，也不依赖侧边面板状态。

## 路由

在 `dashboard/src/router/index.ts` 增加路由：

- `path: '/requests/:requestId'`
- `name: 'requestDetail'`
- `component: () => import('@/views/RequestDetailView.vue')`

`/requests` 保持现有 `RequestsView.vue`。导航栏高亮逻辑需要把 `requestDetail` 视为请求模块，因此 `AppSidebar.vue` 不能只用 `route.name === item.name` 判断 active，需要为请求详情路由映射到 `requests`。

`App.vue` 的页面标题映射增加 `requestDetail`，标题沿用 `请求`，提示文案表达当前是请求详情。

## 组件结构

现有 `RequestDetailsPanel.vue` 同时负责加载详情内容和渲染 `SidePanel` 外壳。为复用同一份内容，拆成两个层次：

- `RequestDetailsContent.vue`：负责请求 span 加载、meta/upstream 切换、tab 切换、概览、原始请求、原始响应和日志展示。接收 `requestId` 和可选 `providers`。
- `RequestDetailsPanel.vue`：保留 `SidePanel` 外壳，只包裹 `RequestDetailsContent.vue`，继续向 `SidePanelHost` 暴露 `close` 事件。
- `RequestDetailView.vue`：全屏详情页面，复用 `RequestDetailsContent.vue`，用 dashboard 的普通内容区域渲染一个全宽详情面板，不使用 `SidePanel`。

`RequestDetailsContent.vue` 使用现有 `useApi` 请求 `/api/picotera/requests/{id}/spans`，不新增 API。全屏页也通过 `useProvidersMap` 获取 providers，让渠道名称与列表弹窗一致。加载失败时在内容区域显示 `StateText` 或错误条，不把错误埋在 side panel 专有 slot 中。

## URL 同步行为

`RequestsView.vue` 点击行时执行单一入口函数：

1. 如果当前已打开同一请求，关闭面板并把 URL 还原到 `{ name: 'requests', query: route.query }`。
2. 否则用 `panel.open` 打开 `RequestDetailsPanel`。
3. 同步地址到 `{ name: 'requestDetail', params: { requestId: r.id }, query: route.query }`，使用 `router.replace`。

面板关闭时要还原 URL。由于 `SidePanelHost` 当前直接调用全局 `close`，`RequestsView.vue` 需要监听侧边面板状态变化：当 active key 从当前请求详情变成非请求详情，并且当前路由是 `requestDetail`，就 `router.replace` 回 `requests` 并保留 query。

`RequestsView.vue` 只作为 `/requests` 的组件运行，不承担直接访问 `/requests/:requestId` 的全屏渲染。这样避免在同一个组件里维护“列表内弹窗”和“全屏详情”两套布局状态。

## Query 保留

现有请求列表通过 `parentSpanId` query 与筛选同步。点击详情时保留当前 query，关闭详情时也保留 query。例如：

- `/requests?parentSpanId=abc` 点击 `req_1` 后变为 `/requests/req_1?parentSpanId=abc`
- 关闭后还原为 `/requests?parentSpanId=abc`

`syncParentSpanFilterToQuery` 当前使用 `{ name: 'requests', query }`，它会在详情路径下把用户带回列表。实现时需要改成保留当前 route name 和 params：在 `/requests` 上更新 `/requests` 的 query，在 `/requests/:requestId` 上更新 `/requests/:requestId` 的 query。

## 全屏详情页面

`RequestDetailView.vue` 从 `route.params.requestId` 严格读取字符串参数。参数缺失或不是字符串时显示错误状态，不做容错转换。页面提供一个返回按钮，跳回 `{ name: 'requests', query: route.query }`，保留筛选上下文。

全屏页面布局使用现有设计系统：`DataCard` 或等价的非嵌套表面，头部包含请求 ID 和返回操作，正文复用 `RequestDetailsContent.vue`。不引入第三方 UI 库，不新增动画。

## API

不新增或修改后端 API。现有接口已经覆盖详情加载：

- `GET /api/picotera/requests/{id}/spans`

因此不需要更新 `openapi.yaml` 或 dashboard OpenAPI 类型。
