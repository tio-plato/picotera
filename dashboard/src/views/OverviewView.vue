<script setup lang="ts">
import { computed, reactive, ref } from 'vue'
import { useQuery } from '@tanstack/vue-query'
import {
  getOverviewDistribution,
  getOverviewSeries,
  getOverviewSummary,
  listApiKeys,
  listModels,
  listProviders,
} from '@/api/client'
import { OPERATIONAL_STALE_TIME } from '@/api/queryClient'
import { queryKeys, type OverviewFilters } from '@/api/queryKeys'
import type {
  OverviewDimension,
  OverviewRange,
  OverviewSeriesDimension,
  OverviewSeriesPointView,
} from '@/api'
import { DataCard, MoneyDisplay, SegmentedControl, Select, StateText } from '@/ui'
import { useCurrency } from '@/composables/useCurrency'
import OverviewDonut from '@/components/charts/OverviewDonut.vue'
import OverviewAreaStack from '@/components/charts/OverviewAreaStack.vue'
import OverviewSankey, { type SankeyLink, type SankeyNode } from '@/components/charts/OverviewSankey.vue'

const filters = reactive({
  range: '1d' as OverviewRange,
  apiKeyId: 0,
  model: '',
  upstreamModel: '',
  providerId: 0,
})
const distributionDimension = ref<OverviewDimension>('provider')
const seriesDimension = ref<OverviewSeriesDimension>('none')

type SankeyVariant = 'tokenComposition' | 'tokensIn' | 'tokensOut' | 'costIn' | 'costOut'

const sankeyVariant = ref<SankeyVariant>('tokenComposition')
const sankeyVariantOptions: { value: SankeyVariant; label: string }[] = [
  { value: 'tokenComposition', label: 'Token 构成' },
  { value: 'tokensIn',         label: 'Token 上游' },
  { value: 'tokensOut',        label: 'Token 下游' },
  { value: 'costIn',           label: '费用上游' },
  { value: 'costOut',          label: '费用下游' },
]

const ccy = useCurrency()
const isOriginalMode = computed(() => ccy.targetCurrency.value == null)

const rangeOptions: { value: OverviewRange; label: string }[] = [
  { value: '1d', label: '24 小时' },
  { value: '7d', label: '7 天' },
  { value: '1m', label: '30 天' },
]
const distributionDimensionOptions: { value: OverviewDimension; label: string }[] = [
  { value: 'provider', label: '渠道' },
  { value: 'apiKey', label: 'API Key' },
  { value: 'model', label: '请求模型' },
  { value: 'upstreamModel', label: '上游模型' },
]
const seriesDimensionOptions: { value: OverviewSeriesDimension; label: string }[] = [
  { value: 'none', label: '全部' },
  { value: 'provider', label: '渠道' },
  { value: 'apiKey', label: 'API Key' },
  { value: 'model', label: '请求模型' },
  { value: 'upstreamModel', label: '上游模型' },
]

const overviewFilters = computed<OverviewFilters>(() => {
  const out: { range: OverviewRange; apiKeyId?: number; model?: string; upstreamModel?: string; providerId?: number } = {
    range: filters.range,
  }
  if (filters.apiKeyId) out.apiKeyId = filters.apiKeyId
  if (filters.model) out.model = filters.model
  if (filters.upstreamModel) out.upstreamModel = filters.upstreamModel
  if (filters.providerId) out.providerId = filters.providerId
  return out
})

const apiKeysQuery = useQuery({ queryKey: queryKeys.apiKeys.all, queryFn: listApiKeys })
const providersQuery = useQuery({ queryKey: queryKeys.providers.all, queryFn: listProviders })
const modelsQuery = useQuery({ queryKey: queryKeys.models.all, queryFn: listModels })

const apiKeys = computed(() => apiKeysQuery.data.value ?? [])
const providers = computed(() => providersQuery.data.value ?? [])
const models = computed(() => modelsQuery.data.value ?? [])

const apiKeyLabelById = computed(() => {
  const m = new Map<number, string>()
  for (const k of apiKeys.value) m.set(k.id, k.name)
  return m
})
const providerLabelById = computed(() => {
  const m = new Map<number, string>()
  for (const p of providers.value) m.set(p.id, p.name)
  return m
})

