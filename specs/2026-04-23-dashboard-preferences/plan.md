# Plan: 控制台偏好设置菜单

改动仅在 `dashboard/` 下。无后端、无迁移、无 API 变更，不需要 `sqlc generate` 或 `openapi` 重新生成。

## 1. 新增 `dashboard/src/stores/preferences.ts`

```ts
import { defineStore } from 'pinia'
import { ref, watch } from 'vue'

export type Theme = 'light' | 'solarized-light' | 'solarized-dark' | 'dark'
export type PanelMode = 'auto' | 'right' | 'modal'
export type Density = 'wide' | 'cozy' | 'compact'

const STORAGE_KEY = 'picotera.preferences'
const DEFAULTS = { theme: 'light' as Theme, panelMode: 'auto' as PanelMode, density: 'cozy' as Density }

function load() {
  try {
    const raw = localStorage.getItem(STORAGE_KEY)
    if (!raw) return { ...DEFAULTS }
    return { ...DEFAULTS, ...JSON.parse(raw) }
  } catch { return { ...DEFAULTS } }
}

export const usePreferencesStore = defineStore('preferences', () => {
  const initial = load()
  const theme = ref<Theme>(initial.theme)
  const panelMode = ref<PanelMode>(initial.panelMode)
  const density = ref<Density>(initial.density)

  function apply() {
    const root = document.documentElement
    root.dataset.theme = theme.value
    root.dataset.panelMode = panelMode.value
    root.dataset.density = density.value
    const dark = theme.value === 'dark' || theme.value === 'solarized-dark'
    root.classList.toggle('p-dark', dark)
  }

  function persist() {
    localStorage.setItem(STORAGE_KEY, JSON.stringify({ theme: theme.value, panelMode: panelMode.value, density: density.value }))
  }

  watch([theme, panelMode, density], () => { apply(); persist() })

  function init() { apply() }

  return { theme, panelMode, density, init }
})
```

## 2. 修改 `dashboard/src/main.ts`

- 在 `app.use(createPinia())` 之后、`app.mount` 之前，调用 `usePreferencesStore().init()`。
- 给 `PrimeVue` 的 `theme.options` 加 `darkModeSelector: '.p-dark'`。

```ts
import { usePreferencesStore } from './stores/preferences'
// ...after app.use(createPinia()) / app.use(router) / app.use(apiPlugin)
usePreferencesStore().init()
```

## 3. 新增 `dashboard/src/components/PreferencesMenu.vue`

用 PrimeVue `Menu` popup 作为壳，`#item` 插槽整体覆写内容。

