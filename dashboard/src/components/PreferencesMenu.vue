<script setup lang="ts">
import { ref, useTemplateRef, watch, onBeforeUnmount } from 'vue'
import { useFloating, offset, flip, shift, autoUpdate } from '@floating-ui/vue'
import { usePreferencesStore } from '@/stores/preferences'
import type { Theme, PanelMode } from '@/stores/preferences'

const prefs = usePreferencesStore()
const open = ref(false)
const triggerRef = useTemplateRef<HTMLElement>('triggerRef')
const floatingRef = useTemplateRef<HTMLElement>('floatingRef')

const { floatingStyles } = useFloating(triggerRef, floatingRef, {
  placement: 'top-start',
  strategy: 'fixed',
  whileElementsMounted: autoUpdate,
  middleware: [offset(8), flip({ padding: 8 }), shift({ padding: 8 })],
})

function toggle() {
  open.value = !open.value
}

function close() {
  open.value = false
}

function onDocMouseDown(e: MouseEvent) {
  const t = e.target as Node
  if (floatingRef.value?.contains(t)) return
  if (triggerRef.value?.contains(t)) return
  close()
}

function onKeydown(e: KeyboardEvent) {
  if (e.key === 'Escape') close()
}

watch(open, (v) => {
  if (v) {
    document.addEventListener('mousedown', onDocMouseDown, true)
    document.addEventListener('keydown', onKeydown)
  } else {
    document.removeEventListener('mousedown', onDocMouseDown, true)
    document.removeEventListener('keydown', onKeydown)
  }
})

onBeforeUnmount(() => {
  document.removeEventListener('mousedown', onDocMouseDown, true)
  document.removeEventListener('keydown', onKeydown)
})

function setTheme(t: Theme) {
  prefs.theme = t
}

function setPanelMode(m: PanelMode) {
  prefs.panelMode = m
}

type ThemeOption = { value: Theme; label: string; surface: string; accent: string }
const themes: ThemeOption[] = [
  { value: 'light',            label: 'Pico Light',       surface: 'oklch(0.986 0.003 250)', accent: 'oklch(0.54 0.19 262)' },
  { value: 'solarized-light',  label: 'Solarized Light',  surface: 'oklch(0.965 0.036 92)',  accent: 'oklch(0.72 0.15 85)'  },
  { value: 'solarized-dark',   label: 'Solarized Dark',   surface: 'oklch(0.30 0.035 210)',  accent: 'oklch(0.68 0.14 235)' },
  { value: 'dark',             label: 'Tera Dark',        surface: 'oklch(0.22 0.02 255)',   accent: 'oklch(0.70 0.18 262)' },
]

const panelModes: { value: PanelMode; label: string }[] = [
  { value: 'auto',  label: '自动' },
  { value: 'right', label: '右侧' },
  { value: 'modal', label: '弹窗' },
]
</script>

<template>
  <button
    ref="triggerRef"
    class="prefs-trigger"
    type="button"
    aria-label="设置"
    title="设置"
    :aria-expanded="open"
    aria-haspopup="menu"
    @click="toggle"
  >
    <svg viewBox="0 0 24 24" width="14" height="14" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round">
      <circle cx="12" cy="12" r="3" />
      <path d="M19.4 15a1.65 1.65 0 0 0 .33 1.82l.06.06a2 2 0 1 1-2.83 2.83l-.06-.06a1.65 1.65 0 0 0-1.82-.33 1.65 1.65 0 0 0-1 1.51V21a2 2 0 1 1-4 0v-.09a1.65 1.65 0 0 0-1-1.51 1.65 1.65 0 0 0-1.82.33l-.06.06a2 2 0 1 1-2.83-2.83l.06-.06a1.65 1.65 0 0 0 .33-1.82 1.65 1.65 0 0 0-1.51-1H3a2 2 0 1 1 0-4h.09a1.65 1.65 0 0 0 1.51-1 1.65 1.65 0 0 0-.33-1.82l-.06-.06a2 2 0 1 1 2.83-2.83l.06.06a1.65 1.65 0 0 0 1.82.33H9a1.65 1.65 0 0 0 1-1.51V3a2 2 0 1 1 4 0v.09a1.65 1.65 0 0 0 1 1.51 1.65 1.65 0 0 0 1.82-.33l.06-.06a2 2 0 1 1 2.83 2.83l-.06.06a1.65 1.65 0 0 0-.33 1.82V9a1.65 1.65 0 0 0 1.51 1H21a2 2 0 1 1 0 4h-.09a1.65 1.65 0 0 0-1.51 1z" />
    </svg>
  </button>

  <Teleport to="body">
    <div
      v-if="open"
      ref="floatingRef"
      class="prefs-menu"
      role="menu"
      :style="floatingStyles"
    >
      <section class="prefs-section">
        <h3 class="prefs-label">外观</h3>
        <ul class="theme-list" role="radiogroup" aria-label="外观主题">
          <li v-for="t in themes" :key="t.value">
            <button
              class="theme-row"
              role="radio"
              :aria-checked="prefs.theme === t.value"
              :class="{ active: prefs.theme === t.value }"
              @click="setTheme(t.value)"
            >
              <span
                class="swatch"
                aria-hidden="true"
                :style="{
                  '--sw-surface': t.surface,
                  '--sw-accent': t.accent,
                }"
              />
              <span class="theme-name">{{ t.label }}</span>
              <span v-if="prefs.theme === t.value" class="theme-dot" aria-hidden="true" />
            </button>
          </li>
        </ul>
      </section>

      <hr class="prefs-rule" />

      <section class="prefs-section">
        <h3 class="prefs-label">面板位置</h3>
        <div class="seg" role="radiogroup" aria-label="面板位置">
          <button
            v-for="m in panelModes"
            :key="m.value"
            class="seg-btn"
            role="radio"
            :aria-checked="prefs.panelMode === m.value"
            :class="{ active: prefs.panelMode === m.value }"
            @click="setPanelMode(m.value)"
          >
            {{ m.label }}
          </button>
        </div>
      </section>
    </div>
  </Teleport>
