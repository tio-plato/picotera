<script setup lang="ts">
import { ref, reactive, watch, onMounted, computed } from 'vue'
import { useApi } from '@/composables/useApi'
import { useProvidersMap } from '@/composables/useProvidersMap'
import type {
  RequestView,
  EndpointView,
  ModelView,
} from '@/api'
import RequestDetailsPanel from '@/components/RequestDetailsPanel.vue'
import { useSidePanel } from '@/composables/useSidePanel'
import {
  Button,
  IconButton,
  DataCard,
  AutoDataTable,
  Tag,
  Field,
  Icon,
  SegmentedControl,
  ColumnFilter,
  type AutoDataTableColumn,
  type ColumnFilterOption,
} from '@/ui'

const api = useApi()
const panel = useSidePanel()
const { providers, providerLabel, fetchProviders } = useProvidersMap()

const requests = ref<RequestView[]>([])
const loading = ref(false)
const hasMore = ref(false)
const nextCursor = ref('')

const endpoints = ref<EndpointView[]>([])
const models = ref<ModelView[]>([])

type RequestKind = 'meta' | 'upstream' | 'all'

const filters = reactive({
  type: 'meta' as RequestKind,
  providerId: 0,
  endpointPath: '',
  model: '',
  upstreamModel: '',
})

const typeOptions: { value: RequestKind; label: string }[] = [
  { value: 'meta', label: '元请求' },
  { value: 'upstream', label: '上游请求' },
  { value: 'all', label: '全部' },
]

async function fetchReferenceData() {
  const [, endpointsRes, modelsRes] = await Promise.all([
    fetchProviders(),
    api.GET('/api/picotera/endpoints'),
    api.GET('/api/picotera/models'),
  ])
  endpoints.value = (endpointsRes.data as EndpointView[] | undefined) ?? []
  models.value = (modelsRes.data as ModelView[] | undefined) ?? []
}

async function fetchRequests(cursor?: string) {
  loading.value = true
  const query: Record<string, string | number | undefined> = {
    limit: 30,
    cursor: cursor || undefined,
  }
  if (filters.type === 'meta') query.type = 0
  else if (filters.type === 'upstream') query.type = 1
  if (filters.providerId) query.providerId = filters.providerId
  if (filters.endpointPath) query.endpointPath = filters.endpointPath
  if (filters.model) query.model = filters.model
  if (filters.upstreamModel) query.upstreamModel = filters.upstreamModel

  const { data, error } = await api.GET('/api/picotera/requests', {
    params: { query: query as never },
  })
  if (!error && data) {
    if (cursor) {
      requests.value.push(...(data.items ?? []))
    } else {
      requests.value = data.items ?? []
    }
    hasMore.value = data.pagination.hasMore
    nextCursor.value = data.pagination.nextCursor ?? ''
  }
  loading.value = false
}

onMounted(async () => {
  await fetchReferenceData()
  fetchRequests()
})

watch(
  () => [filters.type, filters.providerId, filters.endpointPath, filters.model, filters.upstreamModel],
  () => {
    fetchRequests()
  },
)

function rowKey(r: RequestView) {
  return r.id
}

function openDetails(r: RequestView) {
  panel.toggle(
    RequestDetailsPanel,
    { requestId: r.id, providers: providers.value },
    { key: `request:${r.id}`, width: '520px' },
  )
}

function rowSelected(r: RequestView) {
  return panel.isActive(`request:${r.id}`)
}

const columns = computed<AutoDataTableColumn<RequestView>[]>(() => {
  const base: AutoDataTableColumn<RequestView>[] = [
    { key: 'createdAt', header: '时间' },
  ]
  if (filters.type === 'all') {
    base.push({ key: 'type', header: '类型' })
  }
  base.push(
    {
      key: 'providerId',
      header: '渠道',
      headerClass: filters.providerId ? 'shadow-[inset_0_-2px_0_var(--color-accent)]' : '',
    },
    {
      key: 'endpointPath',
      header: '端点',
      headerClass: filters.endpointPath ? 'shadow-[inset_0_-2px_0_var(--color-accent)]' : '',
    },
    {
      key: 'model',
      headerClass: (filters.model || filters.upstreamModel) ? 'shadow-[inset_0_-2px_0_var(--color-accent)]' : '',
    },
    { key: 'status', header: '状态' },
    { key: 'tokens', header: 'Token' },
    { key: 'timeSpentMs', header: '耗时', align: 'right' },
  )
  return base
})