const modelOptions = computed(() => {
  const set = new Set<string>()
  for (const m of models.value) if (m.name) set.add(m.name)
  return Array.from(set).sort()
})

const upstreamModelOptions = computed(() => {
  const set = new Set<string>()
  for (const p of providers.value) {
    for (const pm of p.providerModels ?? []) {
      if (pm.upstreamModelName) set.add(pm.upstreamModelName)
      else if (pm.model) set.add(pm.model)
    }
  }
  return Array.from(set).sort()
})

const summaryQuery = useQuery({
  queryKey: computed(() => queryKeys.overview.summary(overviewFilters.value)),
  queryFn: () => getOverviewSummary(overviewFilters.value),
  staleTime: OPERATIONAL_STALE_TIME,
})

const distributionQuery = useQuery({
  queryKey: computed(() => queryKeys.overview.distribution(overviewFilters.value, distributionDimension.value)),
  queryFn: () => getOverviewDistribution(overviewFilters.value, distributionDimension.value),
  staleTime: OPERATIONAL_STALE_TIME,
})

const seriesQuery = useQuery({
  queryKey: computed(() => queryKeys.overview.series(overviewFilters.value, seriesDimension.value)),
  queryFn: () => getOverviewSeries(overviewFilters.value, seriesDimension.value),
  staleTime: OPERATIONAL_STALE_TIME,
})

function dimensionLabel(dim: OverviewDimension | OverviewSeriesDimension, key: string): string {
  if (key === '') return '全部'
  if (dim === 'provider') {
    const id = Number(key)
    return Number.isFinite(id) ? providerLabelById.value.get(id) ?? `#${key}` : key
  }
  if (dim === 'apiKey') {
    const id = Number(key)
    return Number.isFinite(id) ? apiKeyLabelById.value.get(id) ?? `#${key}` : key
  }
  return key
}

const TOP_N = 8

const distributionRows = computed(() => distributionQuery.data.value?.rows ?? [])

interface DonutItem { key: string; label: string; value: number }

function buildItemsAndTopN(items: DonutItem[]): DonutItem[] {
  const sorted = items.filter((d) => d.value > 0).sort((a, b) => b.value - a.value)
  if (sorted.length <= TOP_N) return sorted
  const top = sorted.slice(0, TOP_N)
  const restValue = sorted.slice(TOP_N).reduce((acc, d) => acc + d.value, 0)
  if (restValue > 0) top.push({ key: '__other__', label: '其他', value: restValue })
  return top
}

const tokenDonutData = computed<DonutItem[]>(() =>
  buildItemsAndTopN(
    distributionRows.value.map((r) => ({
      key: r.key,
      label: dimensionLabel(distributionDimension.value, r.key),
      value: r.totalTokens ?? 0,
    })),
  ),
)

function buildCostDonutDataForCurrency(currency: string): DonutItem[] {
  return buildItemsAndTopN(
    distributionRows.value.map((r) => {
      const cost = (r.costs ?? []).find((c) => c.currency === currency)
      return {
        key: r.key,
        label: dimensionLabel(distributionDimension.value, r.key),
        value: cost?.amount ?? 0,
      }
    }),
  )
}

const costDonutDataConverted = computed<DonutItem[]>(() => {
  const target = ccy.targetCurrency.value
  if (!target) return []
  return buildItemsAndTopN(
    distributionRows.value.map((r) => {
      const sum = (r.costs ?? []).reduce(
        (acc, c) => acc + ccy.convert(c.amount, c.currency).amount,
        0,
      )
      return {
        key: r.key,
        label: dimensionLabel(distributionDimension.value, r.key),
        value: sum,
      }
    }),
  )
})

const distributionCurrenciesPresent = computed(() => {
  const set = new Set<string>()
  for (const row of distributionRows.value) {
    for (const c of row.costs ?? []) if (c.currency) set.add(c.currency)
  }
  return Array.from(set).sort()
})

const seriesData = computed(() => seriesQuery.data.value)

const seriesCurrenciesPresent = computed(() => {
  const set = new Set<string>()
  for (const p of seriesData.value?.points ?? []) {
    if (p.metric === 'cost' && p.currency) set.add(p.currency)
  }
  return Array.from(set).sort()
})

