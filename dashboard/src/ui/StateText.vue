<script setup lang="ts">
withDefaults(
  defineProps<{
    dashed?: boolean
    compact?: boolean
  }>(),
  { dashed: true, compact: false },
)
</script>

<template>
  <div
    class="flex items-center justify-center text-ink-faint bg-surface-0 border rounded-xl"
    :class="[
      dashed ? 'border-dashed border-line' : 'border-solid border-line',
      compact ? 'py-2 text-sm' : 'py-14 px-4 text-sm',
    ]"
  >
    <slot />
  </div>
</template>
