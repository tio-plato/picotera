<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from 'vue'
import { useSidePanel } from '@/composables/useSidePanel'
import { usePreferencesStore } from '@/stores/preferences'

const { state, close } = useSidePanel()
const prefs = usePreferencesStore()
const cssWidth = computed(() => state.value?.width ?? '420px')

const narrow = ref(false)
let mql: MediaQueryList | null = null
function onChange(e: MediaQueryListEvent) {
  narrow.value = e.matches
}
onMounted(() => {
  mql = window.matchMedia('(max-width: 960px)')
  narrow.value = mql.matches
  mql.addEventListener('change', onChange)
})
onUnmounted(() => {
  mql?.removeEventListener('change', onChange)
})

const mode = computed<'right' | 'modal'>(() => {
  if (prefs.panelMode === 'right') return 'right'
  if (prefs.panelMode === 'modal') return 'modal'
  return narrow.value ? 'modal' : 'right'
})
</script>

<template>
  <aside
    v-if="state"
    class="side-panel-host"
    :data-mode="mode"
    :style="{ '--side-panel-width': cssWidth }"
  >
    <div class="side-panel-host__backdrop" @click="close" />
    <component
      :is="state.component"
      :key="state.key"
      v-bind="state.props"
      class="side-panel-host__panel"
      @close="close"
    />
  </aside>
</template>

<style scoped>
.side-panel-host {
  flex: 0 0 var(--side-panel-width);
  width: var(--side-panel-width);
  display: flex;
  min-height: 0;
  align-self: stretch;
  padding: 0.75rem 2rem 2rem 0;
}
.side-panel-host__backdrop { display: none; }
.side-panel-host__panel {
  width: 100%;
  max-height: 100%;
  min-height: 0;
}

.side-panel-host[data-mode="modal"] {
  position: fixed;
  inset: 0;
  z-index: 900;
  flex: 0 0 auto;
  width: auto;
  align-items: center;
  justify-content: center;
  padding: 1rem;
}
.side-panel-host[data-mode="modal"] .side-panel-host__backdrop {
  display: block;
  position: absolute;
  inset: 0;
  background: var(--color-overlay-bg);
  backdrop-filter: blur(4px);
}
.side-panel-host[data-mode="modal"] .side-panel-host__panel {
  position: relative;
  width: min(var(--side-panel-width), 100%);
  max-height: calc(100vh - 2rem);
  box-shadow: 0 25px 50px -12px oklch(0.1 0.02 250 / 0.25);
}
</style>
