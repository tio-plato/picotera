<script setup lang="ts">
defineProps<{ actions?: boolean }>()
</script>

<template>
  <th
    class="text-left py-2.5 px-4 bg-surface-50 text-ink-muted font-medium text-xs border-b border-line whitespace-nowrap first:pl-[1.125rem] last:pr-[1.125rem]"
    :class="actions ? 'w-[1%] text-right' : ''"
  >
    <slot />
  </th>
</template>
