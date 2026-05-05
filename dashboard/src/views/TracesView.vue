<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { useApi } from '@/composables/useApi'
import { useCurrency } from '@/composables/useCurrency'
import { useExchangeRatesStore } from '@/stores/exchangeRates'
import type { RequestTraceView, TraceCostView } from '@/api'
import {
  AutoDataTable,
  Button,
  DataCard,
  Icon,
  IconButton,
  type AutoDataTableColumn,
} from '@/ui'

const api = useApi()
const router = useRouter()
const currency = useCurrency()
const exchange = useExchangeRatesStore()

const traces = ref<RequestTraceView[]>([])
const loading = ref(false)
const hasMore = ref(false)
const nextCursor = ref('')

const columns = computed<AutoDataTableColumn<RequestTraceView>[]>(() => [
  { key: 'lastRequestAt', header: '最近请求' },
  { key: 'parentSpanId', header: 'Parent Span ID' },
  { key: 'requestCount', header: '请求', align: 'right' },
  { key: 'totalTokens', header: 'Token', align: 'right' },
  { key: 'cacheHitRate', header: '缓存命中', align: 'right' },
  { key: 'modelCosts', header: '模型成本', align: 'right' },
  { key: 'upstreamCosts', header: '上游成本', align: 'right' },
])

async function fetchTraces(cursor?: string) {
  loading.value = true
  try {
    const { data, error } = await api.GET('/api/picotera/request-traces', {
      params: {
        query: {
          limit: 30,
          cursor: cursor || undefined,
        },
      },
    })
    if (!error && data) {
      if (cursor) {
        traces.value.push(...(data.items ?? []))
      } else {
        traces.value = data.items ?? []
      }
      hasMore.value = data.pagination.hasMore
      nextCursor.value = data.pagination.nextCursor ?? ''
    }
  } finally {
    loading.value = false
  }
}

onMounted(async () => {
  await exchange.fetch().catch(() => {
    // Missing rates are reflected by native-currency cost display.
  })
  fetchTraces()
})

function rowKey(row: RequestTraceView) {
  return row.parentSpanId
}

function openTrace(row: RequestTraceView) {
  router.push({ name: 'requests', query: { parentSpanId: row.parentSpanId } })
}

function formatTimeParts(iso: string | undefined): { time: string; date: string } {
  if (!iso) return { time: '—', date: '' }
  const d = new Date(iso)
  if (Number.isNaN(d.getTime())) return { time: iso, date: '' }
  const pad = (n: number) => String(n).padStart(2, '0')
  const time = `${pad(d.getHours())}:${pad(d.getMinutes())}:${pad(d.getSeconds())}`
  const date = `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}`
  return { time, date }
}

function formatNumber(value: number): string {
  return value.toLocaleString()
}

function cacheHitRate(row: RequestTraceView): number | null {
  const denominator = row.inputTokens + row.cacheReadTokens + row.cacheWriteTokens
  if (denominator <= 0) return null
  return row.cacheReadTokens / denominator
}

function formatCacheHitRate(row: RequestTraceView): string {
  const rate = cacheHitRate(row)
  if (rate == null) return '—'
  return `${parseFloat((rate * 100).toFixed(2))}%`
}

function normalizeCosts(costs: TraceCostView[] | null): TraceCostView[] {
  return costs ?? []
}

function nativeCostText(costs: TraceCostView[] | null): string {
  const items = normalizeCosts(costs)
  if (items.length === 0) return '—'
  return items.map((c) => currency.format(c.amount, c.currency)).join(' + ')
}

function formatCosts(costs: TraceCostView[] | null): { text: string; title?: string } {
  const items = normalizeCosts(costs)
  if (items.length === 0) return { text: '—' }
  const target = currency.targetCurrency.value
  if (!target) return { text: nativeCostText(items) }

  let total = 0
  for (const cost of items) {
    const converted = currency.convert(cost.amount, cost.currency)
    if (converted.currency !== target) {
      return { text: nativeCostText(items) }
    }
    total += converted.amount
  }

  return {
    text: currency.format(total, target),
    title: nativeCostText(items),
  }
}
</script>

<template>
  <div class="flex flex-col gap-3.5">
    <div class="flex items-center justify-between gap-3 flex-wrap">
      <span class="text-xs text-ink-faint tabular-nums">
        {{ traces.length }} 条追踪<span v-if="hasMore">（还有更多）</span>
      </span>
      <IconButton title="刷新" aria-label="刷新" :disabled="loading" @click="fetchTraces()">
        <Icon name="refresh" :size="13" />
      </IconButton>
    </div>

    <DataCard>
      <AutoDataTable
        :columns="columns"
        :items="traces"
        :row-key="rowKey"
        :on-row-click="openTrace"
      >
        <template #cell-lastRequestAt="{ row }">
          <div class="flex flex-col leading-tight">
            <span class="font-mono tabular-nums text-ink">{{ formatTimeParts(row.lastRequestAt).time }}</span>
            <span class="font-mono text-2xs text-ink-faint">{{ formatTimeParts(row.lastRequestAt).date }}</span>
          </div>
        </template>
        <template #cell-parentSpanId="{ row }">
          <span class="block max-w-[34rem] truncate font-mono text-ink" :title="row.parentSpanId">
            {{ row.parentSpanId }}
          </span>
        </template>
        <template #cell-requestCount="{ row }">
          <span class="font-mono tabular-nums">{{ row.requestCount.toLocaleString() }}</span>
        </template>
        <template #cell-totalTokens="{ row }">
          <div class="flex flex-col items-end leading-tight">
            <span class="font-mono tabular-nums text-ink">{{ formatNumber(row.totalTokens) }}</span>
            <span class="font-mono text-2xs text-ink-faint tabular-nums">
              {{ formatNumber(row.inputTokens + row.cacheReadTokens + row.cacheWriteTokens) }} / {{ formatNumber(row.outputTokens) }}
            </span>
          </div>
        </template>
        <template #cell-cacheHitRate="{ row }">
          <span
            class="font-mono tabular-nums"
            :class="cacheHitRate(row) == null ? 'text-ink-faint' : 'text-ink'"
            :title="`cache read ${formatNumber(row.cacheReadTokens)} / input ${formatNumber(row.inputTokens + row.cacheReadTokens + row.cacheWriteTokens)}`"
          >
            {{ formatCacheHitRate(row) }}
          </span>
        </template>
        <template #cell-modelCosts="{ row }">
          <span
            class="font-mono tabular-nums"
            :class="normalizeCosts(row.modelCosts).length === 0 ? 'text-ink-faint' : 'text-ink'"
            :title="formatCosts(row.modelCosts).title"
          >
            {{ formatCosts(row.modelCosts).text }}
          </span>
        </template>
        <template #cell-upstreamCosts="{ row }">
          <span
            class="font-mono tabular-nums"
            :class="normalizeCosts(row.upstreamCosts).length === 0 ? 'text-ink-faint' : 'text-ink'"
            :title="formatCosts(row.upstreamCosts).title"
          >
            {{ formatCosts(row.upstreamCosts).text }}
          </span>
        </template>
        <template #empty>
          <span v-if="loading">加载中…</span>
          <span v-else>暂无追踪</span>
        </template>
      </AutoDataTable>
    </DataCard>

    <div v-if="hasMore" class="flex justify-center py-1">
      <Button variant="ghost" :disabled="loading" @click="fetchTraces(nextCursor)">
        {{ loading ? '加载中…' : '加载更多' }}
      </Button>
    </div>
  </div>
</template>
