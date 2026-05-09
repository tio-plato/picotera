<script setup lang="ts">
import { computed, reactive } from 'vue'
import { useQuery } from '@tanstack/vue-query'
import { CurveType, Scale } from '@unovis/ts'
import { VisArea, VisAxis, VisDonut, VisXYContainer } from '@unovis/vue'
import type {
  ApiKeyView,
  ModelView,
  OverviewCostView,
  OverviewDistributionRowView,
  OverviewSeriesRowView,
  ProviderView,
} from '@/api'
import { getOverview, listApiKeys, listModels, listProviders } from '@/api/client'
import {
  queryKeys,
  type OverviewDimension,
  type OverviewFilters,
  type OverviewRange,
  type OverviewSeriesDimension,
} from '@/api/queryKeys'
import { OPERATIONAL_STALE_TIME } from '@/api/queryClient'
import { useCurrency } from '@/composables/useCurrency'
import { DataCard, Field, Icon, IconButton, SegmentedControl, Select, StateText } from '@/ui'

type Metric = 'tokens' | 'cost' | 'requests' | 'traces'
type ChartPoint = { bucket: number; [group: string]: number }
type DonutDatum = { key: string; label: string; value: number; color: string }

const chartColors = [
  'var(--color-accent)',
  'var(--color-ok)',
  'var(--color-warn)',
  'var(--color-err)',
  'var(--color-accent-strong)',
  'var(--color-ok-ink)',
  'var(--color-warn-ink)',
  'var(--color-ink-muted)',
]

const currency = useCurrency()
const filters = reactive({
  range: '24h' as OverviewRange,
  apiKeyId: 0,
  model: '',
  upstreamModel: '',
  providerId: 0,
  distributionDimension: 'provider' as OverviewDimension,
  seriesDimension: 'none' as OverviewSeriesDimension,
})

const rangeOptions = [
  { value: '24h', label: '24h' },
  { value: '1d', label: '1d' },
  { value: '7d', label: '7d' },
  { value: '1m', label: '1m' },
]

const dimensionOptions: { value: OverviewDimension; label: string }[] = [
  { value: 'apiKey', label: 'API Key' },
  { value: 'model', label: '实际模型' },
  { value: 'upstreamModel', label: '上游模型' },
  { value: 'provider', label: '渠道' },
]

const seriesDimensionOptions: { value: OverviewSeriesDimension; label: string }[] = [
  { value: 'none', label: '不聚合' },
  ...dimensionOptions,
]

const apiKeysQuery = useQuery({
  queryKey: queryKeys.apiKeys.all,
  queryFn: listApiKeys,
})
const providersQuery = useQuery({
  queryKey: queryKeys.providers.all,
  queryFn: listProviders,
})
const modelsQuery = useQuery({
  queryKey: queryKeys.models.all,
  queryFn: listModels,
})

const overviewFilters = computed<OverviewFilters>(() => {
  const out: {
    range: OverviewRange
    apiKeyId?: number
    model?: string
    upstreamModel?: string
    providerId?: number
    distributionDimension?: OverviewDimension
    seriesDimension?: OverviewSeriesDimension
  } = {
    range: filters.range,
    distributionDimension: filters.distributionDimension,
    seriesDimension: filters.seriesDimension,
  }
  if (filters.apiKeyId) out.apiKeyId = filters.apiKeyId
  if (filters.model) out.model = filters.model
  if (filters.upstreamModel) out.upstreamModel = filters.upstreamModel
  if (filters.providerId) out.providerId = filters.providerId
  return out
})

const overviewQuery = useQuery({
  queryKey: computed(() => queryKeys.overview.detail(overviewFilters.value)),
  queryFn: () => getOverview(overviewFilters.value),
  staleTime: OPERATIONAL_STALE_TIME,
})

const apiKeys = computed<ApiKeyView[]>(() => apiKeysQuery.data.value ?? [])
const providers = computed<ProviderView[]>(() => providersQuery.data.value ?? [])
const models = computed<ModelView[]>(() => modelsQuery.data.value ?? [])
const overview = computed(() => overviewQuery.data.value)
const loading = computed(() => overviewQuery.isLoading.value || overviewQuery.isFetching.value)
const distributions = computed<OverviewDistributionRowView[]>(
  () => overview.value?.distributions ?? [],
)
const series = computed<OverviewSeriesRowView[]>(() => overview.value?.series ?? [])

const upstreamModels = computed(() => {
  const seen = new Set<string>()
  for (const provider of providers.value) {
    for (const entry of provider.providerModels ?? []) {
      const upstream = entry.upstreamModelName
      if (upstream) seen.add(upstream)
    }
  }
  return [...seen].sort((a, b) => a.localeCompare(b))
})

