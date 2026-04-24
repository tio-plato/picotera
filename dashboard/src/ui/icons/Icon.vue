<script setup lang="ts">
import { computed } from 'vue'
import { iconComponents, type IconName } from './paths'

const props = defineProps<{
  name: IconName
  size?: number | string
  strokeWidth?: number | string
}>()

const component = computed(() => iconComponents[props.name])
</script>

<template>
  <component
    :is="component"
    :size="props.size ?? 14"
    :stroke-width="props.strokeWidth ?? 1.8"
    aria-hidden="true"
  />
</template>
