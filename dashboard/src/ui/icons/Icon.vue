<script setup lang="ts">
import { iconPaths, type IconName } from './paths'

const props = defineProps<{
  name: IconName
  size?: number | string
  strokeWidth?: number | string
}>()

const sz = () => props.size ?? 14
</script>

<template>
  <svg
    :width="sz()"
    :height="sz()"
    viewBox="0 0 24 24"
    fill="none"
    stroke="currentColor"
    :stroke-width="strokeWidth ?? 1.8"
    stroke-linecap="round"
    stroke-linejoin="round"
    aria-hidden="true"
    v-html="iconPaths[name]"
  />
</template>
