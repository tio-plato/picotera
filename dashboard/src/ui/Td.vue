<script setup lang="ts">
defineProps<{ actions?: boolean }>()
</script>

<template>
  <td
    class="py-3 px-4 border-b border-line-soft align-middle text-ink whitespace-nowrap first:pl-[1.125rem] last:pr-[1.125rem]"
    :class="actions ? 'w-[1%] text-right' : ''"
  >
    <slot />
  </td>
</template>
