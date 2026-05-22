<script setup lang="ts">
import { computed, ref } from 'vue'
import { VisXYContainer, VisArea, VisAxis, VisCrosshair, VisTooltip } from '@unovis/vue'
import { Tag } from '@/ui'
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
  buckets: string[]
  points: SeriesPoint[]
  height?: number
  valueFormat?: (value: number) => string
  bucketFormat?: (iso: string) => string
}>()

interface Datum {
  bucket: string
  bucketIndex: number
  values: Record<string, number>
}

const hiddenKeys = ref<Set<string>>(new Set())

function toggleSeries(key: string) {
  // Clicking the only visible series — restore all instead of hiding it
  if (hiddenKeys.value.size === props.groups.length - 1 && !hiddenKeys.value.has(key)) {
    hiddenKeys.value = new Set()
    return
  }
  const next = new Set(hiddenKeys.value)
  if (next.has(key)) next.delete(key)
  else next.add(key)
  hiddenKeys.value = next
}

function isolateSeries(key: string) {
  if (hiddenKeys.value.size === props.groups.length - 1 && !hiddenKeys.value.has(key)) {
    // Already isolated on this series — restore all
    hiddenKeys.value = new Set()
  } else {
    // Hide everything except this key
    const next = new Set(props.groups.map((g) => g.key))
    next.delete(key)
    hiddenKeys.value = next
  }
}

const visibleGroups = computed(() => props.groups.filter((g) => !hiddenKeys.value.has(g.key)))
const visibleColors = computed(() => {
  const idxMap = new Map(props.groups.map((g, i) => [g.key, i]))
  return visibleGroups.value.map((g) => {
    const i = idxMap.get(g.key)
    return i !== undefined ? colors.value[i] : colors.value[0]
  })
})

const dataset = computed<Datum[]>(() => {
  const indexByBucket = new Map<string, number>()
  props.buckets.forEach((b, i) => indexByBucket.set(b, i))
  const rows: Datum[] = props.buckets.map((b, i) => ({
    bucket: b,
    bucketIndex: i,
    values: Object.fromEntries(props.groups.map((g) => [g.key, 0])),
  }))
  for (const point of props.points) {
    const idx = indexByBucket.get(point.bucketAt)
    if (idx === undefined) continue
    const row = rows[idx]
    if (!row) continue
    row.values[point.groupKey] = (row.values[point.groupKey] ?? 0) + (point.value ?? 0)
  }
  return rows
})

const colors = computed(() => props.groups.map((_, i) => groupColor(i)))

const accessorsX = (d: Datum) => d.bucketIndex
const accessorsY = computed(() => visibleGroups.value.map((g) => (d: Datum) => d.values[g.key] ?? 0))

const xTickFormat = (i: number | { valueOf(): number }) => {
  const idx = typeof i === 'number' ? i : Number(i)
  const iso = props.buckets[idx]
  if (!iso) return ''
  return props.bucketFormat ? props.bucketFormat(iso) : defaultBucketFormat(iso)
}

const yTickFormat = (v: number) => (props.valueFormat ? props.valueFormat(v) : compactNumber(v))

function defaultBucketFormat(iso: string) {
  const d = new Date(iso)
  if (Number.isNaN(d.getTime())) return iso
  const total = props.buckets.length
  if (total <= 24) {
    return `${d.getHours().toString().padStart(2, '0')}:00`
  }
  return `${d.getMonth() + 1}/${d.getDate()}`
}

function compactNumber(v: number) {
  if (!Number.isFinite(v)) return ''
  if (Math.abs(v) >= 1e9) return `${(v / 1e9).toFixed(1)}B`
  if (Math.abs(v) >= 1e6) return `${(v / 1e6).toFixed(1)}M`
  if (Math.abs(v) >= 1e3) return `${(v / 1e3).toFixed(1)}k`
  if (Math.abs(v) >= 1) return v.toFixed(0)
  if (v === 0) return '0'
  return v.toFixed(2)
}

function tooltipTemplate(datum: Datum | undefined) {
  if (!datum) return ''
  const lines = visibleGroups.value
    .map((g) => {
      const originalIdx = props.groups.findIndex((pg) => pg.key === g.key)
      const color = originalIdx >= 0 ? colors.value[originalIdx] : colors.value[0]
      const v = datum.values[g.key] ?? 0
      const formatted = props.valueFormat ? props.valueFormat(v) : compactNumber(v)
      return {
        ...g,
        html: `<div class="flex items-center gap-1 text-2xs"><span style="background:${color};display:inline-block;width:8px;height:8px;border-radius:2px"></span><span class="text-ink-muted">${escape(g.label || '-')}</span><span class="ml-auto mono tabular">${formatted}</span></div>`,
      }
    })
    .filter((g) => datum.values[g.key] != null && datum.values[g.key] !== 0)
    .map((g) => g.html)
    .join('')
  const bucket = props.bucketFormat
    ? props.bucketFormat(datum.bucket)
    : defaultBucketFormat(datum.bucket)
  const head = `<div class="text-2xs text-ink-muted mb-1">${escape(bucket)}</div>`
  return `<div class="min-w-32">${head}${lines}</div>`
}

function escape(s: string) {
  return s.replace(/[&<>"']/g, (c) =>
    c === '&' ? '&amp;' : c === '<' ? '&lt;' : c === '>' ? '&gt;' : c === '"' ? '&quot;' : '&#39;',
  )
}
</script>

<template>
  <div class="flex flex-col gap-2">
    <div :style="{ height: (height ?? 180) + 'px' }">
      <VisXYContainer
        :data="dataset"
        :height="height ?? 180"
        :margin="{ left: 32, right: 8, top: 8, bottom: 24 }"
      >
        <VisArea
          :x="accessorsX"
          :y="accessorsY"
                    :color="visibleColors"
          :curve-type="'monotoneX'"
          :opacity="0.85"
        />
        <VisAxis type="x" :tick-format="xTickFormat" :grid-line="false" :num-ticks="6" />
        <VisAxis type="y" :tick-format="yTickFormat" :num-ticks="4" />
        <VisCrosshair :template="tooltipTemplate" />
        <VisTooltip />
      </VisXYContainer>
    </div>
    <ul class="flex flex-wrap gap-1">
      <li
        v-for="(g, i) in groups"
        :key="g.key || `__${i}`"
        class="flex items-center gap-1 cursor-pointer select-none"
        :class="{ 'opacity-30': hiddenKeys.has(g.key) }"
        @click="toggleSeries(g.key)"
        @contextmenu.prevent="isolateSeries(g.key)"
      >
        <span class="h-2 w-2 shrink-0 rounded-xs" :style="{ background: colors[i] }" />
        <Tag variant="default">{{ g.label || '—' }}</Tag>
      </li>
    </ul>
  </div>
</template>
