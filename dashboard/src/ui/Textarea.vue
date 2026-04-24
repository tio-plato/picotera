<script setup lang="ts">
withDefaults(
  defineProps<{
    modelValue?: string
    mono?: boolean
  }>(),
  { mono: false },
)

defineEmits<{ 'update:modelValue': [string] }>()

defineOptions({ inheritAttrs: true })
</script>

<template>
  <textarea
    :value="modelValue"
    class="w-full px-2.5 py-2 border border-line rounded-md bg-surface-0 text-ink text-sm leading-[1.55] transition-colors resize-y min-h-24 focus:outline-none focus:border-accent focus:ring-[3px] focus:ring-accent/20 placeholder:text-ink-faint"
    :class="mono ? 'font-mono' : 'font-sans'"
    @input="$emit('update:modelValue', ($event.target as HTMLTextAreaElement).value)"
  />
</template>