```vue
<script setup lang="ts">
import { ref } from 'vue'
import Menu from 'primevue/menu'
import { usePreferencesStore } from '@/stores/preferences'
import type { Theme, PanelMode, Density } from '@/stores/preferences'

const prefs = usePreferencesStore()
const menuRef = ref<InstanceType<typeof Menu> | null>(null)
const items = ref([{ label: 'prefs' }]) // 占位，仅为满足 Menu 要求

function toggle(event: Event) { menuRef.value?.toggle(event) }

const THEMES: { value: Theme; label: string }[] = [
  { value: 'light', label: '亮色' },
  { value: 'solarized-light', label: 'Solarized L' },
  { value: 'solarized-dark', label: 'Solarized D' },
  { value: 'dark', label: '暗色' },
]
const PANEL_MODES: { value: PanelMode; label: string }[] = [
  { value: 'auto', label: '自动' },
  { value: 'right', label: '右侧' },
  { value: 'modal', label: '弹窗' },
]
const DENSITIES: { value: Density; label: string }[] = [
  { value: 'wide', label: '宽' },
  { value: 'cozy', label: '适中' },
  { value: 'compact', label: '窄' },
]

defineExpose({ toggle })
</script>

<template>
  <Menu ref="menuRef" :model="items" :popup="true" class="prefs-menu">
    <template #item>
      <div class="prefs-panel" @click.stop>
        <section>
          <h4>主题</h4>
          <div class="seg">
            <button v-for="t in THEMES" :key="t.value"
                    :class="{ active: prefs.theme === t.value }"
                    @click="prefs.theme = t.value">{{ t.label }}</button>
          </div>
        </section>
        <hr />
        <section>
          <h4>弹窗样式</h4>
          <div class="seg">
            <button v-for="m in PANEL_MODES" :key="m.value"
                    :class="{ active: prefs.panelMode === m.value }"
                    @click="prefs.panelMode = m.value">{{ m.label }}</button>
          </div>
        </section>
        <hr />
        <section>
          <h4>边距</h4>
          <div class="seg">
            <button v-for="d in DENSITIES" :key="d.value"
                    :class="{ active: prefs.density === d.value }"
                    @click="prefs.density = d.value">{{ d.label }}</button>
          </div>
        </section>
      </div>
    </template>
  </Menu>
</template>

<style scoped>
.prefs-panel { width: 260px; padding: 0.75rem; display: flex; flex-direction: column; gap: 0.5rem; }
.prefs-panel h4 { margin: 0 0 0.375rem; font-size: 0.6875rem; font-weight: 550; color: var(--color-ink-muted); text-transform: uppercase; letter-spacing: 0.04em; }
.prefs-panel hr { border: none; border-top: 1px solid var(--color-line); margin: 0.125rem 0; }
.seg { display: flex; gap: 0.25rem; }
.seg button {
  flex: 1 1 auto; padding: 0.3125rem 0.375rem; font-size: 0.75rem;
  background: var(--color-surface-0); border: 1px solid var(--color-line);
  border-radius: 0.375rem; color: var(--color-ink-muted); cursor: pointer;
  transition: background 0.12s, color 0.12s, border-color 0.12s;
}
.seg button:hover { background: var(--color-surface-50); color: var(--color-ink); }
.seg button.active {
  background: var(--color-accent-faint); color: var(--color-accent-ink);
  border-color: transparent;
}
</style>
```

> `Menu` 的 `model` 必须非空才能渲染 `#item` 插槽；塞一个占位对象，在插槽中完全替换渲染。`@click.stop` 阻止 PrimeVue 把整个面板当菜单项点击后自动关闭。

## 4. 修改 `dashboard/src/components/AppSidebar.vue`

替换 `.sidebar-footer`：

```vue
<div class="sidebar-footer">
  <button class="btn-icon" aria-label="设置" title="设置" @click="openPrefs">
    <svg viewBox="0 0 24 24" width="14" height="14" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round">
      <circle cx="12" cy="12" r="3" />
      <path d="M19.4 15a1.65 1.65 0 0 0 .33 1.82l.06.06a2 2 0 1 1-2.83 2.83l-.06-.06a1.65 1.65 0 0 0-1.82-.33 1.65 1.65 0 0 0-1 1.51V21a2 2 0 1 1-4 0v-.09a1.65 1.65 0 0 0-1-1.51 1.65 1.65 0 0 0-1.82.33l-.06.06a2 2 0 1 1-2.83-2.83l.06-.06a1.65 1.65 0 0 0 .33-1.82 1.65 1.65 0 0 0-1.51-1H3a2 2 0 1 1 0-4h.09a1.65 1.65 0 0 0 1.51-1 1.65 1.65 0 0 0-.33-1.82l-.06-.06a2 2 0 1 1 2.83-2.83l.06.06a1.65 1.65 0 0 0 1.82.33H9a1.65 1.65 0 0 0 1-1.51V3a2 2 0 1 1 4 0v.09a1.65 1.65 0 0 0 1 1.51 1.65 1.65 0 0 0 1.82-.33l.06-.06a2 2 0 1 1 2.83 2.83l-.06.06a1.65 1.65 0 0 0-.33 1.82V9a1.65 1.65 0 0 0 1.51 1H21a2 2 0 1 1 0 4h-.09a1.65 1.65 0 0 0-1.51 1z" />
    </svg>
  </button>
  <span class="version">v1.0.0</span>
  <PreferencesMenu ref="prefsRef" />
</div>
```

