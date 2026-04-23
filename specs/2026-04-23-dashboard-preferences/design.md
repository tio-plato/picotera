# Design: 控制台偏好设置菜单

## Overview

本次改动仅在 `dashboard/` 前端完成，不涉及后端、API、数据库。

三项用户偏好由一个 Pinia store 统一管理，持久化到 `localStorage`，并通过在 `<html>` 根节点上设置 `data-theme`、`data-panel-mode`、`data-density` 三个属性驱动 CSS 变量切换：

- **主题**：亮色（默认）、Solarized Light、Solarized Dark、暗色。配合 PrimeVue Aura preset 的 `.p-dark` 约定处理 PrimeVue 组件在暗色主题下的表现。
- **弹窗样式**：自动（现有响应式行为）、固定右侧（无论视口宽度都走右栏抽屉）、固定弹窗（无论视口宽度都走居中 modal + 遮罩）。
- **边距**：宽/适中/窄三档，通过全局 CSS 变量（`--density-*`）驱动主内容 padding、表格行高、表单 gap、侧边栏 padding 等。

左下角新增设置按钮，点击用 PrimeVue `Menu`（popup）打开单层菜单，菜单项使用自定义 `item` 插槽承载三组分段单选控件。顶部 sidebar footer 里的「已连接 · 状态圆点」整行移除。

## UI: 设置按钮 + 菜单

### 位置与触发

- 设置按钮放在 `AppSidebar.vue` 的 `.sidebar-footer` 里，替换原来的「已连接」行。底部保留版本号。
- 按钮样式沿用 `.btn-icon`（齿轮 SVG），`aria-label="设置"`。
- 点击调用 PrimeVue `Menu` 的 `toggle(event)`；`Menu` `:popup="true"` 绑定到 `Menu` ref。

### 菜单结构

`Menu` 的 `model` 只给一个占位条目，真正的内容通过 `#item` 插槽全覆写，直接渲染三段设置区：

```
┌───────── 偏好设置 ─────────┐
│ 主题                       │
│ [亮色][Solarized L][Solarized D][暗色] │
│ ── 分割线 ──               │
│ 弹窗样式                   │
│ [自动][右侧][弹窗]         │
│ ── 分割线 ──               │
│ 边距                       │
│ [宽][适中][窄]             │
└───────────────────────────┘
```

每组用小标题 + 分段选择器（button group）实现。选中项用 `.btn-icon--active` 风格（accent faint 背景）。菜单整体宽度约 `260px`。

> 用 `Menu` 的好处：获得开箱即用的 popup 定位、点击外部关闭、焦点陷阱、遮盖层 z-index、过渡动画；我们只复用壳，内部完全自定义。

### 状态同步

- 每个分段按钮 `@click` 直接写 store action，无需 "应用" 按钮；变更即生效，即时持久化。
- 不关闭菜单，方便连续切换对比效果。用户通过点外部关闭。

## 偏好状态管理

### Pinia Store

新建 `dashboard/src/stores/preferences.ts`，`defineStore('preferences', ...)`：

```ts
type Theme = 'light' | 'solarized-light' | 'solarized-dark' | 'dark'
type PanelMode = 'auto' | 'right' | 'modal'
type Density = 'wide' | 'cozy' | 'compact'  // cozy = 当前默认

interface Preferences {
  theme: Theme
  panelMode: PanelMode
  density: Density
}
```

- 初始化：从 `localStorage['picotera.preferences']` 读取；缺失或解析失败用默认值 `{ theme: 'light', panelMode: 'auto', density: 'cozy' }`。
- `watch` 整个 state，变更时写回 `localStorage` 并调用 `applyPreferences()` 把值映射到 `<html>` 的 data 属性与 PrimeVue 暗色 class：
  - `document.documentElement.dataset.theme = theme`
  - `document.documentElement.dataset.panelMode = panelMode`
  - `document.documentElement.dataset.density = density`
  - 根据 theme 切换 `.p-dark` class（暗色、Solarized Dark 时加）
- Actions：`setTheme`, `setPanelMode`, `setDensity`（各自直接赋值即可；`watch` 统一处理副作用）。
- 在 `main.ts` 中调用一次 `applyPreferences()`（通过一个 `init()` action），确保首屏渲染前属性就位，避免闪一下亮色再切暗色。

### PrimeVue 暗色适配

PrimeVue Aura 默认根据 `.p-dark` class 切换深色调。修改 `main.ts`：

```ts
theme: {
  preset: Noir,
  options: {
    darkModeSelector: '.p-dark',
    ripple: true,
    cssLayer: { name: 'primevue', order: 'theme, base, primevue' },
  }
}
```

store 在 theme 为 `dark` 或 `solarized-dark` 时给 `<html>` 加 `.p-dark`。

## CSS: 主题变量

重构 `dashboard/src/index.css`：把现有 `@theme` 内的颜色变量拆到 `:root[data-theme="light"]`（默认）下，另外 3 个主题在各自选择器里覆写同名变量。保留 `@theme` 仅用于字体、shadow 等稳定量。

### 变量分组

现有颜色变量已经覆盖：surfaces、ink、accent、status、line、sidebar、overlay、shadow。四套主题各定义一组 OKLCH 值。

