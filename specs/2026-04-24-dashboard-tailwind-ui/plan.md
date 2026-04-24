# Execution Plan

一次性大改，单 PR。按以下顺序执行，每步结束都要求 `pnpm type-check` 通过；最后一步跑 `pnpm build` 并在浏览器逐页回归。

## 1. 重写 `src/index.css`

- 保留 `@import "tailwindcss";`、`@font-face` 块、`html`/`body` 全局样式、`.mono`/`.tabular`。
- 新建 `@theme` 块，声明全部 `--color-*`、`--font-sans/mono`、`--shadow-xs/sm/lg`、新增 `--radius-xs/xl`、新增 `--text-2xs`。把 `--color-indicator-ok/warn/err` 改名为 `--color-ok/warn/err`（三组 base/ink/faint）。
- 删除 `@layer components` 块全部内容。
- 把四套 `:root[data-theme="..."]` 里的 `--color-indicator-*` 也改名为 `--color-ok/warn/err`。
- `data-theme="light"` 的默认值从 `@theme` 里搬到 `:root, :root[data-theme="light"] { ... }`（避免 `@theme` 里写具体颜色导致其被 Tailwind 视为静态默认色）。

## 2. 搭 `src/ui/` 基础组件

按此顺序新建文件（每个文件写完即可独立使用）：

1. `ui/icons/paths.ts` + `ui/icons/Icon.vue` —— 汇总项目用到的 13 个 SVG path。
2. `ui/Button.vue` —— props: `variant: 'primary'|'ghost'|'danger' = 'primary'`, `size: 'sm'|'md' = 'md'`, `type`, `disabled`。
3. `ui/IconButton.vue` —— props: `variant: 'default'|'danger' = 'default'`, `active?: boolean`。默认渲染 24x24 按钮容纳 13px 图标。
4. `ui/Input.vue` —— props: `size: 'sm'|'md' = 'md'`, `modelValue`, `type='text'`；透传 `$attrs`。
5. `ui/Select.vue` —— 同上，包 `<select>`。
6. `ui/Textarea.vue` —— 同上，包 `<textarea>`，支持等宽字体 prop。
7. `ui/Field.vue` —— slot = input；props: `label`, `error?`。渲染 `label.flex.flex-col.gap-1 > span.uppercase.text-2xs.font-medium.text-ink-muted.tracking-[0.03em] + <slot/>`。
8. `ui/Badge.vue` —— 数字徽章。
9. `ui/Tag.vue` + `ui/TagList.vue` —— variant: `default|accent|ok|more`；TagList 提供 flex-wrap gap-1。
10. `ui/DataCard.vue` —— 卡片容器。
11. `ui/DataTable.vue` —— `<table>` 壳子 + `<thead>`/`<tbody>` 样式通过插槽传入；actions 列的 hover 淡入通过 `group-hover:opacity-100` 实现。内部约定 `<Th>`/`<Td>` 子组件以便保持 padding 一致。
12. `ui/StateText.vue` —— props: `dashed?: boolean`, `compact?: boolean`。
13. `ui/SegmentedControl.vue` —— props: `modelValue`, `options: {value, label}[]`, `columns?: number`。
14. `ui/Tabs.vue` —— props: `modelValue`, `tabs: {value, label, icon?: IconName}[]`。匹配 Anno/ModelList 的 tab 外观。
15. `ui/Overlay.vue` —— Teleport 到 body，遮罩 + 居中 slot；props: `open`, `blur?`。
16. `ui/SidePanel.vue` —— 从现有 `components/SidePanel.vue` 搬来重写；props 不变 (`title`/`kicker`/`subtitle`)，slot 不变。
17. `ui/ConfirmDialog.vue` —— 从 `components/ConfirmDialog.vue` 搬来重写。
18. `ui/index.ts` —— barrel。

## 3. 改 `src/App.vue`

删 `<style scoped>`，shell 用 utility：
```html
<div class="flex min-h-dvh bg-surface-50">
  <AppSidebar />
  <main class="flex-1 flex flex-col min-w-0">
    <header class="px-8 pt-[1.125rem] pb-3.5"> ... </header>
    <div class="flex-1 flex min-h-0 min-w-0">
      <div class="flex-1 min-w-0 px-8 pt-3 pb-8 overflow-y-auto"><RouterView /></div>
      <SidePanelHost />
    </div>
  </main>
</div>
<ConfirmDialog />
```