const seriesGroups = computed(() => {
  const groups = seriesData.value?.groups ?? []
  return groups.map((g) => ({ key: g.key, label: dimensionLabel(seriesDimension.value, g.key) }))
})
const seriesBuckets = computed(() => seriesData.value?.buckets ?? [])

interface SeriesPointVM { groupKey: string; bucketAt: string; value: number }

function nonCostPoints(metric: 'tokens' | 'requests' | 'traces'): SeriesPointVM[] {
  const points: OverviewSeriesPointView[] = seriesData.value?.points ?? []
  return points
    .filter((p) => p.metric === metric)
    .map((p) => ({ groupKey: p.groupKey, bucketAt: p.bucketAt, value: p.value }))
}

function costPointsForCurrency(currency: string): SeriesPointVM[] {
  return (seriesData.value?.points ?? [])
    .filter((p) => p.metric === 'cost' && p.currency === currency)
    .map((p) => ({ groupKey: p.groupKey, bucketAt: p.bucketAt, value: p.value }))
}

const costPointsConverted = computed<SeriesPointVM[]>(() => {
  const target = ccy.targetCurrency.value
  if (!target) return []
  const byGroup = new Map<string, Map<string, number>>()
  for (const p of seriesData.value?.points ?? []) {
    if (p.metric !== 'cost' || !p.currency) continue
    let m = byGroup.get(p.groupKey)
    if (!m) { m = new Map(); byGroup.set(p.groupKey, m) }
    const v = ccy.convert(p.value, p.currency).amount
    m.set(p.bucketAt, (m.get(p.bucketAt) ?? 0) + v)
  }
  const out: SeriesPointVM[] = []
  for (const [groupKey, m] of byGroup) {
    for (const [bucketAt, value] of m) out.push({ groupKey, bucketAt, value })
  }
  return out
})

const seriesTokens = computed(() => nonCostPoints('tokens'))
const seriesRequests = computed(() => nonCostPoints('requests'))
const seriesTraces = computed(() => nonCostPoints('traces'))

const summaryConvertedTotal = computed(() => {
  const target = ccy.targetCurrency.value
  if (!target) return 0
  return (summaryQuery.data.value?.costs ?? []).reduce(
    (acc, c) => acc + ccy.convert(c.amount, c.currency).amount,
    0,
  )
})

const tokenCompositionSankey = computed<{ nodes: SankeyNode[]; links: SankeyLink[] }>(() => {
  const tb = summaryQuery.data.value?.tokenBreakdown
  if (!tb) return { nodes: [], links: [] }
  const inputTotal = tb.input + tb.cacheRead + tb.cacheWrite + tb.cacheWrite1h
  const outputTotal = tb.output
  if (inputTotal === 0 && outputTotal === 0) return { nodes: [], links: [] }

  const allNodes: SankeyNode[] = [
    { id: 'root',                label: '总 Token',     layer: 0 },
    { id: 'output',              label: '输出',          layer: 1 },
    { id: 'input',               label: '输入',          layer: 1 },
    { id: 'in_uncached',         label: '未缓存输入',     layer: 2 },
    { id: 'in_cache_read',       label: '缓存读取',       layer: 2 },
    { id: 'in_cache_write',      label: '缓存写入',       layer: 2 },
    { id: 'in_cache_write_1h',   label: '长期缓存写入',   layer: 2 },
  ]
  const links: SankeyLink[] = []
  if (outputTotal > 0)     links.push({ source: 'root',  target: 'output',              value: outputTotal })
  if (inputTotal > 0)      links.push({ source: 'root',  target: 'input',               value: inputTotal })
  if (tb.input > 0)        links.push({ source: 'input', target: 'in_uncached',         value: tb.input })
  if (tb.cacheRead > 0)    links.push({ source: 'input', target: 'in_cache_read',       value: tb.cacheRead })
  if (tb.cacheWrite > 0)   links.push({ source: 'input', target: 'in_cache_write',      value: tb.cacheWrite })
  if (tb.cacheWrite1h > 0) links.push({ source: 'input', target: 'in_cache_write_1h',   value: tb.cacheWrite1h })

  const used = new Set<string>(['root'])
  for (const l of links) { used.add(l.source); used.add(l.target) }
  return { nodes: allNodes.filter((n) => used.has(n.id)), links }
})