- `import PreferencesMenu from '@/components/PreferencesMenu.vue'`
- `const prefsRef = ref<InstanceType<typeof PreferencesMenu> | null>(null)`
- `function openPrefs(e: Event) { prefsRef.value?.toggle(e) }`
- 删除 `.footer-row`、`.status-dot`、`.footer-label` 相关 HTML 与 CSS。

## 5. 修改 `dashboard/src/index.css`

### 5.1 主题变量重构

把现有 `@theme` 内的颜色变量（surface / ink / accent / status / line / sidebar / overlay / shadow）挪到 `:root[data-theme="light"]`。在新的 `@theme` 里保留字体与字体 metrics。

新增三个主题块：

```css
:root[data-theme="light"] { /* 现值照搬 */ }

:root[data-theme="solarized-light"] {
  --color-surface-0: oklch(0.99 0.025 95);
  --color-surface-50: oklch(0.972 0.03 95);   /* #fdf6e3 近似 */
  --color-surface-100: oklch(0.945 0.04 90);
  --color-surface-200: oklch(0.91 0.045 88);
  --color-surface-300: oklch(0.86 0.05 85);
  --color-ink: oklch(0.42 0.03 210);           /* #586e75 近似 */
  --color-ink-muted: oklch(0.53 0.03 210);
  --color-ink-faint: oklch(0.66 0.025 200);
  --color-accent: oklch(0.58 0.14 235);        /* #268bd2 近似 */
  --color-accent-strong: oklch(0.50 0.16 235);
  --color-accent-faint: oklch(0.93 0.05 235);
  --color-accent-ink: oklch(0.42 0.14 235);
  --color-line: oklch(0.88 0.04 90);
  --color-line-soft: oklch(0.93 0.035 90);
  --color-sidebar-bg: oklch(0.955 0.035 92);
  /* ... 其余同构覆写 */
}

:root[data-theme="solarized-dark"] {
  --color-surface-0: oklch(0.27 0.035 210);    /* #073642 近似 */
  --color-surface-50: oklch(0.23 0.035 210);   /* #002b36 近似 */
  --color-surface-100: oklch(0.31 0.035 210);
  --color-surface-200: oklch(0.36 0.035 210);
  --color-surface-300: oklch(0.44 0.03 210);
  --color-ink: oklch(0.82 0.02 205);           /* #93a1a1 近似 */
  --color-ink-muted: oklch(0.70 0.025 205);
  --color-ink-faint: oklch(0.58 0.025 205);
  --color-accent: oklch(0.66 0.14 235);
  --color-accent-strong: oklch(0.72 0.15 235);
  --color-accent-faint: oklch(0.32 0.06 235);
  --color-accent-ink: oklch(0.80 0.12 235);
  --color-line: oklch(0.34 0.035 210);
  --color-line-soft: oklch(0.30 0.035 210);
  --color-sidebar-bg: oklch(0.25 0.035 210);
  /* ... */
}

:root[data-theme="dark"] {
  --color-surface-0: oklch(0.20 0.02 255);
  --color-surface-50: oklch(0.18 0.02 255);
  --color-surface-100: oklch(0.24 0.02 255);
  --color-surface-200: oklch(0.28 0.02 255);
  --color-surface-300: oklch(0.36 0.02 255);
  --color-ink: oklch(0.92 0.015 255);
  --color-ink-muted: oklch(0.72 0.02 255);
  --color-ink-faint: oklch(0.56 0.02 255);
  --color-accent: oklch(0.66 0.18 262);
  --color-accent-strong: oklch(0.74 0.18 262);
  --color-accent-faint: oklch(0.28 0.08 262);
  --color-accent-ink: oklch(0.82 0.15 262);
  --color-line: oklch(0.30 0.02 255);
  --color-line-soft: oklch(0.26 0.02 255);
  --color-sidebar-bg: oklch(0.19 0.02 255);
  /* ... */
}
```