const providerOptions = computed<ColumnFilterOption<number>[]>(() =>
  providers.value.map((p) => ({ value: p.id, label: p.name })),
)
const endpointOptions = computed<ColumnFilterOption<string>[]>(() =>
  endpoints.value.map((e) => ({ value: e.path, label: e.path })),
)
const modelOptions = computed<ColumnFilterOption<string>[]>(() =>
  models.value.map((m) => ({ value: m.name, label: m.name })),
)
const upstreamModelOptions = computed<ColumnFilterOption<string>[]>(() => {
  const seen = new Set<string>()
  const opts: ColumnFilterOption<string>[] = []
  for (const r of requests.value) {
    if (r.upstreamModel && !seen.has(r.upstreamModel)) {
      seen.add(r.upstreamModel)
      opts.push({ value: r.upstreamModel, label: r.upstreamModel })
    }
  }
  return opts
})

function activeFilterCount(): number {
  let n = 0
  if (filters.providerId) n++
  if (filters.endpointPath) n++
  if (filters.model) n++
  if (filters.upstreamModel) n++
  return n
}

function clearAllFilters() {
  filters.providerId = 0
  filters.endpointPath = ''
  filters.model = ''
  filters.upstreamModel = ''
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

function statusVariant(code: number | undefined): 'ok' | 'warn' | 'err' {
  if (code === undefined) return 'err'
  if (code >= 200 && code < 300) return 'ok'
  if (code >= 400 && code < 500) return 'warn'
  return 'err'
}

type RequestState = 'pending' | 'ok' | 'warn' | 'err'
function requestState(r: RequestView): RequestState {
  // status: 0=Pending 1=HeaderReceived 2=Completed 3=Failed
  if (r.status === 0 || r.status === 1) return 'pending'
  return statusVariant(r.statusCode)
}

function formatTimeSpent(ms: number | undefined): string {
  if (ms === undefined) return '—'
  if (ms < 1000) return `${ms}ms`
  return `${parseFloat((ms / 1000).toFixed(1))}s`
}

function outputSpeed(r: RequestView): string | null {
  if (!r.outputTokens || !r.timeSpentMs) return null
  const seconds = r.timeSpentMs / 1000
  if (seconds <= 0) return null
  return (r.outputTokens / seconds).toFixed(0)
}

function resetCursorAndReload() {
  fetchRequests()
}
</script>

<template>
  <div class="flex flex-col gap-3.5">
    <div class="flex items-end justify-between gap-3 flex-wrap">
      <Field label="类型" as="div">
        <SegmentedControl v-model="filters.type" :options="typeOptions" />
      </Field>
      <div class="flex items-center gap-2">
        <button
          v-if="activeFilterCount() > 0"
          type="button"
          class="inline-flex items-center gap-1 px-1.5 py-0.5 bg-transparent border-0 rounded-xs text-xs text-ink-faint cursor-pointer transition-colors hover:text-ink hover:bg-surface-100"
          @click="clearAllFilters"
        >
          <Icon name="close" :size="11" />
          <span>清除筛选 ({{ activeFilterCount() }})</span>
        </button>
        <span class="text-xs text-ink-faint tabular-nums">
          {{ requests.length }} 条<span v-if="hasMore">（还有更多）</span>
        </span>
        <IconButton title="刷新" aria-label="刷新" @click="resetCursorAndReload">
          <Icon name="refresh" :size="13" />
        </IconButton>
      </div>
    </div>

    <DataCard>
      <AutoDataTable
        :columns="columns"
        :items="requests"
        :row-key="rowKey"
        :selected="rowSelected"
        :on-row-click="(r) => openDetails(r)"
      >
        <template #header-providerId>
          <ColumnFilter
            v-model.number="filters.providerId"
            label="渠道"
            :options="providerOptions"
            :empty-value="0"
            placeholder="过滤渠道…"
          />
        </template>
        <template #header-endpointPath>
          <ColumnFilter
            v-model="filters.endpointPath"
            label="端点"
            :options="endpointOptions"
            placeholder="过滤端点…"
          />
        </template>
        <template #header-model>
          <ColumnFilter
            v-model="filters.upstreamModel"
            label="模型"
            :options="upstreamModelOptions"
            placeholder="过滤实际上游请求模型…"
          />
          <ColumnFilter
            v-model="filters.model"
            label="上游"
            :options="modelOptions"
            placeholder="过滤客户端请求模型…"
          />
        </template>
        <template #cell-createdAt="{ row }">
          <div class="flex flex-col leading-tight">
            <span class="font-mono tabular-nums text-ink">{{ formatTimeParts(row.createdAt).time }}</span>
            <span class="font-mono text-2xs text-ink-faint">{{ formatTimeParts(row.createdAt).date }}</span>
          </div>
        </template>
        <template #cell-type="{ row }">
          <Tag :variant="row.type === 0 ? 'accent' : 'muted'">{{ row.type === 0 ? 'META' : 'UP' }}</Tag>
        </template>
        <template #cell-providerId="{ row }">
          <span v-if="row.providerId" class="font-medium">{{ providerLabel(row.providerId) }}</span>
          <span v-else class="text-ink-faint">—</span>
        </template>
        <template #cell-endpointPath="{ row }">
          <span class="font-mono text-ink-faint">{{ row.endpointPath }}</span>
        </template>
        <template #cell-model="{ row }">
          <div class="flex flex-col leading-tight">
            <span v-if="row.upstreamModel" class="font-mono text-ink">{{ row.upstreamModel }}</span>
            <span v-else-if="row.model" class="font-mono text-ink">{{ row.model }}</span>
            <span v-else class="text-ink-faint">—</span>
            <span
              v-if="row.model && row.upstreamModel && row.model !== row.upstreamModel"
              class="font-mono text-2xs text-ink-faint"
            >{{ row.model }}</span>
          </div>
        </template>
        <template #cell-status="{ row }">
          <div class="inline-flex items-center gap-1.5">
            <span
              v-if="requestState(row) === 'pending'"
              class="inline-flex items-center px-1.5 py-0.5 rounded-[5px] font-mono text-2xs leading-[1.2] bg-surface-100 text-ink-muted border border-line-soft"
            >...</span>
            <span
              v-else
              class="inline-flex items-center px-1.5 py-0.5 rounded-[5px] font-mono text-2xs leading-[1.2] border border-transparent"
              :class="{
                'bg-ok-faint text-ok-ink': requestState(row) === 'ok',
                'bg-warn-faint text-warn-ink': requestState(row) === 'warn',
                'bg-err-faint text-err-ink': requestState(row) === 'err',
              }"
            >{{ row.statusCode }}</span>
          </div>
        </template>
        <template #cell-tokens="{ row }">
          <div class="text-xs">
            <span class="font-mono tabular-nums text-ink">
              {{ ((row.inputTokens || 0 ) + (row.outputTokens || 0) + (row.cacheReadTokens || 0) + (row.cacheWriteTokens || 0)).toLocaleString() }}
            </span>
            <div v-if="row.cacheReadTokens || row.cacheWriteTokens" class="flex items-center gap-1.5 mt-0.5 text-ink-faint text-2xs">
              <span v-if="row.cacheReadTokens" >{{ parseFloat((row.cacheReadTokens / (((row.inputTokens || 0) + row.cacheReadTokens + (row.cacheWriteTokens || 0)) || 1) * 100).toFixed(2)) }}%</span>
            </div>
          </div>
        </template>
        <template #cell-timeSpentMs="{ row }">
          <div class="flex flex-col items-end leading-tight">
            <span class="font-mono tabular-nums text-ink">{{ formatTimeSpent(row.timeSpentMs) }}</span>
            <span v-if="row.ttftMs != null || outputSpeed(row)" class="font-mono text-2xs text-ink-faint tabular-nums">
              <span v-if="row.ttftMs != null" title="TTFT">{{ formatTimeSpent(row.ttftMs) }}</span>
              <span v-if="row.ttftMs != null && outputSpeed(row)" class="px-0.5">&middot;</span>
              <span v-if="outputSpeed(row)" title="输出速度">{{ outputSpeed(row) }}<span class="pl-0.5">tps</span></span>
            </span>
          </div>
        </template>
        <template #empty>
          <span v-if="loading">加载中…</span>
          <span v-else>暂无请求</span>
        </template>
      </AutoDataTable>
    </DataCard>

    <div v-if="hasMore" class="flex justify-center py-1">
      <Button variant="ghost" :disabled="loading" @click="fetchRequests(nextCursor)">
        {{ loading ? '加载中…' : '加载更多' }}
      </Button>
    </div>
  </div>
</template>
