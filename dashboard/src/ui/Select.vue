<script setup lang="ts">
import { computed } from 'vue'

const props = withDefaults(
  defineProps<{
    modelValue?: string | number
    modelModifiers?: { number?: boolean }
    size?: 'sm' | 'md'
  }>(),
  { size: 'md', modelModifiers: () => ({}) },
)

const emit = defineEmits<{ 'update:modelValue': [string | number] }>()

defineOptions({ inheritAttrs: true })

const sizeClass = computed(() =>
  props.size === 'sm' ? 'px-2 py-1.5 text-sm' : 'px-3 py-2 text-sm',
)

function onChange(e: Event) {
  let v: string | number = (e.target as HTMLSelectElement).value
  if (props.modelModifiers?.number) {
    const n = Number(v)
    v = Number.isNaN(n) ? (v as string) : n
  }
  emit('update:modelValue', v)
}
</script>

<template>
  <select
    :value="modelValue"
    class="border border-line rounded-md bg-surface-0 text-ink font-sans transition-colors hover:border-surface-300 focus:outline-none focus:border-accent disabled:opacity-55 disabled:cursor-not-allowed"
    :class="sizeClass"
    @change="onChange"
  >
    <slot />
  </select>
</template>