const totalCost = computed(() => formatCosts(overview.value?.summary.costs ?? []))
const nativeTotalCost = computed(() => nativeCostText(overview.value?.summary.costs ?? []))
const tokenDistribution = computed<DonutDatum[]>(() =>
  distributions.value
    .filter((row) => row.totalTokens > 0)
    .map((row, index) => ({
      key: row.key,
      label: displayLabel(row.label),
      value: row.totalTokens,
      color: chartColor(index),
    })),
)
const costDistribution = computed<DonutDatum[]>(() =>
  distributions.value
    .map((row, index) => ({
      key: row.key,
      label: displayLabel(row.label),
      value: convertedCostTotal(row.costs ?? []),
      color: chartColor(index),
    }))
    .filter((row) => row.value > 0),
)

const seriesCharts = computed(() => [
  buildSeriesChart('tokens', 'Token 数', formatCompactNumber),
  buildSeriesChart('cost', '费用', formatCompactMoney),
  buildSeriesChart('requests', '请求数', formatCompactNumber),
  buildSeriesChart('traces', '追踪数', formatCompactNumber),
])

function displayLabel(label: string): string {
  return label || '未设置'
}

function chartColor(index: number): string {
  return chartColors[index % chartColors.length] ?? 'var(--color-accent)'
}

function formatNumber(value: number): string {
  return Math.round(value).toLocaleString()
}

function formatCompactNumber(value: number): string {
  return new Intl.NumberFormat(undefined, {
    notation: 'compact',
    maximumFractionDigits: 1,
  }).format(value)
}

function normalizeCosts(costs: OverviewCostView[] | null | undefined): OverviewCostView[] {
  return costs ?? []
}

function nativeCostText(costs: OverviewCostView[] | null | undefined): string {
  const items = normalizeCosts(costs)
  if (items.length === 0) return '—'
  return items.map((cost) => currency.format(cost.amount, cost.currency)).join(' + ')
}

function convertedCostTotal(costs: OverviewCostView[] | null | undefined): number {
  const target = currency.targetCurrency.value
  let total = 0
  for (const cost of normalizeCosts(costs)) {
    const converted = currency.convert(cost.amount, cost.currency)
    if (target && converted.currency !== target) return 0
    total += converted.amount
  }
  return total
}

function formatCosts(costs: OverviewCostView[] | null | undefined): { text: string; title?: string } {
  const items = normalizeCosts(costs)
  if (items.length === 0) return { text: '—' }
  const target = currency.targetCurrency.value
  if (!target) return { text: nativeCostText(items) }

  let total = 0
  for (const cost of items) {
    const converted = currency.convert(cost.amount, cost.currency)
    if (converted.currency !== target) return { text: nativeCostText(items) }
    total += converted.amount
  }
  return { text: currency.format(total, target), title: nativeCostText(items) }
}

function formatCompactMoney(value: number): string {
  const target = currency.targetCurrency.value
  if (!target) return formatCompactNumber(value)
  return currency.format(value, target, { minDigits: 0, maxDigits: 1 })
}

function formatBucket(value: number): string {
  const date = new Date(value)
  const monthDay = `${date.getMonth() + 1}/${date.getDate()}`
  const hour = String(date.getHours()).padStart(2, '0')
  return `${monthDay} ${hour}:00`
}

function groupKey(row: OverviewSeriesRowView): string {
  if (row.metric === 'cost' && row.currency) return `${row.groupKey}:${row.currency}`
  return row.groupKey || 'total'
}

function groupLabel(row: OverviewSeriesRowView): string {
  return displayLabel(row.groupLabel)
}

function buildSeriesChart(metric: Metric, title: string, formatter: (value: number) => string) {
  const rows = series.value.filter((row) => row.metric === metric)
  const keys: string[] = []
  const labels = new Map<string, string>()
  const byBucket = new Map<number, ChartPoint>()

  for (const row of rows) {
    const bucket = new Date(row.bucketAt).getTime()
    if (Number.isNaN(bucket)) continue
    const key = groupKey(row)
    if (!keys.includes(key)) keys.push(key)
    labels.set(key, groupLabel(row))
    const point = byBucket.get(bucket) ?? { bucket }
    point[key] = row.value
    byBucket.set(bucket, point)
  }

  const data = [...byBucket.values()].sort((a, b) => a.bucket - b.bucket)
  for (const point of data) {
    for (const key of keys) {
      point[key] ??= 0
    }
  }

  return {
    metric,
    title,
    data,
    keys,
    labels,
    formatter,
    empty: data.every((point) => keys.every((key) => !point[key])),
  }
}

function chartYAccessors(keys: string[]) {
  return keys.map((key) => (d: ChartPoint) => d[key] ?? 0)
}

function colorForKey() {
  return (_: ChartPoint[], index: number) => chartColor(index)
}

