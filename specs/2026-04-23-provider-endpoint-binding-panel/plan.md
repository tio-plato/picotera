# Plan: 渠道端点绑定侧边栏

所有改动仅在 `dashboard/` 下。

## 1. 新建 `dashboard/src/components/ProviderEndpointsPanel.vue`

Props / Emits：

```ts
const props = defineProps<{ providerId: number; providerName: string }>()
const emit = defineEmits<{ close: [] }>()
```

内部状态：

- `providerEndpoints = ref<ProviderEndpointView[]>([])`
- `endpoints = ref<EndpointView[]>([])`
- `loading = ref(false)`、`error = ref('')`
- 「新增」表单本地 `form = ref({ endpointPath: '', upstreamUrl: '' })`
- 「编辑」本地映射 `drafts = reactive<Record<string, string>>({})`，键为 endpointPath，值为 upstreamUrl 草稿

行为：

- `onMounted`：并行 `GET /api/picotera/endpoints` 与 `fetchBindings()`。
- `fetchBindings()`：`GET /api/picotera/provider-endpoints?providerId=props.providerId`，成功后把 `drafts` 重新同步为已绑定的 upstreamUrl。
- `watch(() => props.providerId, fetchBindings)`：切换 provider 时刷新。
- `availableEndpoints = computed(() => endpoints.value.filter(e => !providerEndpoints.value.some(pe => pe.endpointPath === e.path)))`
- `addBinding()`：校验 `form.endpointPath && form.upstreamUrl`，调用 `PUT /provider-endpoints`，成功后清空 form 并 `fetchBindings`。
- `saveDraft(path)`：与 `providerEndpoints` 中原值对比，变化才发 `PUT`。
- `deleteBinding(path)`：`POST /provider-endpoints/delete`，成功后 `fetchBindings`。

模板结构：

```
.panel
  .panel-header (title + close btn)
  .panel-body
    section.bindings (v-for providerEndpoints, inline edit + delete)
    section.add (select + input + button; disabled when availableEndpoints.length === 0)
  .panel-error (v-if error)
```

样式 scoped：卡片底色 `var(--color-card-bg)`、左侧 1px 边框、上下内边距 1rem 1.25rem；与 data-card 视觉一致。

## 2. 改造 `dashboard/src/views/ProvidersView.vue`

结构：

- 新增 `selectedProviderId = ref<number | null>(null)`。
- `selectedProvider = computed(...)`，根据 id 从 `providers.value` 取。
- 删除/编辑 provider 时若删到或 id 变化，重置 `selectedProviderId`（简单做法：删除后若相等则清空）。
- 模板根 `.view` 内改为 `.view-body` 两栏容器：左 `.view-main`（当前表格）、右 `<ProviderEndpointsPanel v-if="selectedProvider" :providerId :providerName @close="...">`。
- 操作列前新增按钮「端点绑定」，点击 `toggleSelect(p.id)`：选中则取消，否则切换。
- 行高亮：`<tr :class="{ selected: selectedProviderId === p.id }">`，样式 scoped 加 `tr.selected { background: var(--color-surface-50); box-shadow: inset 2px 0 0 var(--color-accent); }`。
- 工具栏不动。

样式：

- `.view-body { display: flex; gap: 1rem; align-items: flex-start; flex-wrap: wrap; }`
- `.view-main { flex: 1 1 0; min-width: 0; }`
- `.side-panel-host { flex: 0 0 420px; position: sticky; top: 0; }`（随内容滚动保持在视窗内的简单实现；若 app-content 是滚动容器则 sticky 不起作用，改为 `position: relative`，接受随页滚动）
- 窄屏 `@media (max-width: 960px)` 中 `.side-panel-host { flex: 1 1 100%; }`。

## 3. 改造 `dashboard/src/components/MappingForm.vue`

- `endpoints` ref 删除；替换为 `providerEndpoints = ref<ProviderEndpointView[]>([])`、`loadingEndpoints = ref(false)`。
- `onMounted`：保持拉 models、providers；不再拉 endpoints；若 `form.providerId` 已有（edit 模式）立即调用 `fetchProviderEndpoints(form.providerId)`。
- 新增 `fetchProviderEndpoints(pid: number)`：`GET /provider-endpoints?providerId=pid`；结果写入 `providerEndpoints`；若 `form.endpointPath` 不在结果集中：
  - 新建模式：清空 `form.endpointPath`。
  - 编辑模式：保留，并在渲染时把它作为兜底 option 加进下拉。
- `watch(() => form.value.providerId, (pid, old) => { if (pid === old) return; if (isEdit) return; form.value.endpointPath = ''; if (pid) fetchProviderEndpoints(pid); else providerEndpoints.value = [] })`。
- 模板内端点 `<select>`:
  ```html
  <select v-model="form.endpointPath" class="input" required :disabled="isEdit || !form.providerId">
    <option value="" disabled>{{ form.providerId ? (providerEndpoints.length ? '选择端点' : '该渠道暂无绑定端点') : '先选择渠道' }}</option>
    <option v-for="pe in providerEndpoints" :key="pe.endpointPath" :value="pe.endpointPath">{{ pe.endpointPath }}</option>
    <option v-if="isEdit && form.endpointPath && !providerEndpoints.some(pe => pe.endpointPath === form.endpointPath)" :value="form.endpointPath">{{ form.endpointPath }}（脏数据）</option>
  </select>
  ```
- 提交逻辑不变（`PUT /model-provider-endpoints`）。

## 4. 类型与 API 客户端

无需重新生成。`ProviderEndpointView`、`EndpointView` 已存在于 `dashboard/src/api.d.ts`；openapi-fetch 已包含 `/api/picotera/provider-endpoints` 三个操作。

## 5. 验证

本地 smoke（`go run ./cmd/picotera` + `pnpm --dir dashboard dev`）：

- 渠道列表：点击某行「端点绑定」按钮，右侧出现侧边栏，列出已绑定端点；切换另一行、关闭按钮行为正常。
- 侧边栏新增一条绑定（选 path、填 upstreamUrl），列表立即刷新；修改 upstreamUrl 并失焦生效；删除按钮移除绑定。
- 下拉在所有端点都绑定后禁用添加按钮并显示提示。
- 映射页面新增：先选渠道，端点下拉出现该渠道的绑定；切换渠道清空端点选择；未绑定任何端点的渠道下拉显示空提示。
- 编辑已有映射：下拉禁用，显示原 endpointPath；若原 path 仍存在于绑定表，下拉展示一次；若不存在，兜底「（脏数据）」选项。
- 窄屏（< 960px）：侧边栏降级到表格下方，仍可用。

无需运行 Go 构建；如希望回归：`go build ./... && pnpm --dir dashboard type-check`（若已有脚本）。