## 4. 改 `AppSidebar.vue` 和 `PreferencesMenu.vue`

- `AppSidebar`：删 `<style scoped>`，改用 utility；导航图标通过 `<Icon name="db|cpu|plug|branch" />`。
- `PreferencesMenu`：保留 floating-ui 逻辑；内部的 `.theme-row`/`.seg` 全部改成 utility，`.seg` 用新 `ui/SegmentedControl`；theme swatch 保持 inline `--sw-surface`/`--sw-accent` CSS 变量方案。

## 5. 改业务组件

按表替换：

| 文件 | 操作 |
|---|---|
| `components/SidePanel.vue` | 删除，引用方改 import `@/ui/SidePanel` |
| `components/ConfirmDialog.vue` | 删除，App.vue 改 import `@/ui/ConfirmDialog` |
| `components/SidePanelHost.vue` | 保留；内部使用 `ui/Overlay` 的 backdrop，shell 改 utility |
| `components/ProviderForm.vue` | `btn-ghost/btn-primary/field/field-label/input` → `Button`/`Field`/`Input` 等；删 `<style scoped>` |
| `components/EndpointForm.vue` | 同上 |
| `components/ModelForm.vue` | 同上 |
| `components/MappingForm.vue` | 同上（含 `<select>` 使用 `Select`） |
| `components/AnnotationsEditor.vue` | tab 用 `ui/Tabs`；row/输入框/删除按钮改 utility + `IconButton`；remove style scoped |
| `components/ModelListEditor.vue` | 同上 |
| `components/ProviderEndpointsPanel.vue` | 整块重写为 utility；`input--sm`/`btn-primary--sm` → `<Input size="sm">`/`<Button size="sm">`；remove style scoped |

## 6. 改视图

`ProvidersView.vue` / `EndpointsView.vue` / `ModelsView.vue` / `MappingsView.vue`：

- `.view` 容器 → `<div class="flex flex-col gap-3.5">`。
- `.view-toolbar` → `<div class="flex items-center justify-between gap-3">`。
- `.view-toolbar__meta` → `<span class="text-xs text-ink-faint tabular-nums">`。
- `.data-card` → `<DataCard>`；内含 `<DataTable>`（带 `<Th>`/`<Td>` 插槽）。
- `.state-text` → `<StateText>`。
- `.badge`/`.tag`/`.tag--accent`/`.tag--more`/`.tag-list` → `<Badge>`/`<Tag variant="…">`/`<TagList>`。
- `.btn-primary`/`.btn-icon` 等 → `<Button>`/`<IconButton>`；操作按钮内的 SVG → `<Icon name="plus|edit|trash|link">`。
- `.col-actions` 的 hover 淡入：表格行加 `group`，action 单元格 `opacity-55 group-hover:opacity-100 transition`。
- `.mono`/`.muted`/`.font-medium` 保留（`mono` 是辅助类，`muted` 改为 `text-ink-faint` utility）。

## 7. 清理验证

1. `pnpm lint:eslint`（项目已配置）—— 修掉未使用 import。
2. `pnpm type-check`。
3. `pnpm build`。
4. `grep -rn '<style scoped>' src/views src/components` → 空。
5. `grep -rn 'var(--color-indicator-' src/` → 空（确认已改名）。
6. 起 `pnpm dev`，逐项过：
   - 四个 view 的列表渲染、hover、选中高亮。
   - 新建 / 编辑 / 删除 Provider（确认 ConfirmDialog、SidePanel 三种 panelMode）。
   - Provider 端点绑定面板（新增/修改 draft/删除）。
   - Mapping 编辑（下拉与 dirty endpoint 处理）。
   - AnnotationsEditor 三个 tab，ModelListEditor 两个 tab。
   - PreferencesMenu：四个 theme 切换全部生效，panelMode 三档切换。
   - 窄屏（≤960px）下 SidePanelHost 自动 modal。

## 8. 补记忆

- 保存 feedback：UI 层迁移到 `src/ui/` 组件库 + Tailwind utility，不引入第三方 UI / variant 库。