</template>

<style scoped>
.prefs-trigger {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 1.75rem;
  height: 1.75rem;
  padding: 0;
  background: transparent;
  color: var(--color-ink-muted);
  border: 1px solid transparent;
  border-radius: 0.375rem;
  cursor: pointer;
  transition: background-color 0.1s ease, color 0.1s ease, border-color 0.1s ease;
}
.prefs-trigger:hover {
  background: var(--color-sidebar-hover);
  color: var(--color-ink);
}
.prefs-trigger[aria-expanded='true'] {
  background: var(--color-sidebar-active-bg);
  color: var(--color-sidebar-active-text);
  border-color: var(--color-line);
}

.prefs-menu {
  width: 15rem;
  padding: 0.375rem;
  background: var(--color-card-bg);
  border: 1px solid var(--color-line);
  border-radius: 0.625rem;
  box-shadow: var(--shadow-lg);
  z-index: 1000;
  color: var(--color-ink);
}

.prefs-section {
  padding: 0.375rem 0.25rem 0.5rem;
}
.prefs-label {
  margin: 0 0 0.5rem;
  padding: 0 0.375rem;
  font-size: 0.6875rem;
  font-weight: 550;
  letter-spacing: 0.06em;
  text-transform: uppercase;
  color: var(--color-ink-faint);
}

.prefs-rule {
  margin: 0;
  height: 1px;
  border: 0;
  background: var(--color-line-soft);
}

/* ----- Theme list ----- */
.theme-list {
  list-style: none;
  padding: 0;
  margin: 0;
  display: flex;
  flex-direction: column;
  gap: 1px;
}
.theme-row {
  display: grid;
  grid-template-columns: auto 1fr auto;
  align-items: center;
  gap: 0.625rem;
  width: 100%;
  padding: 0.375rem 0.5rem;
  background: transparent;
  border: 1px solid transparent;
  border-radius: 0.375rem;
  color: var(--color-ink);
  font-size: 0.8125rem;
  line-height: 1;
  text-align: left;
  cursor: pointer;
  transition: background-color 0.1s ease, border-color 0.1s ease;
}
.theme-row:hover {
  background: var(--color-surface-50);
}
.theme-row.active {
  background: var(--color-accent-faint);
  border-color: color-mix(in oklch, var(--color-accent) 25%, transparent);
  color: var(--color-accent-ink);
  font-weight: 500;
}
.swatch {
  position: relative;
  display: inline-block;
  width: 1.25rem;
  height: 1.25rem;
  border-radius: 999px;
  background:
    linear-gradient(
      90deg,
      var(--sw-surface) 0 50%,
      var(--sw-accent) 50% 100%
    );
  box-shadow: inset 0 0 0 1px color-mix(in oklch, var(--color-ink) 18%, transparent);
  flex-shrink: 0;
}
.theme-row.active .swatch {
  box-shadow:
    inset 0 0 0 1px color-mix(in oklch, var(--color-ink) 22%, transparent),
    0 0 0 2px var(--color-card-bg),
    0 0 0 3px var(--color-accent);
}
.theme-name {
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.theme-dot {
  display: inline-block;
  width: 0.375rem;
  height: 0.375rem;
  border-radius: 999px;
  background: var(--color-accent);
}

/* ----- Segmented control ----- */
.seg {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: 2px;
  padding: 2px;
  background: var(--color-surface-50);
  border: 1px solid var(--color-line);
  border-radius: 0.375rem;
}
.seg-btn {
  padding: 0.3125rem 0.25rem;
  background: transparent;
  border: 0;
  border-radius: 0.25rem;
  color: var(--color-ink-muted);
  font-size: 0.75rem;
  font-weight: 500;
  cursor: pointer;
  transition: background-color 0.1s ease, color 0.1s ease, box-shadow 0.1s ease;
}
.seg-btn:hover:not(.active) {
  color: var(--color-ink);
}
.seg-btn.active {
  background: var(--color-card-bg);
  color: var(--color-accent-ink);
  box-shadow:
    0 0 0 1px var(--color-line),
    0 1px 2px oklch(0 0 0 / 0.06);
}
</style>
