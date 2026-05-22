# 设计

## 现状

`RequestDetailsContent.vue` 是请求详情视图的核心组件，被两条入口共用：

- `RequestDetailView`（`/requests/:requestId` 全页路由）。
- `RequestsView` → `useSidePanel().open(RequestDetailsPanel, …, { key: 'request:<id>' })` → `SidePanelHost` 用 `<component :is="state.component" :key="state.key">` 渲染。**`state.key` 含 request id，切换不同的 request 会换 key，整个 panel 实例被销毁重挂。**

它内部维护：

- `selectedId` — 当前选中的 span（meta 或某次 upstream attempt）。
- `detailTab` — 主 Tab，`overview | request | response | logs`。

切换 `selectedId` 时，下方 Tab 内容通过 `v-else-if` 渲染：

- `detailTab === 'request'` → `RawArtifactView (kind='request')`。
- `detailTab === 'response'` → `RawArtifactView (kind='response')` → 内部又渲染 `ResponseArtifactView`。
- `detailTab === 'logs'` → `LogsArtifactView`。

`RawArtifactView` 维护本地 `requestBodyView`（`raw | json`），`ResponseArtifactView` 维护本地 `subView`（`raw | json | aggregated | events | rendered`）。两者各自的 Headers 用原生 `<details>` 元素，展开态由 DOM 自己管理。

## 状态丢失的根因

四类原因导致用户选择被重置：

1. **`watch(selectedId, () => { detailTab.value = 'overview' })`** — 切换 span 时强制把主 Tab 重置为「概览」。
2. **`watch(requestJsonBody, …)` 与 `watch(jsonBody, …)`** — 每当 artifact 数据变化（典型场景：切换 span 后 `useArtifact` 拿到新 payload），就根据是否能解析为 JSON 把 body Tab 重置为 `json` 或 `raw`。
3. **`<details>` 是不受控元素**，组件被 `v-else-if` 卸载再挂载后，DOM 重新创建，展开态丢失。即便不卸载，跨 span 切换也无法记住「上一个 span 的 headers 是展开的」。
4. **跨请求切换会换 `RequestDetailsPanel` 的 `:key`，整个 `RequestDetailsContent` 实例都被替换**。任何放在组件实例内的 `ref`（包括 `defineModel` 解出来的 ref）都活不过这次重挂。

## 解决方案

需要两层动作：

### A. 视觉状态提到组件实例之上：模块级 composable

新建 `dashboard/src/composables/useRequestDetailUiState.ts`，在**模块作用域**声明 ref，所有 `RequestDetailsContent` 实例共享同一份引用：

```ts
import { ref } from 'vue'
import type { SubView as ResponseSubView } from '@/components/ResponseArtifactView.vue'

export type DetailTab = 'overview' | 'request' | 'response' | 'logs'

const detailTab = ref<DetailTab>('overview')
const requestBodyView = ref<'raw' | 'json'>('json')
const requestHeadersOpen = ref(false)
const responseSubView = ref<ResponseSubView>('json')
const responseHeadersOpen = ref(false)
const responseThinkingOpen = ref(false)

export function useRequestDetailUiState() {
  return {
    detailTab,
    requestBodyView,
    requestHeadersOpen,
    responseSubView,
    responseHeadersOpen,
    responseThinkingOpen,
  }
}
```

`RequestDetailsContent` 不再用 `ref(...)` 声明这 5 个状态，而是从 composable 里解构。模块只会被求值一次，跨 panel key 切换、跨路由跳转都仍是同一份 ref。

`RawArtifactView` 和 `ResponseArtifactView` 仍然通过 `defineModel`（`bodyView` / `headersOpen` / `subView`）从父组件接 v-model，因此组件 API 维持不变，只是父端的来源换成模块级引用。

不进 Pinia / preferences store / localStorage：

- 用户明确说跨会话持久化不在范围内。
- 这是纯视觉的临时偏好，不值得污染全局 store。
- 模块级 ref 已经满足「整个 SPA 生命周期内一致」的需求。

### B. 重置语义的取舍

需要明确分清两种 watcher，避免把「合理回退」一起删掉：

- **保留**（防御性回退，处理「当前选中项已不在可选项里」的情况）：
  - `RequestDetailsContent` 中 `watch(detailTabs, …)`：当 meta 不再被选中、`logs` 选项消失时，把主 Tab 拉回 `overview`。
  - `RawArtifactView` 中 `watch(requestBodyOptions, …)`：当 body 变为不可解析时，`JSON` 选项消失，把当前选中拉回第一个可用选项。
  - `ResponseArtifactView` 中 `watch(subViewOptions, …, { immediate: true })`：同上，优先回退到 `json`，否则 `raw`。
- **删除**（破坏用户选择的「主动重置」）：
  - `RequestDetailsContent`: `watch(selectedId, () => { detailTab.value = 'overview' })`。
  - `RawArtifactView`: `watch(requestJsonBody, (parsed) => { requestBodyView.value = parsed.ok ? 'json' : 'raw' })`。
  - `ResponseArtifactView`: `watch(jsonBody, (parsed) => { if (!isSSE.value && parsed.ok) subView.value = 'json' })`。

把默认值改为 `'json'` 后，仍由「防御性回退」watcher 在 immediate 阶段自动修正为合法选项，因此 binary / 非 JSON / SSE 的首屏体验与现状一致。

### C. `<details>` 改为受控

请求 / 响应的 Headers `<details>`、以及响应「渲染」视图里的「思考过程」`<details>` 都改为受控：`:open="open" @toggle="open = ($event.currentTarget as HTMLDetailsElement).open"`，让展开态走 Vue 响应式，从而被 composable 持有。

`thinkingOpen` 只在 `kind === 'response'` 分支有意义。`RawArtifactView` 把它声明为 `defineModel<boolean>('thinkingOpen', { default: false })`（非 required），仅在透传给 `ResponseArtifactView` 时使用；`kind === 'request'` 父端不传，按 default false 走，对请求视图无影响。

## 不引入新依赖

仅使用 Vue 自带的 `defineModel` + 模块级 `ref`。无第三方库，无 Pinia store 新增。
