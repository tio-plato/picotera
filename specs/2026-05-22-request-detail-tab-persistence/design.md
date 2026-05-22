# 设计

## 现状

`RequestDetailsContent.vue` 是请求详情视图的核心组件，被 `RequestDetailView`（路由页）和 `RequestDetailsPanel`（侧栏）共用。它内部维护：

- `selectedId` — 当前选中的 span（meta 或某次 upstream attempt）。
- `detailTab` — 主 Tab，`overview | request | response | logs`。

切换 `selectedId` 时，下方 Tab 内容通过 `v-else-if` 渲染：

- `detailTab === 'request'` → `RawArtifactView (kind='request')`
- `detailTab === 'response'` → `RawArtifactView (kind='response')` → 内部又渲染 `ResponseArtifactView`
- `detailTab === 'logs'` → `LogsArtifactView`

`RawArtifactView` 维护本地 `requestBodyView`（`raw | json`），`ResponseArtifactView` 维护本地 `subView`（`raw | json | aggregated | events | rendered`）。两者各自的 Headers 用原生 `<details>` 元素，展开态由 DOM 自己管理。

## 状态丢失的根因

三类原因导致用户选择被重置：

1. **`watch(selectedId, () => { detailTab.value = 'overview' })`** — 切换 span 时强制把主 Tab 重置为「概览」。
2. **`watch(requestJsonBody, …)` 与 `watch(jsonBody, …)`** — 每当 artifact 数据变化（典型场景：切换 span 后 `useArtifact` 拿到新 payload），就根据是否能解析为 JSON 把 body Tab 重置为 `json` 或 `raw`。
3. **`<details>` 是不受控元素**，组件被 `v-else-if` 卸载再挂载后，DOM 重新创建，展开态丢失。即便不卸载，跨 span 切换也无法记住「上一个 span 的 headers 是展开的」。

## 解决方案

把这些视觉状态**提升到 `RequestDetailsContent.vue`**，让它跨 span 切换稳定存在，子组件用 `defineModel` 双向绑定（Vue 3.5+ 已支持，dashboard 在 `vue ^3.5.32`）。

提升的状态：

| 名字 | 类型 | 初始值 |
| --- | --- | --- |
| `detailTab` | `'overview' \| 'request' \| 'response' \| 'logs'` | `'overview'`（已存在） |
| `requestBodyView` | `'raw' \| 'json'` | `'json'` |
| `requestHeadersOpen` | `boolean` | `false` |
| `responseSubView` | `'raw' \| 'json' \| 'aggregated' \| 'events' \| 'rendered'` | `'json'` |
| `responseHeadersOpen` | `boolean` | `false` |

子组件改造：

- `RawArtifactView` 增加 `defineModel<…>('bodyView')` 与 `defineModel<boolean>('headersOpen')`，替换内部 `requestBodyView` ref 与 `<details>` 的无状态使用。当 `kind === 'response'` 时把这两个 model 透传给 `ResponseArtifactView`（绑到 `subView` / `headersOpen` 上）。
- `ResponseArtifactView` 增加 `defineModel<SubView>('subView')` 与 `defineModel<boolean>('headersOpen')`，替换内部 `subView` ref 与 `<details>` 无状态使用。

`<details>` 改为受控：`:open="headersOpen" @toggle="headersOpen = ($event.currentTarget as HTMLDetailsElement).open"`。

## 重置语义的取舍

需要明确分清两种 watcher，避免把「合理回退」一起删掉：

- **保留**（防御性回退，处理「当前选中项已不在可选项里」的情况）：
  - `RequestDetailsContent` 中 `watch(detailTabs, …)`：当 meta 不再被选中，`logs` 选项消失时，把主 Tab 拉回 `overview`。
  - `RawArtifactView` 中 `watch(requestBodyOptions, …)`：当 body 变为不可解析时，`JSON` 选项消失，把当前选中拉回第一个可用选项。
  - `ResponseArtifactView` 中 `watch(subViewOptions, …, { immediate: true })`：同上，优先回退到 `json`，否则 `raw`。
- **删除**（破坏用户选择的「主动重置」）：
  - `RequestDetailsContent`: `watch(selectedId, () => { detailTab.value = 'overview' })`。
  - `RawArtifactView`: `watch(requestJsonBody, (parsed) => { requestBodyView.value = parsed.ok ? 'json' : 'raw' })`。
  - `ResponseArtifactView`: `watch(jsonBody, (parsed) => { if (!isSSE.value && parsed.ok) subView.value = 'json' })`。

把默认值改为 `'json'` 后，仍由「防御性回退」watcher 在 immediate 阶段自动修正为合法选项，因此 binary / 非 JSON / SSE 的首屏体验与现状一致。

## 不引入新依赖

仅使用 Vue 自带的 `defineModel`。无第三方库。状态不进 Pinia / preferences store（用户没要求跨会话持久化，且会让简单的视觉状态污染全局 store）。
