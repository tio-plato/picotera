<script setup lang="ts">
import { computed } from 'vue'
import { VisXYContainer, VisTimeline, VisAxis, VisCrosshair, VisTooltip, VisXYLabels } from '@unovis/vue'
import { groupColor } from './colors'

interface SeriesGroup {
  key: string
  label: string
}

interface SeriesPoint {
  groupKey: string
  bucketAt: string
  value: number
}

const props = defineProps<{
  groups: SeriesGroup[]
  points: SeriesPoint[]
  height?: number
  valueFormat?: (value: number) => string
}>()

interface TimelineDatum {
  row: string
  x: number
  length: number
  color: string
  min: number
  max: number
}

const colors = computed(() => props.groups.map((_, i) => groupColor(i) ?? 'var(--color-accent)'))

const dataset = computed<TimelineDatum[]>(() => {
  const groupPoints = new Map<string, number[]>()
  for (const p of props.points) {
    if (!groupPoints.has(p.groupKey)) groupPoints.set(p.groupKey, [])
    groupPoints.get(p.groupKey)!.push(p.value)
  }
  return props.groups.map((g, i) => {
    const values = groupPoints.get(g.key) ?? []
    const color = colors.value[i] ?? 'var(--color-accent)'
    if (values.length === 0) {
      return {
        row: g.label || '-',
        x: 0,
        length: 0,
        color,
        min: 0,
        max: 0,
      }
    }
    const min = Math.min(...values)
    const max = Math.max(...values)
    return {
      row: g.label || '-',
      x: min,
      length: max - min,
      color,
      min,
      max,
    }
  })
})

const noData = computed(() => dataset.value.every((d) => d.length === 0))

const lineRow = (d: TimelineDatum) => d.row
const lineX = (d: TimelineDatum) => d.x
const lineDuration = (d: TimelineDatum) => d.length
const lineColor = (d: TimelineDatum) => d.color

const xTickFormat = (v: number) => (props.valueFormat ? props.valueFormat(v) : `${v.toFixed(0)} tok/s`)

function escape(s: string) {
  return s.replace(/[&<>"']/g, (c) =>
    c === '&' ? '&amp;' : c === '<' ? '&lt;' : c === '>' ? '&gt;' : c === '"' ? '&quot;' : '&#39;',
  )
}

function tooltipTemplate(d: TimelineDatum | undefined) {
  if (!d) return ''
  const fmt = props.valueFormat ?? ((v: number) => `${v.toFixed(0)} tok/s`)
  return `<div class="min-w-32">
    <div class="text-2xs text-ink-muted mb-1">${escape(d.row)}</div>
    <div class="flex items-center gap-1 text-2xs">
      <span style="background:${d.color};display:inline-block;width:8px;height:8px;border-radius:2px"></span>
      <span class="mono tabular">${escape(fmt(d.min))} — ${escape(fmt(d.max))}</span>
    </div>
  </div>`
}

function formatLabel(key: string, items: any[], i: number) {
  return `${items?.[0]?.row ?? i}`
}
</script>

<template>
  <div class="flex flex-col gap-2">
    <div v-if="noData" class="text-2xs text-ink-muted">暂无数据</div>
    <div v-else>
      <VisXYContainer
        :data="dataset"
        :height="height ?? 200"
      >
        <VisTimeline
          :x="lineX"
          :line-row="lineRow"
          :line-duration="lineDuration"
          :color="lineColor"
          :showLabels="false"
          :rowHeight="20"
          :lineWidth="12"
          :rowLabelFormatter="formatLabel"
        />
        <VisAxis type="x" :tick-format="xTickFormat" :grid-line="true" :num-ticks="6" />
        <VisCrosshair :template="tooltipTemplate" />
        <VisTooltip />
      </VisXYContainer>
    </div>
  </div>
</template>
