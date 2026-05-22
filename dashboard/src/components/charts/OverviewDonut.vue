<script setup lang="ts">
import { computed, ref } from 'vue'
import { VisSingleContainer, VisDonut, VisTooltip, VisDonutSelectors } from '@unovis/vue'
import { Tag } from '@/ui'
import { groupColor } from './colors'

interface DonutDatum {
  key: string
  label: string
  value: number
}

interface DonutArcDatum {
  data?: DonutDatum & { _color: string }
  value?: number
}

const props = defineProps<{
  data: DonutDatum[]
  centralLabel?: string
  centralSubLabel?: string
  valueFormat?: (value: number, datum: DonutDatum) => string
}>()

const hiddenKeys = ref<Set<string>>(new Set())

function toggleSeries(key: string) {
  if (hiddenKeys.value.size === props.data.length - 1 && !hiddenKeys.value.has(key)) {
    hiddenKeys.value = new Set()
    return
  }
  const next = new Set(hiddenKeys.value)
  if (next.has(key)) next.delete(key)
  else next.add(key)
  hiddenKeys.value = next
}

function isolateSeries(key: string) {
  if (hiddenKeys.value.size === props.data.length - 1 && !hiddenKeys.value.has(key)) {
    hiddenKeys.value = new Set()
  } else {
    const next = new Set(props.data.map((d) => d.key))
    next.delete(key)
    hiddenKeys.value = next
  }
}

const total = computed(() => props.data.reduce((acc, d) => acc + (d.value ?? 0), 0))

const colored = computed(() => props.data.map((d, i) => ({ ...d, _color: groupColor(i) })))

const visibleData = computed(() => colored.value.filter((d) => !hiddenKeys.value.has(d.key)))

const value = (d: DonutDatum) => d.value
const colorFn = (d: DonutDatum & { _color: string }) => d._color

const tooltipTriggers = computed(() => ({
  [VisDonutSelectors.segment]: (d: unknown) => {
    const arc = d as DonutArcDatum | null
    const datum = arc?.data
    if (!datum) return ''
    const value = datum.value ?? arc?.value ?? 0
    const formatted = props.valueFormat ? props.valueFormat(value, datum) : String(value)
    const pct = total.value === 0 ? '0' : ((value / total.value) * 100).toFixed(1)
    return `<div class="text-xs"><div class="font-medium">${escape(datum.label)}</div><div class="mono tabular text-ink-muted">${formatted} · ${pct}%</div></div>`
  },
}))

function escape(s: string) {
  return s.replace(/[&<>"']/g, (c) =>
    c === '&' ? '&amp;' : c === '<' ? '&lt;' : c === '>' ? '&gt;' : c === '"' ? '&quot;' : '&#39;',
  )
}
</script>

<template>
  <div class="flex flex-col gap-4">
    <div class="relative h-[180px]">
      <VisSingleContainer :data="visibleData" :height="180">
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
      <li
        v-for="d in colored"
        :key="d.key"
        class="flex items-center gap-1 cursor-pointer select-none"
        :class="{ 'opacity-30': hiddenKeys.has(d.key) }"
        @click="toggleSeries(d.key)"
        @contextmenu.prevent="isolateSeries(d.key)"
      >
        <span class="h-2 w-2 shrink-0 rounded-xs" :style="{ background: d._color }" />
        <Tag variant="default">{{ d.label }}</Tag>
        <span class="mono tabular text-ink-muted text-2xs">
          {{ valueFormat ? valueFormat(d.value, d) : d.value }}
        </span>
      </li>
    </ul>
  </div>
</template>
