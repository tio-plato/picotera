# 执行计划

## 1. 新增 `dashboard/src/composables/useRequestDetailUiState.ts`

模块级声明 6 个 ref（`detailTab` / `requestBodyView` / `requestHeadersOpen` / `responseSubView` / `responseHeadersOpen` / `responseThinkingOpen`），导出一个返回这些 ref 的 `useRequestDetailUiState()`。`DetailTab` 类型从这里导出。`ResponseSubView` 仍从 `ResponseArtifactView.vue` 导入。

## 2. `RequestDetailsContent.vue`

- 删除：本地 `type DetailTab` 声明，以及 6 个 `ref(...)` 声明（`detailTab` / `requestBodyView` / `requestHeadersOpen` / `responseSubView` / `responseHeadersOpen` / `responseThinkingOpen`）。
- 删除：`watch(selectedId, () => { detailTab.value = 'overview' })`。
- 新增：`import { useRequestDetailUiState, type DetailTab } from '@/composables/useRequestDetailUiState'`，并在 `<script setup>` 中解构。
- 保留：`watch(detailTabs, …)`（用于 `logs` 选项消失时回退）。
- 保留模板里给两个 `RawArtifactView` 的 v-model：
  - 请求：`v-model:body-view="requestBodyView"` `v-model:headers-open="requestHeadersOpen"`。
  - 响应：`v-model:body-view="responseSubView"` `v-model:headers-open="responseHeadersOpen"` `v-model:thinking-open="responseThinkingOpen"`。

## 3. `RawArtifactView.vue`

- 已经用 `defineModel` 暴露 `bodyView` 与 `headersOpen`，不动。
- 已经删除 `watch(requestJsonBody, …)` 主动重置 watcher，不动。
- 已经保留 `watch(requestBodyOptions, …)` 的回退逻辑，不动。
- 已经把 Headers `<details>` 改为受控（`:open` + `@toggle`），不动。
- 新增 `defineModel<boolean>('thinkingOpen', { default: false })`，仅在 `kind === 'response'` 透传。
- 在 `kind === 'response'` 分支用 `v-model:sub-view` / `v-model:headers-open` / `v-model:thinking-open` 透传给 `ResponseArtifactView`。

## 4. `ResponseArtifactView.vue`

- 用 `defineModel` 暴露 `subView` / `headersOpen` / `thinkingOpen`。
- 删除 `watch(jsonBody, …)` 主动重置 watcher。
- 保留 `watch(subViewOptions, …, { immediate: true })`。
- Headers `<details>` 改为受控（`:open` + `@toggle`）。
- 「渲染」视图里的「思考过程」`<details>` 也改为受控，绑定 `thinkingOpen`。

## 5. 验证

按以下 case 手动验证：

- 切换主 Tab 到「原始响应」，在响应 body 切到「聚合」，再切换 span 卡片到另一个 upstream attempt → 主 Tab 仍为「原始响应」，body Tab 仍为「聚合」。
- 在「原始请求」展开 Headers，切换 span → Headers 仍展开。
- 在「原始响应 → 渲染」展开「思考过程」，切换 span / 列表 row / 路由 id → 「思考过程」仍展开。
- **关键新 case：** 在请求列表页打开一个 request 的侧栏，切到「原始响应 → 聚合」并展开 headers，点击列表里另一个 request → 侧栏 `:key` 变化、面板重挂，但「原始响应」、「聚合」、headers 展开都仍保持。
- **关键新 case：** 在 `/requests/:id` 全页路由上切到「原始请求 → JSON」，地址栏切到另一个 request id（路由实例同名异参，组件可能复用或重挂）→ 状态仍保持。
- 选中 meta（有 logs），切到「日志」，再点一个 upstream span（没有 logs）→ 主 Tab 回退到「概览」；再点回 meta → 主 Tab 不会自动跳回「日志」，保持「概览」（这是「合法选项消失则回退」的预期副作用）。
- 第一次加载一个 JSON 响应 → body Tab 默认是「JSON」。
- 第一次加载一个 SSE 响应 → body Tab 由 `subViewOptions` immediate watcher 修正为「JSON」不存在时的首选项（即 SSE 流程下没有 `json` 选项，会落到 `'raw'`，与现状一致）。
- 跑 `pnpm --dir dashboard type-check` 与 `pnpm --dir dashboard lint`。

## 6. 范围之外

- 不引入 Pinia / preferences store；不做跨会话持久化。
- 不重命名现有 prop / 事件（除 v-model props 外不动 API）。
- 不动 `LogsArtifactView`，它没有需要持久化的内部 Tab。
- 不动 `SidePanelHost.vue` 的 `:key="state.key"` 逻辑——它是按 request id keying 的合理设计（不同请求是不同的「面板内容」），我们的方案是绕开它，而不是改它。
