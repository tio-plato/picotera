<script setup lang="ts">
import { computed } from 'vue'

type Option<T extends string | number> = { value: T; label: string }

const props = withDefaults(
  defineProps<{
    modelValue: string | number
    options: Option<string | number>[]
    columns?: number
  }>(),
  { columns: 0 },
)

const emit = defineEmits<{ 'update:modelValue': [string | number] }>()

const gridStyle = computed(() => {
  const cols = props.columns || props.options.length
  return { gridTemplateColumns: `repeat(${cols}, 1fr)` }
})

function select(v: string | number) {
  if (v !== props.modelValue) emit('update:modelValue', v)
}
</script>

<template>
  <div
    class="grid gap-0.5 p-0.5 bg-surface-50 border border-line rounded-md"
    :style="gridStyle"
    role="radiogroup"
  >
    <button
      v-for="opt in options"
      :key="opt.value"
      type="button"
      role="radio"
      :aria-checked="modelValue === opt.value"
      class="px-2 py-1.5 text-xs font-medium rounded-sm cursor-pointer transition-colors"
      :class="
        modelValue === opt.value
          ? 'bg-surface-0 text-accent-ink shadow-[0_0_0_1px_var(--color-line),0_1px_2px_oklch(0_0_0/0.06)]'
          : 'bg-transparent text-ink-muted hover:text-ink'
      "
      @click="select(opt.value)"
    >
      {{ opt.label }}
    </button>
  </div>
</template>
