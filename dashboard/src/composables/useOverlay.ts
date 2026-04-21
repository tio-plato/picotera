import { ref, shallowRef, type Component } from 'vue'

const visible = ref(false)
const component = shallowRef<Component | null>(null)
const props = ref<Record<string, any>>({})

function open(comp: Component, p: Record<string, any> = {}) {
  component.value = comp
  props.value = p
  visible.value = true
}

function close() {
  visible.value = false
  component.value = null
  props.value = {}
}

export function useOverlay() {
  return { visible, component, props, open, close }
}