function compactNumber(v: number) {
  if (!Number.isFinite(v)) return ''
  if (Math.abs(v) >= 1e9) return `${(v / 1e9).toFixed(1)}B`
  if (Math.abs(v) >= 1e6) return `${(v / 1e6).toFixed(1)}M`
  if (Math.abs(v) >= 1e3) return `${(v / 1e3).toFixed(1)}k`
  return v.toFixed(0)
}

function formatBucket(iso: string) {
  const d = new Date(iso)
  if (Number.isNaN(d.getTime())) return iso
  const buckets = seriesBuckets.value
  if (buckets.length <= 24) {
    return `${d.getHours().toString().padStart(2, '0')}:00`
  }
  if (buckets.length <= 24 * 7) {
    return `${d.getMonth() + 1}/${d.getDate()} ${d.getHours().toString().padStart(2, '0')}:00`
  }
  return `${d.getMonth() + 1}/${d.getDate()}`
}

function formatCurrencyCompact(v: number, code: string) {
  if (!Number.isFinite(v)) return ''
  const abs = Math.abs(v)
  let scaled = v
  let suffix = ''
  if (abs >= 1e9) { scaled = v / 1e9; suffix = 'B' }
  else if (abs >= 1e6) { scaled = v / 1e6; suffix = 'M' }
  else if (abs >= 1e3) { scaled = v / 1e3; suffix = 'k' }
  const digits = suffix ? 1 : 2
  return ccy.format(scaled, code, { minDigits: digits, maxDigits: digits }) + suffix
}
</script>

