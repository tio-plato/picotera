# Plan: ModelsView 上游聚合视图

无后端改动，仅 dashboard 修改。

## 步骤 1：增强 `ModelForm.vue`

- 新增 `defaultName?: string`、`lockedName?: boolean` props。
- 表单初始值：`name: props.model?.name ?? props.defaultName ?? ''`。
- `name` Input 的 `:disabled` 改为 `isEdit || lockedName`。
- 当 `!isEdit && lockedName` 时，`SidePanel` 的 `kicker` 用「新增模型 · 来自上游」；其他场景保留现有 kicker 文案。

## 步骤 2：新建 `dashboard/src/components/ModelUpstreamsPanel.vue`

- `defineProps<{ modelName: string; upstreams: Upstream[] }>()`。`Upstream` 类型在 `ModelUpstreamsPanel.vue` 顶部 `export type Upstream`，`ModelsView` 从这里 import。
- 模板：`SidePanel` + 上游 `<ul>`，按设计 §3.2 渲染：
  - provider 名（粗体） → 上游模型名 `Tag variant="accent"`。
  - 端点 `TagList`：当端点列表来源于 entry.endpoints 时直接渲染；当来源于 provider-endpoints 兜底时，在 `TagList` 后追加 `<span class="text-2xs text-ink-faint">全部端点</span>`（panel 接收一个 `expandedFromProvider: boolean` per-upstream 标志，或在 Upstream 类型里加该字段——选后者，前端在 ModelsView 构建时记录）。
  - `priority > 0` 时再追加 `Tag` `P{priority}`。
- 底部 footer 只有一个 `Button variant="ghost"` 关闭按钮。
- 空 `upstreams` 时 `StateText` 显示「该模型暂无上游」（理论上不会被打开，但保底）。

## 步骤 3：在 `ModelsView.vue` 聚合数据

- `script setup` 内：
  - 新增 `providers = ref<ProviderView[]>([])` 和 `providerEndpoints = ref<ProviderEndpointView[]>([])`。
  - `fetchAll()` 并行三个请求，`Promise.all`。
  - `computed upstreamIndex`：按设计 §2 构建 `Record<string, Upstream[]>`，并把 `expandedFromProvider` 字段写在每个 `Upstream` 上（true 表示端点列表来自 provider-endpoints 兜底）。
  - `computed orphanRows`：`{ name: string; providerNames: string[] }[]`，按模型名排序。
  - `computed registeredNames` 用于差集。
- 模板：
  - 表头新增 `<Th>上游</Th>`，置于「系列」与 `actions` 之间。
  - 行内对应 `<Td>`：渲染数字（`text-ink` 粗体） 或灰色 `—`（数 0 时）。
  - 操作列在「编辑」前插入 `IconButton`（icon `cloud-upload`），`@click` 调用 `openUpstreams(m)`；`count === 0` 时 `:disabled="true"`。
  - `openUpstreams(m)`：`panel.open(ModelUpstreamsPanel, { modelName: m.name, upstreams: upstreamIndex.value[m.name] ?? [] }, { key: \`model-upstreams:${m.name}\` })`。
  - 在 `<DataCard>` 后插入「未注册上游模型」区块：
    - 一个 `<DataCard>`（或简化容器），顶部按钮切换 `orphanExpanded`。
    - 展开后用一个简化 `<ul>` 或 `DataTable`：每行 `mono` 模型名 + provider 名 `TagList` + 行尾 plus `IconButton`。
    - `IconButton` 点击 → `panel.open(ModelForm, { defaultName: name, lockedName: true, onSave: fetchAll }, { key: \`model:new:${name}\` })`。
  - `orphanRows.length === 0` 时整段不渲染。
- 删除 `count` 旧 `computed`，沿用 `models.value.length`，但同时显示「未注册 N」时把展开按钮文案做成「未注册上游模型 (N)」。

## 步骤 4：图标核对

确认 `cloud-upload`、`chevron-down`、`plus` 在 `dashboard/src/ui/icons/paths.ts` 中存在；缺失则从 `@tabler/icons-vue` 拷贝 path 加入并扩展 `IconName`。

## 步骤 5：验证

1. `pnpm --dir dashboard type-check`。
2. `pnpm --dir dashboard lint`。
3. `pnpm --dir dashboard build`。
4. 启动 `mise run server` + `mise run web`，手测：
   - 模型表显示上游列、点击图标打开 panel、端点列表正确（包括 entry.endpoints 为空时展开全部端点）。
   - 删除一个 provider 仍引用的模型，未注册区块出现该模型；点击 plus 打开预填且锁定 name 的 ModelForm，保存后模型回到正式列表。
   - 没有上游的模型，操作按钮禁用。
   - 没有未注册模型时尾部不渲染区块。
