<script setup lang="ts">
import { computed } from 'vue'

const props = withDefaults(
  defineProps<{
    variant?: 'primary' | 'ghost' | 'danger'
    size?: 'sm' | 'md'
    type?: 'button' | 'submit' | 'reset'
    disabled?: boolean
  }>(),
  { variant: 'primary', size: 'md', type: 'button', disabled: false },
)

const variantClass = computed(() => {
  switch (props.variant) {
    case 'ghost':
      return 'bg-surface-0 text-ink-muted border-line hover:bg-surface-50 hover:text-ink hover:border-surface-300 shadow-xs'
    case 'danger':
      return 'bg-err text-white border-transparent hover:opacity-92 shadow-xs'
    default:
      return 'bg-accent text-white border-transparent hover:bg-accent-strong active:translate-y-[0.5px] shadow-xs'
  }
})

const sizeClass = computed(() =>
  props.size === 'sm' ? 'px-3 py-1.5 text-sm' : 'px-3.5 py-2 text-sm',
)
</script>

<template>
  <button
    :type="type"
    :disabled="disabled"
    class="inline-flex items-center gap-1.5 rounded-md border font-medium leading-none cursor-pointer transition-colors focus-visible:outline-2 focus-visible:outline-accent focus-visible:outline-offset-2 disabled:opacity-50 disabled:cursor-not-allowed"
    :class="[variantClass, sizeClass]"
  >
    <slot />
  </button>
</template>
