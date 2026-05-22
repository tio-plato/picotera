# 执行计划

## 1. `RequestDetailsContent.vue`

- 在 `<script setup>` 中新增 4 个 ref：
  - `requestBodyView = ref<'raw' | 'json'>('json')`
  - `requestHeadersOpen = ref(false)`
  - `responseSubView = ref<'raw' | 'json' | 'aggregated' | 'events' | 'rendered'>('json')`
  - `responseHeadersOpen = ref(false)`
  - 抽出 `ResponseSubView` 类型并 `export` 或就近声明，供下游导入复用。
- 删除 `watch(selectedId, () => { detailTab.value = 'overview' })`。
- 保留 `watch(detailTabs, …)`（用于 `logs` 选项消失时回退）。
- 在模板中给两个 `RawArtifactView` 加双向绑定：
  - 请求：`v-model:body-view="requestBodyView"` `v-model:headers-open="requestHeadersOpen"`。
  - 响应：`v-model:body-view="responseSubView"` `v-model:headers-open="responseHeadersOpen"`。

## 2. `RawArtifactView.vue`

- 用 `defineModel` 暴露两条状态：
  - `const bodyView = defineModel<'raw' | 'json' | 'aggregated' | 'events' | 'rendered'>('bodyView', { required: true })`
  - `const headersOpen = defineModel<boolean>('headersOpen', { required: true })`
  - `bodyView` 用宽并集类型，因为 `kind === 'response'` 时它会承载 5 个值；`kind === 'request'` 时父组件只会传入 `'raw' | 'json'`，由父组件做类型约束。
- 删除本地 `const requestBodyView = ref<'raw' | 'json'>('json')`，模板里的 `v-model="requestBodyView"` 改为 `v-model="bodyView"`，同时类型断言收窄到 `'raw' | 'json'`（这一段只走 `kind === 'request'` 分支）。
- 删除 `watch(requestJsonBody, (parsed) => { requestBodyView.value = parsed.ok ? 'json' : 'raw' })`。
- 保留 `watch(requestBodyOptions, …)` 的回退逻辑，把里面对 `requestBodyView` 的赋值改为对 `bodyView` 的赋值；判断条件按 `kind === 'request'` 守住，避免误伤 response 分支。
- Headers `<details>` 改为受控：
  ```html
  <details
    :open="headersOpen"
    @toggle="headersOpen = ($event.currentTarget as HTMLDetailsElement).open"
    class="group flex flex-col gap-2"
  >
  ```
- `kind === 'response'` 分支调用 `ResponseArtifactView` 时透传 v-model：
  ```html
  <ResponseArtifactView
    v-model:sub-view="bodyView"
    v-model:headers-open="headersOpen"
    :payload="payload"
    :url="url"
    :request-id="requestId"
  />
  ```

## 3. `ResponseArtifactView.vue`

- 用 `defineModel` 暴露：
  - `const subView = defineModel<SubView>('subView', { required: true })`
  - `const headersOpen = defineModel<boolean>('headersOpen', { required: true })`
- 删除本地 `const subView = ref<SubView>('raw')`。
- 删除 `watch(jsonBody, (parsed) => { if (!isSSE.value && parsed.ok) subView.value = 'json' })`。
- 保留 `watch(subViewOptions, …, { immediate: true })`（首屏 immediate 阶段会基于父组件给的初值 `'json'` 自动修正为合法选项；逻辑无需改动）。
- Headers `<details>` 同样改为受控（`:open` + `@toggle`）。

## 4. 验证

按以下 case 手动验证：

- 切换主 Tab 到「原始响应」，在响应 body 切到「聚合」，再切换 span 卡片到另一个 upstream attempt → 主 Tab 仍为「原始响应」，body Tab 仍为「聚合」。
- 在「原始请求」展开 Headers，切换 span → Headers 仍展开。
- 选中 meta（有 logs），切到「日志」，再点一个 upstream span（没有 logs）→ 主 Tab 回退到「概览」；再点回 meta → 主 Tab 不会自动跳回「日志」，保持「概览」（这是「合法选项消失则回退」的预期副作用）。
- 第一次加载一个 JSON 响应 → body Tab 默认是「JSON」。
- 第一次加载一个 SSE 响应 → body Tab 由 `subViewOptions` immediate watcher 修正为「JSON」不存在时的首选项（即 SSE 流程下没有 `json` 选项，会落到 `'raw'`，与现状一致）。
- 通过路由切换到另一个 request id（`replaceDetailUrl` 路径不变，组件复用） → 上面所有视觉状态保持不变。
- 跑 `pnpm --dir dashboard type-check` 与 `pnpm --dir dashboard lint`。

## 5. 范围之外

- 不引入 Pinia / preferences store；不做跨会话持久化。
- 不重命名现有 prop / 事件（除新增 model props 外不动 API）。
- 不动 `LogsArtifactView`，它没有需要持久化的内部 Tab。
