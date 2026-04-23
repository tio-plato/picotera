<script setup lang="ts">
import { ref, computed, onMounted, onBeforeUnmount } from 'vue'
import { usePreferencesStore } from '@/stores/preferences'
import type { Theme, PanelMode } from '@/stores/preferences'

const prefs = usePreferencesStore()
const open = ref(false)
const menuEl = ref<HTMLElement | null>(null)

function toggle() {
  open.value = !open.value
}

function close() {
  open.value = false
}

function onClickOutside(e: MouseEvent) {
  if (menuEl.value && !menuEl.value.contains(e.target as Node)) {
    close()
  }
}

onMounted(() => document.addEventListener('click', onClickOutside, true))
onBeforeUnmount(() => document.removeEventListener('click', onClickOutside, true))

function setTheme(t: Theme) {
  prefs.theme = t
  close()
}

function setPanelMode(m: PanelMode) {
  prefs.panelMode = m
  close()
}

const themes: { value: Theme; label: string }[] = [
  { value: 'light', label: 'Pico Light' },
  { value: 'solarized-light', label: 'Solarized Light' },
  { value: 'solarized-dark', label: 'Solarized Dark' },
  { value: 'dark', label: 'Tera Dark' },
]

const panelModes: { value: PanelMode; label: string }[] = [
  { value: 'auto', label: '自动' },
  { value: 'right', label: '右侧' },
  { value: 'modal', label: '弹窗' },
]

defineExpose({ toggle })
</script>

<template>
  <div ref="menuEl" class="prefs-menu-wrap">
    <div v-if="open" class="prefs-menu">
      <div class="prefs-header">主题</div>
      <button
        v-for="t in themes" :key="t.value"
        class="prefs-item"
        :class="{ active: prefs.theme === t.value }"
        @click="setTheme(t.value)"
      >
        <svg v-if="prefs.theme === t.value" viewBox="0 0 24 24" width="14" height="14" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round"><path d="M20 6L9 17l-5-5" /></svg>
        <span v-else class="prefs-icon-spacer" />
        <span>{{ t.label }}</span>
      </button>
      <div class="prefs-separator" />
      <div class="prefs-header">弹窗样式</div>
      <button
        v-for="m in panelModes" :key="m.value"
        class="prefs-item"
        :class="{ active: prefs.panelMode === m.value }"
        @click="setPanelMode(m.value)"
      >
        <svg v-if="prefs.panelMode === m.value" viewBox="0 0 24 24" width="14" height="14" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round"><path d="M20 6L9 17l-5-5" /></svg>
        <span v-else class="prefs-icon-spacer" />
        <span>{{ m.label }}</span>
      </button>
    </div>
  </div>
</template>

<style scoped>
.prefs-menu-wrap {
  position: relative;
}
.prefs-menu {
  position: absolute;
  bottom: 100%;
  left: 0;
  margin-bottom: 0.375rem;
  min-width: 180px;
  padding: 0.25rem 0;
  background: var(--color-card-bg);
  border: 1px solid var(--color-line);
  border-radius: 0.5rem;
  box-shadow: var(--shadow-lg);
  z-index: 100;
}
.prefs-header {
  padding: 0.5rem 0.75rem 0.25rem;
  font-size: 0.6875rem;
  font-weight: 550;
  color: var(--color-ink-muted);
  text-transform: uppercase;
  letter-spacing: 0.04em;
}
.prefs-item {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  width: 100%;
  padding: 0.375rem 0.75rem;
  background: none;
  border: none;
  font-size: 0.8125rem;
  color: var(--color-ink);
  cursor: pointer;
  text-align: left;
  transition: background 0.1s ease;
}
.prefs-item:hover {
  background: var(--color-surface-50);
}
.prefs-item.active {
  color: var(--color-accent-ink);
  font-weight: 500;
}
.prefs-item svg {
  color: var(--color-accent);
  flex-shrink: 0;
}
.prefs-icon-spacer {
  display: inline-block;
  width: 14px;
  height: 14px;
  flex-shrink: 0;
}
.prefs-separator {
  height: 1px;
  margin: 0.25rem 0.5rem;
  background: var(--color-line);
}
</style>
