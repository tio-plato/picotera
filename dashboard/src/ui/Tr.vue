<script setup lang="ts">
defineProps<{ selected?: boolean; hoverable?: boolean; isNew?: boolean }>()
</script>

<template>
  <tr
    class="group transition-colors"
    :class="[
      hoverable !== false ? 'hover:bg-surface-50' : '',
      selected
        ? 'bg-surface-50 shadow-[inset_2px_0_0_var(--color-accent)] hover:bg-surface-100'
        : '',
      isNew ? 'animate-slide-up' : '',
    ]"
  >
    <slot />
  </tr>
</template>
