<script setup lang="ts">
import { computed } from 'vue'
import { useCurrencyContext } from '@/composables/useCurrencyContext'

const props = withDefaults(
  defineProps<{
    amount?: number | null
    currency?: string | null
    fallback?: string
    /** Display options forwarded to format() */
    minDigits?: number
    maxDigits?: number
  }>(),
  { fallback: '—' },
)

const ccy = useCurrencyContext()

const display = computed(() => {
  if (props.amount == null || !props.currency) return null
  const result = ccy.convert(props.amount, props.currency)
  const text = ccy.format(result.amount, result.currency, {
    minDigits: props.minDigits,
    maxDigits: props.maxDigits,
  })
  let title: string | undefined
  if (result.converted) {
    title = ccy.format(result.original.amount, result.original.currency, {
      minDigits: props.minDigits,
      maxDigits: props.maxDigits,
    })
  }
  return { text, title }
})
</script>

<template>
  <span v-if="display" :title="display.title" class="tabular-nums">{{ display.text }}</span>
  <span v-else class="text-ink-faint">{{ fallback }}</span>
</template>
