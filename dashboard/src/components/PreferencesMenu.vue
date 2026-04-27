<script setup lang="ts">
import { ref, useTemplateRef, watch, onBeforeUnmount } from 'vue'
import { useFloating, offset, flip, shift, autoUpdate } from '@floating-ui/vue'
import { usePreferencesStore } from '@/stores/preferences'
import type { Theme, PanelMode, FontSize } from '@/stores/preferences'
import Icon from '@/ui/icons/Icon.vue'
import SegmentedControl from '@/ui/SegmentedControl.vue'

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

const fontSizes: { value: FontSize; label: string }[] = [
  { value: 'tall',    label: 'Tall' },
  { value: 'grande',  label: 'Grande' },
  { value: 'venti',   label: 'Venti' },
  { value: 'trenta',  label: 'Trenta' },
]
</script>

<template>
  <button
    ref="triggerRef"
    type="button"
    aria-label="设置"
    title="设置"
    :aria-expanded="open"
    aria-haspopup="menu"
    class="inline-flex items-center justify-center w-7 h-7 p-0 bg-transparent text-ink-muted border border-transparent rounded-md cursor-pointer transition-colors hover:bg-sidebar-hover hover:text-ink aria-expanded:bg-sidebar-active-bg aria-expanded:text-sidebar-active-text aria-expanded:border-line"
    @click="toggle"
  >
    <Icon name="settings" :size="14" />
  </button>

  <Teleport to="body">
    <div
      v-if="open"
      ref="floatingRef"
      class="w-60 p-1.5 bg-surface-0 border border-line rounded-xl shadow-lg z-[1000] text-ink"
      role="menu"
      :style="floatingStyles"
    >
      <section class="px-1 pt-1.5 pb-2">
        <h3 class="m-0 mb-2 px-1.5 text-2xs font-medium tracking-[0.06em] uppercase text-ink-faint">外观</h3>
        <ul class="list-none p-0 m-0 flex flex-col gap-px" role="radiogroup" aria-label="外观主题">
          <li v-for="t in themes" :key="t.value">
            <button
              type="button"
              role="radio"
              :aria-checked="prefs.theme === t.value"
              class="grid grid-cols-[auto_1fr_auto] items-center gap-2.5 w-full px-2 py-1.5 bg-transparent border border-transparent rounded-md text-sm text-left cursor-pointer transition-colors hover:bg-surface-50"
              :class="
                prefs.theme === t.value
                  ? 'bg-accent-faint border-accent/25 text-accent-ink font-medium'
                  : ''
              "
              @click="setTheme(t.value)"
            >
              <span
                class="relative inline-block w-5 h-5 rounded-full flex-none"
                :style="{
                  background: `linear-gradient(90deg, ${t.surface} 0 50%, ${t.accent} 50% 100%)`,
                  boxShadow:
                    prefs.theme === t.value
                      ? 'inset 0 0 0 1px color-mix(in oklch, var(--color-ink) 22%, transparent), 0 0 0 2px var(--color-surface-0), 0 0 0 3px var(--color-accent)'
                      : 'inset 0 0 0 1px color-mix(in oklch, var(--color-ink) 18%, transparent)',
                }"
                aria-hidden="true"
              />
              <span class="min-w-0 overflow-hidden text-ellipsis whitespace-nowrap">{{ t.label }}</span>
              <span
                v-if="prefs.theme === t.value"
                class="inline-block w-1.5 h-1.5 rounded-full bg-accent"
                aria-hidden="true"
              />
            </button>
          </li>
        </ul>
      </section>

      <hr class="m-0 h-px border-0 bg-line-soft" />

      <section class="px-1 pt-1.5 pb-2">
        <h3 class="m-0 mb-2 px-1.5 text-2xs font-medium tracking-[0.06em] uppercase text-ink-faint">面板位置</h3>
        <SegmentedControl v-model="prefs.panelMode" :options="panelModes" :columns="3" />
      </section>

      <hr class="m-0 h-px border-0 bg-line-soft" />

      <section class="px-1 pt-1.5 pb-2">
        <h3 class="m-0 mb-2 px-1.5 text-2xs font-medium tracking-[0.06em] uppercase text-ink-faint">字体大小</h3>
        <SegmentedControl v-model="prefs.fontSize" :options="fontSizes" :columns="4" />
      </section>
    </div>
  </Teleport>
</template>
