# Design

## 目标

把 dashboard 前端从"手写 CSS + `@layer components` 类"迁移到"Tailwind utility + 少量组件封装"。产出一套内部组件库 `src/ui/`，所有视图只用 utility class 和 UI 组件，不再出现 `<style scoped>` 里的业务样式。设计还原度对齐现状（Grafana/Datadog 风），主题切换继续工作。

## 结构

```
dashboard/src/
├── index.css                # @import tailwindcss + @theme + fonts + theme overrides
├── ui/                      # 可复用组件库
│   ├── index.ts             # barrel export
│   ├── Button.vue           # variant: primary | ghost | danger  size: sm | md
│   ├── IconButton.vue       # variant: default | danger, active 状态
│   ├── Input.vue            # size: sm | md, 支持 type/select 等原生属性透传
│   ├── Select.vue           # 包一层 <select> 保持外观一致
│   ├── Textarea.vue
│   ├── Field.vue            # label + slot + error
│   ├── Badge.vue            # 数值徽章
│   ├── Tag.vue              # variant: default | accent | ok | muted | more
│   ├── TagList.vue          # 统一 flex-wrap 间距
│   ├── DataCard.vue         # 卡片容器（白底 + border + 阴影 + 圆角）
│   ├── DataTable.vue        # 壳子（table + header + body 的 utility 约束）
│   ├── StateText.vue        # 空/加载态文本
│   ├── Kbd.vue              # 预留（可选）
│   ├── SegmentedControl.vue # 三段式切换（PreferencesMenu 抽取）
│   ├── Tabs.vue             # Anno/ModelList 的 rows/bulk/json 切换
│   ├── SidePanel.vue        # 现有 SidePanel 迁移到这里（重命名）
│   ├── ConfirmDialog.vue    # 现有确认框迁移
│   ├── Overlay.vue          # 内部复用的 Teleport + backdrop
│   └── icons/               # SVG 小图标统一导出为 <Icon name="…" />
│       ├── Icon.vue
│       └── paths.ts         # path data map
├── components/              # 业务组件（ProviderForm 等）保留在这里
├── views/                   # 不变
└── ...
```

### 组件库原则

1. **Utility first**：模板里直接写 Tailwind 类；组件库里也是 utility class，不再新增 `@layer components` 自定义类。
2. **Variants 用 props 驱动**，在组件内部用 Tailwind 的条件 class 映射（手写映射表，不引入 `cva` / `tailwind-variants` 依赖）。
3. **无业务耦合**：`src/ui/` 里的组件不读 store、不调 API。
4. **透传 attrs**：表单类组件继承 `$attrs`，保持原生用法。

## 颜色 Theme 映射

在 `index.css` 的 `@theme` 块里，把现有的 CSS 变量 `--color-surface-0` 等直接声明为 Tailwind 色彩 token，使之生成对应 utility：

```css
@import "tailwindcss";

@theme {
  /* 字体 */
  --font-sans: 'Geist', 'Geist Fallback', ui-sans-serif, system-ui, sans-serif;
  --font-mono: 'Geist Mono', 'Geist Mono Fallback', ui-monospace, monospace;

  /* 颜色 token — 值在 :root 和 :root[data-theme="..."] 里定义，@theme 只声明名字 */
  --color-surface-0: initial;
  --color-surface-50: initial;
  --color-surface-100: initial;
  --color-surface-200: initial;
  --color-surface-300: initial;
  --color-ink: initial;
  --color-ink-muted: initial;
  --color-ink-faint: initial;
  --color-accent: initial;
  --color-accent-strong: initial;
  --color-accent-faint: initial;
  --color-accent-ink: initial;
  --color-line: initial;
  --color-line-soft: initial;
  --color-ok: initial;      /* renamed from indicator-ok */
  --color-ok-ink: initial;
  --color-ok-faint: initial;
  --color-warn: initial;
  --color-warn-ink: initial;
  --color-warn-faint: initial;
  --color-err: initial;
  --color-err-ink: initial;
  --color-err-faint: initial;
  --color-sidebar-bg: initial;
  --color-sidebar-hover: initial;
  --color-sidebar-text: initial;
  --color-sidebar-text-active: initial;
  --color-sidebar-active-bg: initial;
  --color-sidebar-active-text: initial;
  --color-overlay-bg: initial;

  /* 阴影 */
  --shadow-xs: 0 1px 1px oklch(0.2 0.02 255 / 0.04);
  --shadow-sm: 0 1px 2px oklch(0.2 0.02 255 / 0.05), 0 1px 1px oklch(0.2 0.02 255 / 0.03);
  --shadow-lg: 0 18px 40px -12px oklch(0.15 0.02 255 / 0.22), 0 4px 12px oklch(0.15 0.02 255 / 0.06);

  /* 补充 spacing / radius / text 档位 */
  --radius-xs: 0.1875rem;   /* 3px，用于 tag 等 */
  --radius-sm: 0.25rem;     /* Tailwind 默认 */
  --radius-md: 0.375rem;    /* Tailwind 默认 */
  --radius-lg: 0.5rem;      /* Tailwind 默认 */
  --radius-xl: 0.625rem;    /* 10px，卡片 */

  --text-2xs: 0.6875rem;    /* 11px */
  --text-2xs--line-height: 1;
}

/* 四套主题：实际值在这里赋给 --color-* 变量 */
:root, :root[data-theme="light"] { /* light 默认值 */ }
:root[data-theme="dark"] { /* ... */ }
:root[data-theme="solarized-light"] { /* ... */ }
:root[data-theme="solarized-dark"] { /* ... */ }
```

