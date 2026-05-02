# Design: ModelsView 上游聚合视图

## 1. 数据来源

完全前端聚合，不新增后端 API。`ModelsView` 挂载时并行拉取：

- `GET /api/picotera/models` → `ModelView[]`
- `GET /api/picotera/providers` → `ProviderView[]`（含 `providerModels: Record<string, ProviderModelEntry>`）
- `GET /api/picotera/provider-endpoints` → `ProviderEndpointView[]`（用于把 entry.endpoints 为空的情形展开成「该 provider 全部已绑定端点」）

三者并行 fetch，全部就绪后构建派生数据。

## 2. 派生数据结构

```ts
type Upstream = {
  providerId: number
  providerName: string
  upstreamModelName: string  // entry.upstreamModelName || modelName
  endpointPaths: string[]    // entry.endpoints 非空 → 用之；否则展开为该 provider 全部已绑定端点路径
  priority: number           // entry.priority ?? 0
}

type UpstreamIndex = Record<string /* modelName */, Upstream[]>
```

构建过程：

1. 按 `providerId` 把 `provider-endpoints` 分组成 `Record<number, string[]>`（已绑定端点路径列表，按 path 排序）。
2. 遍历每个 provider 的 `providerModels`：对每个 `(modelName, entry)`，往 `UpstreamIndex[modelName]` 里 push 一项 `Upstream`。
   - `endpointPaths` 取 `entry.endpoints` 非空数组；否则取 `providerEndpointMap[providerId]`。
   - `upstreamModelName` 缺省时回退到 `modelName`。
3. 同步派生 `orphanModelNames`：`Object.keys(UpstreamIndex)` 减去已注册模型名集合，按字典序排序。

## 3. 视图改动

### 3.1 `ModelsView.vue`

- 表头新增「上游」列，渲染 `UpstreamIndex[m.name]?.length ?? 0`，0 时显示为灰色 `—`。
- 操作列新增 `IconButton`（图标 `cloud-upload`），点击调用 `useSidePanel.open(ModelUpstreamsPanel, { modelName })`。该按钮在 `count === 0` 时禁用并加 `title="无上游"`。
- 列表底部追加一段「未注册的上游模型」可折叠区块（默认折叠，标题旁边显示数量）：
  - 标题行：`<button>` 切换展开，icon 用现有的 `chevron-down` + 旋转 trick；标题文本「未注册的上游模型 (N)」。
  - 展开后渲染一个简化的 `DataTable` 或 `<ul>`，每行：模型名 + 「该模型在哪些 provider 上有」的 Tag 列表（provider name）+ 行尾「添加为模型」`IconButton`（plus 图标）。
  - N === 0 时整段不渲染。
- 点击「添加为模型」→ `panel.open(ModelForm, { defaultName, lockedName: true, onSave: refetchAll })`。

### 3.2 新建 `ModelUpstreamsPanel.vue`

`SidePanel` 内容布局参考 `ProviderEndpointsPanel.vue` 的「列表块」段落：

- 顶部 kicker `上游`，title 用模型名。
- 内容是一个 `<ul>`，每个上游一项：
  - 第一行：`provider.name`（粗体，单色文本） + 灰色箭头 `→` + 上游模型名（mono 字体，`Tag variant="accent"`）。
  - 第二行：端点路径 TagList，每个 path 一个 `Tag`；若 `endpointPaths` 来源是 entry.endpoints 显示原样；若是从 provider-endpoints 兜底展开，附加一个「全部端点」灰色小注释。
  - 第三行（可选）：`P{priority}` Tag，仅当 `priority > 0`。
- 底部仅 `关闭` 按钮（无保存动作）。
- 该 panel 自身不重新 fetch，而是接收已聚合好的 `Upstream[]` 作为 prop。

为了让 panel 在数据变化时同步刷新，`ModelsView` 在 `refetchAll` 后重新构建 `UpstreamIndex`，并通过 `panel.open` 重新打开（与现有 ProviderModelsPanel 模式一致：父级负责数据，panel 是显示层）。

### 3.3 `ModelForm.vue` 增强

新增 props：

```ts
defineProps<{
  model?: ModelView
  defaultName?: string
  lockedName?: boolean
  onSave?: () => void
}>()
```

- 当 `!isEdit && defaultName` 时，初始化 `form.name = defaultName`。
- 当 `lockedName` 为真时，name `Input` 设置 `disabled`。`isEdit` 已 disable name 字段，逻辑合并为 `:disabled="isEdit || lockedName"`。
- panel kicker 在 `lockedName && !isEdit` 时显示「新增模型 · 来自上游」以提示来源。

## 4. 交互细节

- 三个请求任一失败时，整个页面顶部展示一条 `StateText`（保留已成功的部分）。最简实现：维持现有 `loading` 单一信号，任一失败时进入空态并显示错误。
- 「未注册」列表的展开状态用 `ref<boolean>` 存放，刷新数据不重置（保持用户当前展开状态）。
- 添加成功后 `onSave` → `refetchAll`，新模型从「未注册」列表移到正式列表（自动）。
- 删除模型不影响 provider 的 `providerModels`：删后该模型可能掉到「未注册」列表里，符合预期。

## 5. 不引入新依赖

- 复用 `SidePanel`、`useSidePanel`、`DataTable` 系列原语、`Tag` / `TagList`、`Icon`。
- 无需新增图标：`cloud-upload`、`chevron-down`、`plus` 已在现有 paths 中（若个别缺失，按 `dashboard/src/ui/icons/paths.ts` 现有规范补 path）。

## 6. 范围外

- 不为「未注册模型」做批量添加。
- 不在 panel 内提供跳转到对应 provider 的快捷入口（后续可加）。
- 不实现服务端聚合 endpoint。
