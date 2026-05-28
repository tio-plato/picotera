<script setup lang="ts">
import { computed, watch, ref } from 'vue'
import VChart from 'vue-echarts'
import { usePreferencesStore } from '@/stores/preferences'
import { groupColor, getThemeAxisStyle } from './colors'
import type { CallbackDataParams } from 'echarts/types/dist/shared'
import type { EChartsOption } from './echarts'
import './echarts'

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

interface GroupStats {
  label: string
  min: number
  max: number
  hasData: boolean
  colorIndex: number
}

const groupStats = computed<GroupStats[]>(() => {
  const groupPoints = new Map<string, number[]>()
  for (const p of props.points) {
    if (!groupPoints.has(p.groupKey)) groupPoints.set(p.groupKey, [])
    groupPoints.get(p.groupKey)!.push(p.value)
  }
  return props.groups.map((g, i) => {
    const values = groupPoints.get(g.key) ?? []
    if (values.length === 0) {
      return { label: g.label || '-', min: 0, max: 0, hasData: false, colorIndex: i }
    }
    return {
      label: g.label || '-',
      min: Math.min(...values),
      max: Math.max(...values),
      hasData: true,
      colorIndex: i,
    }
  })
})

const noData = computed(() => groupStats.value.every((s) => !s.hasData))

function escape(s: string) {
  return s.replace(/[&<>"']/g, (c) =>
    c === '&' ? '&amp;' : c === '<' ? '&lt;' : c === '>' ? '&gt;' : c === '"' ? '&quot;' : '&#39;',
  )
}

const prefs = usePreferencesStore()
const themeVersion = ref(0)
watch(() => prefs.theme, () => { themeVersion.value++ })

const option = computed<EChartsOption>(() => {
  void themeVersion.value
  const axis = getThemeAxisStyle()
  const fmtValue = props.valueFormat ?? ((v: number, skipUnit = false) => `${v.toFixed(0)}${skipUnit ? '' : ' tok/s'}`)
  const reversed = [...groupStats.value].reverse()
  const labels = reversed.map((s) => s.label)

  return {
    animation: false,
    grid: { left: 8, right: 16, top: 8, bottom: 24, containLabel: true },
    yAxis: {
      type: 'category',
      data: labels,
      axisLine: { show: false },
      axisTick: { show: false },
      axisLabel: { color: axis.axisLabel, fontSize: 10 },
    },
    xAxis: {
      type: 'value',
      axisLabel: { color: axis.axisLabel, fontSize: 10, formatter: (v: number) => fmtValue(v) },
      splitLine: { lineStyle: { color: axis.splitLine } },
      axisLine: { show: false },
      axisTick: { show: false },
    },
    tooltip: {
      trigger: 'item',
      backgroundColor: axis.tooltipBg,
      borderColor: axis.tooltipBorder,
      textStyle: { color: axis.tooltipText, fontSize: 10 },
      formatter: (params: CallbackDataParams | CallbackDataParams[]) => {
        const p = Array.isArray(params) ? params[0]! : params
        const d = p.data as Record<string, unknown> | undefined
        if (!d) return ''
        const stat = d._stat as GroupStats
        const color = groupColor(stat.colorIndex)
        return `<div class="min-w-32">
          <div class="text-2xs text-ink-muted mb-1">${escape(stat.label)}</div>
          <div class="flex items-center gap-1 text-2xs">
            <span style="background:${color};display:inline-block;width:8px;height:8px;border-radius:2px"></span>
            <span class="mono tabular">${escape(fmtValue(stat.min, true))} ~ ${escape(fmtValue(stat.max))}</span>
          </div>
        </div>`
      },
    },
    series: [
      {
        type: 'boxplot',
        itemStyle: { borderWidth: 0 },
        emphasis: { focus: 'self' },
        blur: {
          itemStyle: { opacity: 0.4 },
        },
        data: reversed.map((s) => {
          const median = s.hasData ? (s.min + s.max) / 2 : 0
          return {
            value: [s.min, s.min, median, s.max, s.max],
            _stat: s,
            itemStyle: {
              color: groupColor(s.colorIndex),
              borderColor: groupColor(s.colorIndex),
            },
          }
        }),
      },
    ],
  }
})
</script>

<template>
  <div class="flex flex-col gap-2">
    <div v-if="noData" class="text-2xs text-ink-muted">暂无数据</div>
    <VChart v-else :option="option" :style="{ height: `${height ?? (groupStats.length * 15) + 50}px` }" :autoresize="true" />
  </div>
</template>
