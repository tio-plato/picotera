<script setup lang="ts">
import { computed } from 'vue'

const props = withDefaults(
  defineProps<{
    modelValue?: string | number
    modelModifiers?: { number?: boolean; trim?: boolean; lazy?: boolean }
    size?: 'sm' | 'md'
    type?: string
    mono?: boolean
  }>(),
  { size: 'md', type: 'text', mono: false, modelModifiers: () => ({}) },
)

const emit = defineEmits<{ 'update:modelValue': [string | number] }>()

defineOptions({ inheritAttrs: true })

const sizeClass = computed(() =>
  props.size === 'sm' ? 'px-2 py-1.5 text-sm' : 'px-3 py-2 text-sm',
)

function onInput(e: Event) {
  let v: string | number = (e.target as HTMLInputElement).value
  if (props.modelModifiers?.trim) v = (v as string).trim()
  if (props.modelModifiers?.number) {
    const n = Number(v)
    v = Number.isNaN(n) ? (v as string) : n
  }
  emit('update:modelValue', v)
}
</script>

<template>
  <input
    :type="type"
    :value="modelValue"
    class="border border-line rounded-md bg-surface-0 text-ink font-sans transition-colors hover:border-surface-300 focus:outline-none focus:border-accent focus:ring-[3px] focus:ring-accent/20 disabled:opacity-55 disabled:cursor-not-allowed placeholder:text-ink-faint"
    :class="[sizeClass, mono ? 'font-mono' : '']"
    @input="onInput"
  />
</template>