<template>
  <div class="flex flex-col gap-3">
    <!-- Controls bar -->
    <div class="flex flex-wrap items-end gap-3">
      <div class="flex flex-col gap-1">
        <span class="text-2xs font-medium text-ink-muted uppercase tracking-[0.03em]">时间范围</span>
        <SegmentedControl v-model="filters.range" :options="rangeOptions" />
      </div>
      <div class="flex flex-col gap-1">
        <span class="text-2xs font-medium text-ink-muted uppercase tracking-[0.03em]">API Key</span>
        <Select v-model.number="filters.apiKeyId" size="sm">
          <option :value="0">全部</option>
          <option v-for="k in apiKeys" :key="k.id" :value="k.id">{{ k.name }}</option>
        </Select>
      </div>
      <div class="flex flex-col gap-1">
        <span class="text-2xs font-medium text-ink-muted uppercase tracking-[0.03em]">请求模型</span>
        <Select v-model="filters.model" size="sm">
          <option value="">全部</option>
          <option v-for="m in modelOptions" :key="m" :value="m">{{ m }}</option>
        </Select>
      </div>
      <div class="flex flex-col gap-1">
        <span class="text-2xs font-medium text-ink-muted uppercase tracking-[0.03em]">上游模型</span>
        <Select v-model="filters.upstreamModel" size="sm">
          <option value="">全部</option>
          <option v-for="u in upstreamModelOptions" :key="u" :value="u">{{ u }}</option>
        </Select>
      </div>
      <div class="flex flex-col gap-1">
        <span class="text-2xs font-medium text-ink-muted uppercase tracking-[0.03em]">渠道</span>
        <Select v-model.number="filters.providerId" size="sm">
          <option :value="0">全部</option>
          <option v-for="p in providers" :key="p.id" :value="p.id">{{ p.name }}</option>
        </Select>
      </div>
    </div>

    <!-- Bento totals -->
    <div class="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-3">
      <DataCard>
        <div class="p-4 flex flex-col gap-1.5">
          <span class="text-2xs font-medium text-ink-muted uppercase tracking-[0.03em]">总 Token</span>
          <StateText v-if="summaryQuery.isLoading.value" compact :dashed="false">加载中…</StateText>
          <StateText v-else-if="summaryQuery.isError.value" compact :dashed="false">{{ (summaryQuery.error.value as Error)?.message ?? '加载失败' }}</StateText>
          <span v-else class="text-xl font-semibold mono tabular text-ink">{{ (summaryQuery.data.value?.totalTokens ?? 0).toLocaleString() }}</span>
        </div>
      </DataCard>
      <DataCard>
        <div class="p-4 flex flex-col gap-1.5">
          <span class="text-2xs font-medium text-ink-muted uppercase tracking-[0.03em]">总请求</span>
          <StateText v-if="summaryQuery.isLoading.value" compact :dashed="false">加载中…</StateText>
          <StateText v-else-if="summaryQuery.isError.value" compact :dashed="false">{{ (summaryQuery.error.value as Error)?.message ?? '加载失败' }}</StateText>
          <span v-else class="text-xl font-semibold mono tabular text-ink">{{ (summaryQuery.data.value?.totalRequests ?? 0).toLocaleString() }}</span>
        </div>
      </DataCard>
      <DataCard>
        <div class="p-4 flex flex-col gap-1.5">
          <span class="text-2xs font-medium text-ink-muted uppercase tracking-[0.03em]">总费用</span>
          <StateText v-if="summaryQuery.isLoading.value" compact :dashed="false">加载中…</StateText>
          <StateText v-else-if="summaryQuery.isError.value" compact :dashed="false">{{ (summaryQuery.error.value as Error)?.message ?? '加载失败' }}</StateText>
          <div v-else-if="(summaryQuery.data.value?.costs ?? []).length === 0" class="text-xl text-ink-faint">—</div>
          <MoneyDisplay
            v-else-if="!isOriginalMode"
            class="text-xl font-semibold mono tabular text-ink"
            :amount="summaryConvertedTotal"
            :currency="ccy.targetCurrency.value"
          />
          <div v-else class="flex flex-row flex-wrap items-baseline gap-x-1 gap-y-0.5">
            <template v-for="(c, i) in summaryQuery.data.value?.costs ?? []" :key="c.currency">
              <span v-if="i > 0" class="text-ink-faint text-base">+</span>
              <MoneyDisplay
                class="text-xl font-semibold mono tabular text-ink"
                :amount="c.amount"
                :currency="c.currency"
              />
            </template>
          </div>
        </div>
      </DataCard>
      <DataCard>
        <div class="p-4 flex flex-col gap-1.5">
          <span class="text-2xs font-medium text-ink-muted uppercase tracking-[0.03em]">总追踪</span>
          <StateText v-if="summaryQuery.isLoading.value" compact :dashed="false">加载中…</StateText>
          <StateText v-else-if="summaryQuery.isError.value" compact :dashed="false">{{ (summaryQuery.error.value as Error)?.message ?? '加载失败' }}</StateText>
          <span v-else class="text-xl font-semibold mono tabular text-ink">{{ (summaryQuery.data.value?.totalTraceCount ?? 0).toLocaleString() }}</span>
        </div>
      </DataCard>
    </div>

    <!-- Sankey -->
    <div class="flex flex-wrap items-end gap-3">
      <div class="flex flex-col gap-1">
        <span class="text-2xs font-medium text-ink-muted uppercase tracking-[0.03em]">流向</span>
        <SegmentedControl v-model="sankeyVariant" :options="sankeyVariantOptions" />
      </div>
    </div>
    <div class="grid grid-cols-1 gap-3">
      <DataCard>
        <div class="p-4 flex flex-col gap-3">
          <StateText v-if="summaryQuery.isLoading.value" compact :dashed="false">加载中…</StateText>
          <StateText
            v-else-if="summaryQuery.isError.value"
            compact
            :dashed="false"
          >{{ (summaryQuery.error.value as Error)?.message ?? '加载失败' }}</StateText>

          <template v-else-if="sankeyVariant === 'tokenComposition'">
            <StateText
              v-if="!tokenCompositionSankey.links.length"
              compact
            >暂无数据</StateText>
            <OverviewSankey
              v-else
              :nodes="tokenCompositionSankey.nodes"
              :links="tokenCompositionSankey.links"
              :value-format="(v) => compactNumber(v)"
            />
          </template>

          <StateText v-else compact>暂无数据</StateText>
        </div>
      </DataCard>
    </div>

    <!-- Distribution -->
    <div class="flex flex-wrap items-end gap-3">
      <div class="flex flex-col gap-1">
        <span class="text-2xs font-medium text-ink-muted uppercase tracking-[0.03em]">分布统计</span>
        <SegmentedControl v-model="distributionDimension" :options="distributionDimensionOptions" />
      </div>
    </div>
    <div class="grid grid-cols-1 lg:grid-cols-2 gap-3">
      <DataCard>
        <div class="p-4 flex flex-col gap-3">
          <span class="text-2xs font-medium text-ink-muted uppercase tracking-[0.03em]">Token 分布</span>
          <StateText v-if="distributionQuery.isLoading.value" compact :dashed="false">加载中…</StateText>
          <StateText v-else-if="distributionQuery.isError.value" compact :dashed="false">{{ (distributionQuery.error.value as Error)?.message ?? '加载失败' }}</StateText>
          <StateText v-else-if="!tokenDonutData.length" compact>暂无数据</StateText>
          <OverviewDonut
            v-else
            :data="tokenDonutData"
            :value-format="(v) => compactNumber(v)"
          />
        </div>
      </DataCard>
      <template v-if="!isOriginalMode">
        <DataCard>
          <div class="p-4 flex flex-col gap-3">
            <span class="text-2xs font-medium text-ink-muted uppercase tracking-[0.03em]">费用分布<span class="ml-1 text-ink-faint normal-case">· {{ ccy.targetCurrency.value }}</span></span>
            <StateText v-if="distributionQuery.isLoading.value" compact :dashed="false">加载中…</StateText>
            <StateText v-else-if="distributionQuery.isError.value" compact :dashed="false">{{ (distributionQuery.error.value as Error)?.message ?? '加载失败' }}</StateText>
            <StateText v-else-if="!costDonutDataConverted.length" compact>暂无数据</StateText>
            <OverviewDonut
              v-else
              :data="costDonutDataConverted"
              :value-format="(v) => formatCurrencyCompact(v, ccy.targetCurrency.value ?? '')"
            />
          </div>
        </DataCard>
      </template>
      <template v-else-if="distributionCurrenciesPresent.length === 0">
        <DataCard>
          <div class="p-4 flex flex-col gap-3">
            <span class="text-2xs font-medium text-ink-muted uppercase tracking-[0.03em]">费用分布</span>
            <StateText v-if="distributionQuery.isLoading.value" compact :dashed="false">加载中…</StateText>
            <StateText v-else-if="distributionQuery.isError.value" compact :dashed="false">{{ (distributionQuery.error.value as Error)?.message ?? '加载失败' }}</StateText>
            <StateText v-else compact>暂无数据</StateText>
          </div>
        </DataCard>
      </template>
      <template v-else>
        <DataCard v-for="currency in distributionCurrenciesPresent" :key="currency">
          <div class="p-4 flex flex-col gap-3">
            <span class="text-2xs font-medium text-ink-muted uppercase tracking-[0.03em]">费用分布<span class="ml-1 text-ink-faint normal-case">· {{ currency }}</span></span>
            <StateText v-if="distributionQuery.isLoading.value" compact :dashed="false">加载中…</StateText>
            <StateText v-else-if="distributionQuery.isError.value" compact :dashed="false">{{ (distributionQuery.error.value as Error)?.message ?? '加载失败' }}</StateText>
            <StateText v-else-if="!buildCostDonutDataForCurrency(currency).length" compact>暂无数据</StateText>
            <OverviewDonut
              v-else
              :data="buildCostDonutDataForCurrency(currency)"
              :value-format="(v) => formatCurrencyCompact(v, currency)"
            />
          </div>
        </DataCard>
      </template>
    </div>

    <!-- Series -->
    <div class="flex flex-wrap items-end gap-3">
      <div class="flex flex-col gap-1">
        <span class="text-2xs font-medium text-ink-muted uppercase tracking-[0.03em]">用量统计</span>
        <SegmentedControl v-model="seriesDimension" :options="seriesDimensionOptions" />
      </div>
    </div>
    <div class="grid grid-cols-1 lg:grid-cols-2 gap-3">
      <DataCard>
        <div class="p-4 flex flex-col gap-3">
          <span class="text-2xs font-medium text-ink-muted uppercase tracking-[0.03em]">Token</span>
          <StateText v-if="seriesQuery.isLoading.value" compact :dashed="false">加载中…</StateText>
          <StateText v-else-if="seriesQuery.isError.value" compact :dashed="false">{{ (seriesQuery.error.value as Error)?.message ?? '加载失败' }}</StateText>
          <OverviewAreaStack
            v-else
            :groups="seriesGroups"
            :buckets="seriesBuckets"
            :points="seriesTokens"
            :value-format="(v) => compactNumber(v)"
            :bucket-format="formatBucket"
          />
        </div>
      </DataCard>
      <template v-if="!isOriginalMode">
        <DataCard>
          <div class="p-4 flex flex-col gap-3">
            <span class="text-2xs font-medium text-ink-muted uppercase tracking-[0.03em]">费用<span class="ml-1 text-ink-faint normal-case">· {{ ccy.targetCurrency.value }}</span></span>
            <StateText v-if="seriesQuery.isLoading.value" compact :dashed="false">加载中…</StateText>
            <StateText v-else-if="seriesQuery.isError.value" compact :dashed="false">{{ (seriesQuery.error.value as Error)?.message ?? '加载失败' }}</StateText>
            <OverviewAreaStack
              v-else
              :groups="seriesGroups"
              :buckets="seriesBuckets"
              :points="costPointsConverted"
              :value-format="(v) => formatCurrencyCompact(v, ccy.targetCurrency.value ?? '')"
              :bucket-format="formatBucket"
            />
          </div>
        </DataCard>
      </template>
      <template v-else-if="seriesCurrenciesPresent.length === 0">
        <DataCard>
          <div class="p-4 flex flex-col gap-3">
            <span class="text-2xs font-medium text-ink-muted uppercase tracking-[0.03em]">费用</span>
            <StateText v-if="seriesQuery.isLoading.value" compact :dashed="false">加载中…</StateText>
            <StateText v-else-if="seriesQuery.isError.value" compact :dashed="false">{{ (seriesQuery.error.value as Error)?.message ?? '加载失败' }}</StateText>
            <StateText v-else compact>暂无数据</StateText>
          </div>
        </DataCard>
      </template>
      <template v-else>
        <DataCard v-for="currency in seriesCurrenciesPresent" :key="currency">
          <div class="p-4 flex flex-col gap-3">
            <span class="text-2xs font-medium text-ink-muted uppercase tracking-[0.03em]">费用<span class="ml-1 text-ink-faint normal-case">· {{ currency }}</span></span>
            <StateText v-if="seriesQuery.isLoading.value" compact :dashed="false">加载中…</StateText>
            <StateText v-else-if="seriesQuery.isError.value" compact :dashed="false">{{ (seriesQuery.error.value as Error)?.message ?? '加载失败' }}</StateText>
            <OverviewAreaStack
              v-else
              :groups="seriesGroups"
              :buckets="seriesBuckets"
              :points="costPointsForCurrency(currency)"
              :value-format="(v) => formatCurrencyCompact(v, currency)"
              :bucket-format="formatBucket"
            />
          </div>
        </DataCard>
      </template>
      <DataCard>
        <div class="p-4 flex flex-col gap-3">
          <span class="text-2xs font-medium text-ink-muted uppercase tracking-[0.03em]">请求数</span>
          <StateText v-if="seriesQuery.isLoading.value" compact :dashed="false">加载中…</StateText>
          <StateText v-else-if="seriesQuery.isError.value" compact :dashed="false">{{ (seriesQuery.error.value as Error)?.message ?? '加载失败' }}</StateText>
          <OverviewAreaStack
            v-else
            :groups="seriesGroups"
            :buckets="seriesBuckets"
            :points="seriesRequests"
            :value-format="(v) => compactNumber(v)"
            :bucket-format="formatBucket"
          />
        </div>
      </DataCard>
      <DataCard>
        <div class="p-4 flex flex-col gap-3">
          <span class="text-2xs font-medium text-ink-muted uppercase tracking-[0.03em]">追踪数</span>
          <StateText v-if="seriesQuery.isLoading.value" compact :dashed="false">加载中…</StateText>
          <StateText v-else-if="seriesQuery.isError.value" compact :dashed="false">{{ (seriesQuery.error.value as Error)?.message ?? '加载失败' }}</StateText>
          <OverviewAreaStack
            v-else
            :groups="seriesGroups"
            :buckets="seriesBuckets"
            :points="seriesTraces"
            :value-format="(v) => compactNumber(v)"
            :bucket-format="formatBucket"
          />
        </div>
      </DataCard>
    </div>
  </div>
</template>