- **light**（现状）：沿用现有值。
- **solarized-light**（base03-base3 反色）：背景 `#fdf6e3`（oklch 约 `0.972 0.025 95`），ink `#586e75`、accent 保留蓝但向 `#268bd2` 靠拢。
- **solarized-dark**：背景 `#002b36`、surface `#073642`、ink `#93a1a1`、accent `#268bd2`。
- **dark**：中性冷灰深色（surface 从 `oklch(0.18 0.02 255)` 起），ink 反色，accent 提亮到 `oklch(0.66 0.18 262)`。

实现细节：

```css
:root[data-theme="light"] { /* 当前值 */ }
:root[data-theme="solarized-light"] { --color-surface-0: ...; /* 全量覆写 */ }
:root[data-theme="solarized-dark"],
:root[data-theme="dark"] { /* 暗色共享结构，颜色各自 */ }
```

body 背景绑定 `var(--color-surface-50)`，无需更改；切换主题自动生效。

> 选 OKLCH 而不是十六进制是为了与现有 `index.css` 风格保持一致；四套主题颜色值由实现者按上述锚点调制并在浏览器中肉眼校准。

## CSS: 弹窗样式

现有 `SidePanelHost.vue` 在视口 < 960px 时切换为 modal。改造为基于 `data-panel-mode`：

- `auto`：保留现有的 `@media (max-width: 960px)` 逻辑。
- `right`：无论宽度都走右栏形态（移除 modal 分支）。
- `modal`：无论宽度都走 modal + 遮罩，点遮罩关闭。

实现：把 `SidePanelHost.vue` 的 `<style scoped>` 拆成基础规则 + 三个 `:where(:root[data-panel-mode="X"]) & { ... }` 规则（或用 `:global`）。由于 Vue SFC `scoped` 会给选择器加 hash，改为在组件根元素上动态绑定 `:data-mode` 属性，配合 scoped 规则 `.side-panel-host[data-mode="modal"] { ... }` 更稳妥：

```vue
<aside class="side-panel-host" :data-mode="panelMode" :style="...">
```

`panelMode` 从 store 读取：
- `auto` + `window.innerWidth < 960` → modal 样式
- `right` → 右栏样式
- `modal` → modal 样式
- 其余 → 右栏样式

用 `useMediaQuery`（或一个简单的 `window.matchMedia` ref）监听 `(max-width: 960px)`，在 auto 模式下触发布局切换。

## CSS: 边距（密度）

新建三套密度变量，放在 `:root[data-density="..."]`：

```css
:root[data-density="cozy"] {
  --density-content-x: 2rem;
  --density-content-y: 0.75rem 2rem;
  --density-row-y: 0.6875rem;
  --density-cell-x: 1rem;
  --density-gap: 0.875rem;
  --density-panel-body: 0.875rem 1rem 1rem;
  --density-sidebar-item-y: 0.4375rem;
}
:root[data-density="wide"] { /* ×1.25 */ }
:root[data-density="compact"] { /* ×0.72 */ }
```

改造 `index.css` 中的硬编码间距使用 `var(--density-*)`。主要改点：

- `App.vue`: `.app-content` 的 padding → `var(--density-content-y)` + `var(--density-content-x)`；`.app-header` padding 同步。
- `index.css`: `.view { gap: var(--density-gap); }`、`.data-table td/th` 的 padding、`.panel-body` padding、`.sidebar .nav-item` padding、`.view-toolbar` gap。
- `SidePanelHost.vue` 的 padding 同步。
- 字号不随密度变化；只改 padding / gap / min-height。

> 由于变量名分散，实现时以 grep `rem` 为线索，优先替换显著影响密度观感的 padding / gap / row-height；细节间距（border-radius、icon size）保持不变，避免"缩小到看起来变形"。

## 文件清单

新增：

- `dashboard/src/stores/preferences.ts` — Pinia store + 持久化 + `applyPreferences`。
- `dashboard/src/components/PreferencesMenu.vue` — 设置菜单（用 PrimeVue `Menu` popup + 三段选择器）。

修改：

- `dashboard/src/main.ts` — 注册 store 后调用 `usePreferencesStore().init()`；为 PrimeVue 加 `darkModeSelector: '.p-dark'`。
- `dashboard/src/components/AppSidebar.vue` — 删除「已连接」行，`.sidebar-footer` 改为「齿轮按钮 + 版本号」；挂载 `PreferencesMenu`。
- `dashboard/src/App.vue` — `.app-content` / `.app-header` 使用密度变量。
- `dashboard/src/index.css` — 将颜色变量挪到 `:root[data-theme]`；新增三套主题；新增 `:root[data-density]` 变量；改造硬编码间距为变量；新增 PrimeVue 暗色适配（若需要覆写个别 primevue token）。
- `dashboard/src/components/SidePanelHost.vue` — 依 `data-panel-mode` + 视口宽度决定布局；加一个简单 `useMediaQuery` 组合函数或内联实现。

删除：无。

## Out of Scope

- 不做「跟随系统」主题（prefers-color-scheme）。用户显式选择生效。
- 不做主题缩略图、预览动画。分段按钮即时切换已经足够直观。
- 不做自定义字号 / 字体族选项（与"边距=全局密度"明确分开）。
- 不把偏好同步到后端（当前无用户系统）；`localStorage` 足够。
- 不给 PrimeVue 各组件逐个调色；依赖 Aura `.p-dark` 的默认深色处理，如有个别组件在暗色下对比度不够，留待后续单独微调。
- 不做键盘快捷键（如 `g s` 打开设置）；仅鼠标点击。