这样 Tailwind 会生成 `bg-surface-0`、`text-ink`、`text-ink-muted`、`border-line`、`bg-accent-faint`、`text-err-ink` 等 utility；切换 `data-theme` 时所有 utility 自动随之换色。

### 命名精简

趁这次迁移把冗长的 `indicator-ok/warn/err` 系列重命名为 `ok/warn/err`，少一层前缀。

### `@layer components` 清理

删掉现有 `index.css` 里的 `.btn-primary`, `.btn-ghost`, `.btn-icon`, `.data-table`, `.tag`, `.field`, `.input`, `.state-text` 等组件类——对应能力全部迁进 `src/ui/*.vue`。保留：
- `@font-face`
- `html` / `body` 的全局基线（字号、背景、字体特性）
- `.mono`/`.tabular` 两个辅助类（tabular-nums）

## Spacing / Radius 对齐

现有手写值对齐到 Tailwind 档位：

| 现值 | 用途 | 替换为 |
|---|---|---|
| `0.4375rem` (7px) | 按钮/icon padding | `0.5rem` (p-2) |
| `0.6875rem` (11px) | 表格行 padding | `0.75rem` (p-3) |
| `0.5625rem` (9px) | 表头 padding-y | `0.625rem` (py-2.5) |
| `0.8125rem` (13px) | 字号 base | `var(--text-sm)` (text-sm) |
| `0.6875rem` (11px) | 小字号 | `--text-2xs` (text-2xs) — 新增 |
| `0.3125rem` (5px) | 紧凑 padding | `0.25rem` (p-1) |
| `0.1875rem` (3px) | tag 小圆角 | `--radius-xs` (rounded-xs) — 新增 |
| `0.625rem` (10px) | 卡片圆角 | `--radius-xl` (rounded-xl) — 覆盖 |

> `html { font-size: 14px }` 保留不动，Tailwind 档位的视觉尺寸继续沿用现状。

## 主题切换

保留 `usePreferencesStore` + `data-theme` 属性驱动。`@theme` 声明 token 名，`:root[data-theme]` 给具体值，Tailwind utility 自动消费。无需改 store、路由、组件树。

## SidePanelHost 和 ConfirmDialog

- `SidePanelHost` 的 layout 逻辑（auto/right/modal）保留在 `components/` 里（业务层，读 store），但内部使用 `ui/SidePanel` + `ui/Overlay`。
- `ConfirmDialog` 直接迁进 `ui/`，业务层用 `useConfirm` composable 驱动。

## Icons

现存 inline SVG 出现在 sidebar / toolbar / action button / tabs / empty / remove 等多处，重复率高。新增 `ui/icons/Icon.vue`：

```vue
<script setup lang="ts">
import { iconPaths, type IconName } from './paths'
defineProps<{ name: IconName; size?: number }>()
</script>
<template>
  <svg
    :width="size ?? 14" :height="size ?? 14"
    viewBox="0 0 24 24" fill="none"
    stroke="currentColor" stroke-width="1.8"
    stroke-linecap="round" stroke-linejoin="round"
  ><path :d="iconPaths[name]" /></svg>
</template>
```

`paths.ts` 汇总用到的 path：`plus`, `edit`, `trash`, `close`, `link`, `settings`, `cpu`, `plug`, `branch`, `db`, `list`, `lines`, `braces`, `dash`。复杂多路径的图标走 `iconPaths[name]` 存数组并循环渲染。

## 第三方依赖

无新增。继续使用已有：
- `tailwindcss@4` + `@tailwindcss/vite`
- `@floating-ui/vue`（PreferencesMenu 的 popover）

不引入 `class-variance-authority` / `tailwind-variants` / `tailwind-merge`——组件库规模小，手写 variant 映射即可。

## 非目标

- 不重新设计 UI（视觉、布局、交互保持一致）
- 不引入 shadcn-vue / PrimeVue / Element+ 等第三方组件库
- 不改业务逻辑（API 调用、store、路由）
- 不改 `dashboard/plans/` 下的历史产物

## 验收

1. `pnpm run build` 通过（含 `vue-tsc`）。
2. 浏览器四个视图（Providers/Models/Endpoints/Mappings）与主题切换前后视觉一致。
3. `grep -r '<style scoped>' src/` 在 `src/ui/` 外返回 0 条业务样式（`src/ui/` 允许 scoped 用于极少数 utility 无法表达的场景，但目标是 0）。
4. `src/index.css` 不再包含 `@layer components` 块。
