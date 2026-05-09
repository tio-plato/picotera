<script setup lang="ts">
import { computed } from 'vue'
import { VisSingleContainer, VisDonut, VisTooltip, VisDonutSelectors } from '@unovis/vue'
import { Tag } from '@/ui'
import { groupColor } from './colors'

interface DonutDatum {
  key: string
  label: string
  value: number
}

const props = defineProps<{
  data: DonutDatum[]
  centralLabel?: string
  centralSubLabel?: string
  valueFormat?: (value: number, datum: DonutDatum) => string
}>()

const total = computed(() => props.data.reduce((acc, d) => acc + (d.value ?? 0), 0))

const colored = computed(() =>
  props.data.map((d, i) => ({ ...d, _color: groupColor(i) })),
)

const value = (d: DonutDatum) => d.value
const colorFn = (d: DonutDatum & { _color: string }) => d._color

const tooltipTriggers = computed(() => ({
  [VisDonutSelectors.segment]: (d: unknown) => {
    const datum = d as DonutDatum & { _color: string }
    if (!datum) return ''
    const formatted = props.valueFormat ? props.valueFormat(datum.value, datum) : String(datum.value)
    const pct = total.value === 0 ? '0' : ((datum.value / total.value) * 100).toFixed(1)
    return `<div class="text-xs"><div class="font-medium">${escape(datum.label)}</div><div class="mono tabular text-ink-muted">${formatted} · ${pct}%</div></div>`
  },
}))

function escape(s: string) {
  return s.replace(/[&<>"']/g, (c) =>
    c === '&'
      ? '&amp;'
      : c === '<'
        ? '&lt;'
        : c === '>'
          ? '&gt;'
          : c === '"'
            ? '&quot;'
            : '&#39;',
  )
}
</script>

<template>
  <div class="flex flex-col gap-4">
    <div class="relative h-[180px]">
      <VisSingleContainer :data="colored" :height="180">
        <VisDonut
          :value="value"
          :color="colorFn"
          :arc-width="14"
          :corner-radius="2"
          :pad-angle="0.01"
          :show-background="false"
          :central-label="centralLabel"
          :central-sub-label="centralSubLabel"
        />
        <VisTooltip :triggers="tooltipTriggers" />
      </VisSingleContainer>
    </div>
    <ul class="flex flex-wrap gap-1">
      <li v-for="d in colored" :key="d.key" class="flex items-center gap-1">
        <span
          class="h-2 w-2 shrink-0 rounded-xs"
          :style="{ background: d._color }"
        />
        <Tag variant="default">{{ d.label }}</Tag>
        <span class="mono tabular text-ink-muted text-2xs">
          {{ valueFormat ? valueFormat(d.value, d) : d.value }}
        </span>
      </li>
    </ul>
  </div>
</template>
