# Design: 渠道端点绑定侧边栏

## Overview

本次变更全部在前端 `dashboard/` 内完成：

1. 在 `ProvidersView.vue` 中引入一个行内可展开的「端点绑定」侧边栏。选中一行后主表格变窄、右侧出现操作栏，列出该渠道已绑定的 `provider_endpoint`，支持新增（选择端点 + 填写 upstreamUrl）、就地编辑 upstreamUrl、删除。
2. 在 `MappingForm.vue` 中级联加载：用户选好渠道后，端点下拉只显示该渠道已绑定的 `provider_endpoint.endpoint_path`；切换渠道会清空已选端点。

后端 API 已完备（`GET/PUT/POST /provider-endpoints*`），本次不改后端、sqlc、合约、迁移。

## UI: ProvidersView 侧边栏

### 布局

`ProvidersView.vue` 的根 `.view` 使用 flex 布局：

- 左侧：现有的表格区域，宽度从 100% 收窄为 `flex: 1 1 auto`，允许 min-width 0 以触发列省略。
- 右侧：`ProviderEndpointsPanel.vue`，`width: 420px`，`flex: 0 0 420px`，仅在有 `selectedProviderId` 时渲染。

选择模型使用「本地状态 + 选中高亮」：表格行加一个新的「端点」图标按钮；点击后 `selectedProviderId` 切换。再次点击同一行的按钮或侧边栏内「关闭」可折叠。选中行通过 `tr.selected` 背景色高亮。

不使用浮层 / overlay，不做路由切换。侧边栏本身不是全屏 modal，仅占据 content 区域右侧，表格依旧可以滚动和交互。

### 交互

- 按钮图标使用与现有「编辑/删除」同风格的 inline SVG（链路/链条样式，替代“端点”），放在操作列之前。
- 侧边栏顶部显示「<Provider.name> · 端点绑定」标题与关闭按钮。
- 内容区分两部分：
  - 「已绑定」列表：每项一行，左侧 path，中间 `<input>`（inline 编辑 upstreamUrl，blur 或 Enter 触发 PUT），右侧删除图标。
  - 「新增」表单：`<select>` 选择端点（过滤掉已绑定的 path）、`<input>` 填 upstreamUrl，「添加」按钮触发 PUT。
- 删除按钮直接调用 `POST /provider-endpoints/delete`，无二次确认（本次侧边栏内操作频繁，确认弹窗过重；删除错绑定后可立即重新添加）。
- 所有写操作完成后重新拉 `GET /provider-endpoints?providerId=...` 刷新列表。
- 基础错误处理：接口失败时在侧边栏底部显示红色错误条；不阻塞其他操作。

### 数据来源

- `endpoints` 列表从 `GET /api/picotera/endpoints` 拉一次，存在组件内；侧边栏打开时读取。
- `providerEndpoints` 按所选 `providerId` 即时拉取，不跨 provider 缓存。

## UI: MappingForm 级联

- 内部 `endpoints` ref 不再预加载全量；改为一个 `providerEndpoints` ref，类型 `ProviderEndpointView[]`。
- `watch(() => form.providerId, async (pid) => { ... })`：
  - `pid === 0` 时清空 `providerEndpoints`、将 `form.endpointPath` 重置为 `''`。
  - 有效 pid：`GET /provider-endpoints?providerId=pid`，填充列表；若当前 `form.endpointPath` 不在返回的 path 集合中，清空它。
- `<select v-model="form.endpointPath">` 选项改为 `v-for="pe in providerEndpoints"`，显示 `{{ pe.endpointPath }}`。
- 编辑模式（`isEdit`）下仍禁用端点下拉，但首次挂载时也要触发一次 `providerEndpoints` 拉取，以保证下拉里能渲染当前 `endpointPath`（即使只读）。若已绑定列表不包含该 path（历史脏数据），降级为展示原值作为唯一选项。
- 端点名称不再从 `EndpointView` 取；`ProviderEndpointView` 目前只有 path 与 upstreamUrl，展示 path 已足够识别。

## 组件边界

新增：

- `dashboard/src/components/ProviderEndpointsPanel.vue`：侧边栏组件。Props: `providerId: number`, `providerName: string`；Emits: `close`。
  内部自管 `providerEndpoints`、`endpoints`、loading、error 状态。

修改：

- `dashboard/src/views/ProvidersView.vue`：加入选中逻辑与两栏布局，表格行新增端点绑定按钮。
- `dashboard/src/components/MappingForm.vue`：按上节改造。

不修改：

- 其他 view、后端、contract、sqlc、OpenAPI。生成的 `api.d.ts` 已在上个 spec 包含 ProviderEndpoints 操作，无需重新生成。

## 样式与可访问性

- 选中行 `background: var(--color-surface-50)`，左侧 2px accent 条。
- 侧边栏与主表同处一个滚动容器内，侧边栏内部自己的内容独立滚动。
- 按钮 `aria-label`、`title` 完整；`<select>` 空状态显示「该渠道暂无可绑定端点」并禁用添加按钮。
- 移动端 / 窄屏不做特别优化，视口 < 960px 时侧边栏叠在表格下方（`flex-wrap`），保持可用即可。

## Out of Scope

- 不添加「批量绑定」「一次选多个端点」等功能。
- 不为 `provider_endpoint` 新增字段（如启用/禁用开关、标注）。
- Mapping 表单仍保留旧的「先选端点后选渠道」顺序的可能性：不支持，本 spec 明确要求「先渠道后端点」。
- 不再修改 OverlayPanel、其他表单。
