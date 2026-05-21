import { shallowRef, computed, type Component, type ComponentPublicInstance } from 'vue'

export type SidePanelKey = string | number | symbol | Component

interface SidePanelState {
  key: SidePanelKey
  component: Component
  props: ComponentPublicInstance['$props']
  width: string
}

const state = shallowRef<SidePanelState | null>(null)
const visible = computed(() => state.value !== null)
const activeKey = computed(() => state.value?.key ?? null)

interface OpenOptions {
  key?: SidePanelKey
  width?: string
}

function open(
  comp: Component,
  props: ComponentPublicInstance['$props'] = {},
  options: OpenOptions = {},
) {
  state.value = {
    key: options.key ?? comp,
    component: comp,
    props,
    width: options.width ?? '420px',
  }
}

function close() {
  state.value = null
}

function toggle(
  comp: Component,
  props: ComponentPublicInstance['$props'] = {},
  options: OpenOptions = {},
) {
  const key = options.key ?? comp
  if (state.value?.key === key) {
    close()
    return false
  }
  open(comp, props, options)
  return true
}

function isActive(key: SidePanelKey) {
  return state.value?.key === key
}

export function useSidePanel() {
  return { state, visible, activeKey, open, close, toggle, isActive }
}