> 以上数值是实现起点；在浏览器里逐一肉眼校准对比度。暗色下确保 `.tag--accent`、`.badge`、状态色仍可辨。

### 5.2 密度变量

在同文件追加：

```css
:root { /* fallback; 被 data-density 覆盖 */
  --density-content-x: 2rem;
  --density-content-y-top: 0.75rem;
  --density-content-y-bottom: 2rem;
  --density-header-y: 1.125rem 2rem 0.875rem;
  --density-view-gap: 0.875rem;
  --density-row-y: 0.6875rem;
  --density-cell-x: 1rem;
  --density-head-y: 0.5625rem;
  --density-panel-body-y: 0.875rem;
  --density-panel-body-x: 1rem;
  --density-panel-gap: 1.125rem;
  --density-sidebar-item-y: 0.4375rem;
  --density-sidebar-item-x: 0.625rem;
}
:root[data-density="cozy"] { /* 同上，显式写一遍便于覆盖 */ }
:root[data-density="wide"] {
  --density-content-x: 2.5rem;
  --density-content-y-top: 1.125rem;
  --density-content-y-bottom: 2.5rem;
  --density-view-gap: 1.125rem;
  --density-row-y: 0.875rem;
  --density-cell-x: 1.125rem;
  --density-head-y: 0.75rem;
  --density-panel-body-y: 1.125rem;
  --density-panel-body-x: 1.125rem;
  --density-panel-gap: 1.375rem;
  --density-sidebar-item-y: 0.5625rem;
  --density-sidebar-item-x: 0.75rem;
}
:root[data-density="compact"] {
  --density-content-x: 1.25rem;
  --density-content-y-top: 0.5rem;
  --density-content-y-bottom: 1.25rem;
  --density-view-gap: 0.5rem;
  --density-row-y: 0.4375rem;
  --density-cell-x: 0.625rem;
  --density-head-y: 0.375rem;
  --density-panel-body-y: 0.625rem;
  --density-panel-body-x: 0.75rem;
  --density-panel-gap: 0.75rem;
  --density-sidebar-item-y: 0.3125rem;
  --density-sidebar-item-x: 0.5rem;
}
```

### 5.3 套用密度变量

把 `index.css` 里的硬编码 padding/gap 替换为变量：

- `.view { gap: var(--density-view-gap); }`
- `.data-table thead th { padding: var(--density-head-y) var(--density-cell-x); }`
- `.data-table tbody td { padding: var(--density-row-y) var(--density-cell-x); }`
- `.data-table thead th:first-child`、`tbody td:first-child` 的左 padding（1.125rem 硬编码），改为 `calc(var(--density-cell-x) + 0.125rem)`；末列同理右 padding。

## 6. 修改 `dashboard/src/App.vue`

- `.app-content` padding 改为 `var(--density-content-y-top) var(--density-content-x) var(--density-content-y-bottom)`。
- `.app-header` padding 改为 `var(--density-header-y)`（保持三段 shorthand 一致）。

## 7. 修改 `dashboard/src/components/SidePanel.vue`

- `.panel-body` padding 改为 `var(--density-panel-body-y) var(--density-panel-body-x) calc(var(--density-panel-body-y) + 0.125rem)`、`gap: var(--density-panel-gap)`。
- `.panel-header`、`.panel-footer` padding 可保持不变（头/尾视觉稳定优先）。

## 8. 修改 `dashboard/src/components/SidePanelHost.vue`

改造为按 `panelMode` + 视口宽度选择布局：