function donutValue(datum: DonutDatum): number {
  return datum.value
}

function donutColor(datum: DonutDatum): string {
  return datum.color
}
</script>

<template>
  <div class="flex flex-col gap-4">
    <div class="flex flex-wrap items-end justify-between gap-3">
      <div class="w-full max-w-[22rem]">
        <SegmentedControl v-model="filters.range" :options="rangeOptions" :columns="4" />
      </div>
      <IconButton title="刷新" aria-label="刷新" :disabled="loading" @click="overviewQuery.refetch()">
        <Icon name="refresh" :size="13" />
      </IconButton>
    </div>

    <DataCard>
      <div class="grid gap-3 p-4 md:grid-cols-2 xl:grid-cols-4">
        <Field label="API Key" as="div">
          <Select v-model.number="filters.apiKeyId" class="w-full">
            <option :value="0">全部 API Key</option>
            <option v-for="key in apiKeys" :key="key.id" :value="key.id">{{ key.name }}</option>
          </Select>
        </Field>
        <Field label="实际模型" as="div">
          <Select v-model="filters.model" class="w-full">
            <option value="">全部实际模型</option>
            <option v-for="model in models" :key="model.name" :value="model.name">
              {{ model.name }}
            </option>
          </Select>
        </Field>
        <Field label="上游模型" as="div">
          <Select v-model="filters.upstreamModel" class="w-full">
            <option value="">全部上游模型</option>
            <option v-for="model in upstreamModels" :key="model" :value="model">{{ model }}</option>
          </Select>
        </Field>
        <Field label="渠道" as="div">
          <Select v-model.number="filters.providerId" class="w-full">
            <option :value="0">全部渠道</option>
            <option v-for="provider in providers" :key="provider.id" :value="provider.id">
              {{ provider.name }}
            </option>
          </Select>
        </Field>
      </div>
    </DataCard>

    <StateText v-if="overviewQuery.isError.value" dashed>
      {{ overviewQuery.error.value?.message ?? '加载概览失败' }}
    </StateText>

    <div class="grid gap-3 md:grid-cols-2 xl:grid-cols-4">
      <DataCard>
        <div class="p-4">
          <div class="text-2xs font-medium uppercase tracking-[0.03em] text-ink-muted">Token 总数</div>
          <div class="mt-2 font-mono text-[1.65rem] leading-none tabular-nums text-ink">
            {{ formatNumber(overview?.summary.totalTokens ?? 0) }}
          </div>
        </div>
      </DataCard>
      <DataCard>
        <div class="p-4">
          <div class="text-2xs font-medium uppercase tracking-[0.03em] text-ink-muted">请求总数</div>
          <div class="mt-2 font-mono text-[1.65rem] leading-none tabular-nums text-ink">
            {{ formatNumber(overview?.summary.totalRequests ?? 0) }}
          </div>
        </div>
      </DataCard>
      <DataCard>
        <div class="p-4">
          <div class="text-2xs font-medium uppercase tracking-[0.03em] text-ink-muted">总费用</div>
          <div
            class="mt-2 truncate font-mono text-[1.65rem] leading-none tabular-nums text-ink"
            :title="totalCost.title ?? nativeTotalCost"
          >
            {{ totalCost.text }}
          </div>
        </div>
      </DataCard>
      <DataCard>
        <div class="p-4">
          <div class="text-2xs font-medium uppercase tracking-[0.03em] text-ink-muted">追踪总数</div>
          <div class="mt-2 font-mono text-[1.65rem] leading-none tabular-nums text-ink">
            {{ formatNumber(overview?.summary.totalTraceCount ?? 0) }}
          </div>
        </div>
      </DataCard>
    </div>

    <DataCard>
      <div class="flex flex-wrap items-center justify-between gap-3 border-b border-line px-4 py-3">
        <div>
          <h2 class="m-0 text-base font-semibold text-ink">分布</h2>
          <p class="m-0 mt-0.5 text-sm text-ink-faint">Token 与费用按同一维度切分</p>
        </div>
        <Field label="维度" as="div">
          <Select v-model="filters.distributionDimension" size="sm">
            <option v-for="option in dimensionOptions" :key="option.value" :value="option.value">
              {{ option.label }}
            </option>
          </Select>
        </Field>
      </div>
      <div class="grid gap-0 lg:grid-cols-2">
        <section class="border-b border-line p-4 lg:border-r lg:border-b-0">
          <div class="mb-3 text-sm font-medium text-ink">Token 分布</div>
          <StateText v-if="tokenDistribution.length === 0" compact>暂无 Token 数据</StateText>
          <div v-else class="grid min-h-[17rem] gap-3 md:grid-cols-[minmax(0,1fr)_11rem]">
            <VisDonut
              :data="tokenDistribution"
              :value="donutValue"
              :color="donutColor"
              :arc-width="18"
              :corner-radius="2"
              :central-label="formatCompactNumber(overview?.summary.totalTokens ?? 0)"
              central-sub-label="Tokens"
              class="min-h-[15rem]"
            />
            <div class="flex flex-col justify-center gap-2">
              <div v-for="row in tokenDistribution.slice(0, 8)" :key="row.key" class="min-w-0">
                <div class="flex items-center gap-2">
                  <span class="h-2 w-2 rounded-sm" :style="{ backgroundColor: row.color }" />
                  <span class="truncate text-sm text-ink" :title="row.label">{{ row.label }}</span>
                </div>
                <div class="ml-4 font-mono text-xs tabular-nums text-ink-faint">
                  {{ formatNumber(row.value) }}
                </div>
              </div>
            </div>
          </div>
        </section>
        <section class="p-4">
          <div class="mb-3 text-sm font-medium text-ink">费用分布</div>
          <StateText v-if="costDistribution.length === 0" compact>暂无费用数据</StateText>
          <div v-else class="grid min-h-[17rem] gap-3 md:grid-cols-[minmax(0,1fr)_11rem]">
            <VisDonut
              :data="costDistribution"
              :value="donutValue"
              :color="donutColor"
              :arc-width="18"
              :corner-radius="2"
              :central-label="totalCost.text"
              central-sub-label="Cost"
              class="min-h-[15rem]"
            />
            <div class="flex flex-col justify-center gap-2">
              <div v-for="row in costDistribution.slice(0, 8)" :key="row.key" class="min-w-0">
                <div class="flex items-center gap-2">
                  <span class="h-2 w-2 rounded-sm" :style="{ backgroundColor: row.color }" />
                  <span class="truncate text-sm text-ink" :title="row.label">{{ row.label }}</span>
                </div>
                <div class="ml-4 font-mono text-xs tabular-nums text-ink-faint">
                  {{ formatCompactMoney(row.value) }}
                </div>
              </div>
            </div>
          </div>
        </section>
      </div>
    </DataCard>

    <DataCard>
      <div class="flex flex-wrap items-center justify-between gap-3 border-b border-line px-4 py-3">
        <div>
          <h2 class="m-0 text-base font-semibold text-ink">小时趋势</h2>
          <p class="m-0 mt-0.5 text-sm text-ink-faint">所有指标按小时聚合</p>
        </div>
        <Field label="聚合" as="div">
          <Select v-model="filters.seriesDimension" size="sm">
            <option v-for="option in seriesDimensionOptions" :key="option.value" :value="option.value">
              {{ option.label }}
            </option>
          </Select>
        </Field>
      </div>

      <div class="grid gap-0 lg:grid-cols-2">
        <section
          v-for="(chart, index) in seriesCharts"
          :key="chart.metric"
          class="border-line p-4"
          :class="{
            'border-b': index < 2,
            'lg:border-r': index % 2 === 0,
          }"
        >
          <div class="mb-3 flex items-center justify-between gap-3">
            <div class="text-sm font-medium text-ink">{{ chart.title }}</div>
            <div class="flex max-w-[16rem] flex-wrap justify-end gap-x-3 gap-y-1">
              <span
                v-for="(key, keyIndex) in chart.keys.slice(0, 5)"
                :key="key"
                class="inline-flex min-w-0 items-center gap-1.5 text-2xs text-ink-faint"
              >
                <span
                  class="h-1.5 w-1.5 rounded-sm"
                  :style="{ backgroundColor: chartColor(keyIndex) }"
                />
                <span class="truncate">{{ chart.labels.get(key) }}</span>
              </span>
            </div>
          </div>
          <StateText v-if="chart.empty" compact>暂无趋势数据</StateText>
          <VisXYContainer
            v-else
            :data="chart.data"
            :height="230"
            :x-scale="Scale.scaleTime()"
            :y-domain-min-constraint="[0, undefined]"
            :prevent-empty-domain="true"
          >
            <VisArea
              :x="(d: ChartPoint) => d.bucket"
              :y="chartYAccessors(chart.keys)"
              :color="colorForKey()"
              :curve-type="CurveType.MonotoneX"
              :opacity="0.82"
              line
              :line-width="1"
            />
            <VisAxis
              type="x"
              :grid-line="false"
              :tick-format="(tick: number | Date) => formatBucket(Number(tick))"
              tick-text-font-size="10px"
              tick-text-color="var(--color-ink-faint)"
            />
            <VisAxis
              type="y"
              :tick-format="(tick: number | Date) => chart.formatter(Number(tick))"
              tick-text-font-size="10px"
              tick-text-color="var(--color-ink-faint)"
              :domain-line="false"
            />
          </VisXYContainer>
        </section>
      </div>
    </DataCard>
  </div>
</template>
