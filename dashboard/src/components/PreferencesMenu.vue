<script setup lang="ts">
import { ref, computed } from 'vue'
import Menu from 'primevue/menu'
import { usePreferencesStore } from '@/stores/preferences'
import type { Theme, PanelMode } from '@/stores/preferences'

const prefs = usePreferencesStore()
const menuRef = ref<InstanceType<typeof Menu> | null>(null)

function toggle(event: Event) {
  menuRef.value?.toggle(event)
}

const check = 'pi size-4 pi-check'
const empty = 'pi size-4'

const items = computed(() => [
  { label: '主题', disabled: true, class: 'prefs-header' },
  { label: 'Pico Light', icon: prefs.theme === 'light' ? check : empty, command: () => { prefs.theme = 'light' } },
  { label: 'Solarized Light', icon: prefs.theme === 'solarized-light' ? check : empty, command: () => { prefs.theme = 'solarized-light' } },
  { label: 'Solarized Dark', icon: prefs.theme === 'solarized-dark' ? check : empty, command: () => { prefs.theme = 'solarized-dark' } },
  { label: 'Tera Dark', icon: prefs.theme === 'dark' ? check : empty, command: () => { prefs.theme = 'dark' } },
  { separator: true },
  { label: '弹窗样式', disabled: true, class: 'prefs-header' },
  { label: '自动', icon: prefs.panelMode === 'auto' ? check : empty, command: () => { prefs.panelMode = 'auto' } },
  { label: '右侧', icon: prefs.panelMode === 'right' ? check : empty, command: () => { prefs.panelMode = 'right' } },
  { label: '弹窗', icon: prefs.panelMode === 'modal' ? check : empty, command: () => { prefs.panelMode = 'modal' } },
])

defineExpose({ toggle })
</script>

<template>
  <Menu ref="menuRef" :model="items" :popup="true" class="prefs-menu" />
</template>

<style scoped>
.prefs-menu :deep(.prefs-header) {
  font-size: 0.6875rem;
  font-weight: 550;
  color: var(--color-ink-muted);
  text-transform: uppercase;
  letter-spacing: 0.04em;
  opacity: 1;
}
</style>