```vue
<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from 'vue'
import { useSidePanel } from '@/composables/useSidePanel'
import { usePreferencesStore } from '@/stores/preferences'

const { state, close } = useSidePanel()
const prefs = usePreferencesStore()
const cssWidth = computed(() => state.value?.width ?? '420px')

const narrow = ref(false)
let mql: MediaQueryList | null = null
function onChange(e: MediaQueryListEvent) { narrow.value = e.matches }
onMounted(() => {
  mql = window.matchMedia('(max-width: 960px)')
  narrow.value = mql.matches
  mql.addEventListener('change', onChange)
})
onUnmounted(() => { mql?.removeEventListener('change', onChange) })

const mode = computed<'right' | 'modal'>(() => {
  if (prefs.panelMode === 'right') return 'right'
  if (prefs.panelMode === 'modal') return 'modal'
  return narrow.value ? 'modal' : 'right'
})
</script>

<template>
  <aside v-if="state" class="side-panel-host" :data-mode="mode"
         :style="{ '--side-panel-width': cssWidth }">
    <div class="side-panel-host__backdrop" @click="close" />
    <component :is="state.component" :key="state.key" v-bind="state.props"
               class="side-panel-host__panel" @close="close" />
  </aside>
</template>

<style scoped>
.side-panel-host {
  flex: 0 0 var(--side-panel-width);
  width: var(--side-panel-width);
  display: flex;
  min-height: 0;
  align-self: stretch;
  padding: var(--density-content-y-top) var(--density-content-x) var(--density-content-y-bottom) 0;
}
.side-panel-host__backdrop { display: none; }
.side-panel-host__panel { width: 100%; max-height: 100%; min-height: 0; }

.side-panel-host[data-mode="modal"] {
  position: fixed; inset: 0; z-index: 900;
  flex: 0 0 auto; width: auto;
  align-items: center; justify-content: center; padding: 1rem;
}
.side-panel-host[data-mode="modal"] .side-panel-host__backdrop {
  display: block; position: absolute; inset: 0;
  background: var(--color-overlay-bg); backdrop-filter: blur(4px);
}
.side-panel-host[data-mode="modal"] .side-panel-host__panel {
  position: relative;
  width: min(var(--side-panel-width), 100%);
  max-height: calc(100vh - 2rem);
  box-shadow: 0 25px 50px -12px oklch(0.1 0.02 250 / 0.25);
}
</style>
```

删除旧的 `@media (max-width: 960px)` 块。

## 9. 修改 `dashboard/src/components/AppSidebar.vue`（密度）

- `.sidebar-nav .nav-item` padding 改为 `var(--density-sidebar-item-y) var(--density-sidebar-item-x)`。
- `.sidebar-footer` 重写：

```css
.sidebar-footer {
  padding: 0.625rem 0.875rem 0.75rem;
  border-top: 1px solid var(--color-sidebar-border);
  display: flex; align-items: center; justify-content: space-between; gap: 0.5rem;
}
.sidebar-footer .version {
  font-family: var(--font-mono); font-size: 0.6875rem;
  color: var(--color-ink-faint); font-variant-numeric: tabular-nums;
}
```

删除 `.footer-row`、`.status-dot`、`.footer-label` 相关 CSS 段。

## 10. 验证

`pnpm --dir dashboard dev` 启动后：

1. 左下角有齿轮按钮；「已连接」已从 footer 移除。
2. 点击齿轮打开菜单，菜单居于按钮上方/右方（PrimeVue 自动定位）；包含「主题/弹窗样式/边距」三组分段按钮，默认分别高亮「亮色/自动/适中」。
3. 切主题：
   - Solarized Light：背景偏米黄；文本偏灰蓝。
   - Solarized Dark：深青黑底；accent 与 badge 可辨。
   - 暗色：整体中性深灰；PrimeVue ConfirmPopup、Menu 在暗色下也用深色样式（`.p-dark` 生效）。
4. 切弹窗样式：
   - 右侧：宽屏下与现状一致；窄屏（< 960px）不再切 modal。
   - 弹窗：宽屏下也弹出居中 modal + 遮罩，点击遮罩关闭。
   - 自动：同当前行为。
5. 切边距：`宽/适中/窄` 三档，主内容 padding、表格行高、sidebar 导航间距、侧边栏表单间距明显变化；行为无破坏（按钮仍可点、弹窗不破版）。
6. 刷新页面偏好保留；清空 localStorage 后回到默认。

无 Go 改动。按需跑 `pnpm --dir dashboard type-check`（若 `package.json` 提供该脚本）。
