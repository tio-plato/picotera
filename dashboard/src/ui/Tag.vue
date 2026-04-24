<script setup lang="ts">
import { computed } from 'vue'

const props = withDefaults(
  defineProps<{
    variant?: 'default' | 'accent' | 'ok' | 'muted' | 'more'
  }>(),
  { variant: 'default' },
)

const variantClass = computed(() => {
  switch (props.variant) {
    case 'accent':
      return 'bg-accent-faint text-accent-ink border-transparent'
    case 'ok':
      return 'bg-ok-faint text-ok-ink border-transparent'
    case 'muted':
      return 'bg-surface-100 text-ink-faint border-transparent'
    case 'more':
      return 'bg-transparent text-ink-faint border-line'
    default:
      return 'bg-surface-100 text-ink-muted border-transparent'
  }
})
</script>

<template>
  <span
    class="inline-flex items-center px-1.5 py-0.5 border rounded-[5px] font-mono text-2xs leading-[1.2] whitespace-nowrap"
    :class="variantClass"
  >
    <slot />
  </span>
</template>
