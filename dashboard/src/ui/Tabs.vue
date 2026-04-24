<script setup lang="ts">
import Icon from './icons/Icon.vue'
import type { IconName } from './icons/paths'

type Tab<T extends string | number> = { value: T; label: string; icon?: IconName }

const props = defineProps<{
  modelValue: string | number
  tabs: Tab<string | number>[]
}>()

const emit = defineEmits<{ 'update:modelValue': [string | number] }>()

function select(v: string | number) {
  if (v !== props.modelValue) emit('update:modelValue', v)
}
</script>

<template>
  <div class="inline-flex gap-0.5 bg-surface-100 p-0.5 rounded-md" role="tablist">
    <button
      v-for="tab in tabs"
      :key="tab.value"
      type="button"
      role="tab"
      :aria-selected="modelValue === tab.value"
      class="inline-flex items-center gap-2 px-2 py-1 text-xs font-medium rounded-sm cursor-pointer transition-colors"
      :class="
        modelValue === tab.value
          ? 'bg-surface-0 text-ink shadow-xs [&_svg]:text-accent [&_svg]:opacity-100'
          : 'bg-transparent text-ink-muted hover:text-ink [&_svg]:opacity-70'
      "
      @click="select(tab.value)"
    >
      <Icon v-if="tab.icon" :name="tab.icon" :size="11" />
      {{ tab.label }}
    </button>
  </div>
</template>
