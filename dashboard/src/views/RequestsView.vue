<script setup lang="ts">
import { ref, reactive, watch, onMounted, computed } from 'vue'
import { useApi } from '@/composables/useApi'
import type {
  RequestView,
  ProviderView,
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
  StateText,
  Tag,
  Select,
  Field,
  Icon,
  SegmentedControl,
  type AutoDataTableColumn,
} from '@/ui'

const api = useApi()
const panel = useSidePanel()

const requests = ref<RequestView[]>([])
const loading = ref(false)
const hasMore = ref(false)
const nextCursor = ref('')

const providers = ref<ProviderView[]>([])
const endpoints = ref<EndpointView[]>([])
const models = ref<ModelView[]>([])

const providersMap = computed(() => {
  const m = new Map<number, ProviderView>()
  for (const p of providers.value) m.set(p.id, p)
  return m
})

type RequestKind = 'meta' | 'upstream' | 'all'

const filters = reactive({
  type: 'meta' as RequestKind,
  providerId: 0,
  endpointPath: '',
  model: '',
})

const typeOptions: { value: RequestKind; label: string }[] = [
  { value: 'meta', label: '元请求' },
  { value: 'upstream', label: '上游请求' },
  { value: 'all', label: '全部' },
]

async function fetchReferenceData() {
  const [providersRes, endpointsRes, modelsRes] = await Promise.all([
    api.GET('/api/picotera/providers'),
    api.GET('/api/picotera/endpoints'),
    api.GET('/api/picotera/models'),
  ])
  providers.value = (providersRes.data as ProviderView[] | undefined) ?? []
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
  () => [filters.type, filters.providerId, filters.endpointPath, filters.model],
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
    { key: 'providerId', header: '渠道' },
    { key: 'endpointPath', header: '端点' },
    { key: 'model', header: '模型' },
    { key: 'status', header: '状态' },
    { key: 'tokens', header: 'Token' },
    { key: 'timeSpentMs', header: '耗时', align: 'right' },
  )
  return base
})

function formatTimeParts(iso: string): { time: string; date: string } {
  if (!iso) return { time: '—', date: '' }
  const d = new Date(iso)
  if (Number.isNaN(d.getTime())) return { time: iso, date: '' }
  const pad = (n: number) => String(n).padStart(2, '0')
  const time = `${pad(d.getHours())}:${pad(d.getMinutes())}:${pad(d.getSeconds())}`
  const date = `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}`
  return { time, date }
}

function providerLabel(id: number): string {
  const p = providersMap.value.get(id)
  return p ? p.name : `#${id}`
}

function statusVariant(code: number): 'ok' | 'warn' | 'err' {
  if (code >= 200 && code < 300) return 'ok'
  if (code >= 400 && code < 500) return 'warn'
  return 'err'
}

function formatTimeSpent(ms: number): string {
  if (ms < 1000) return `${ms}ms`
  return `${(ms / 1000).toFixed(1)}s`
}

function resetCursorAndReload() {
  fetchRequests()
}
</script>

<template>
  <div class="flex flex-col gap-3.5">
    <div class="flex items-end justify-between gap-3 flex-wrap">
      <div class="flex items-end gap-2.5 flex-wrap">
        <Field label="类型" as="div">
          <SegmentedControl v-model="filters.type" :options="typeOptions" />
        </Field>
        <Field label="渠道" as="div" class="min-w-40">
          <Select v-model.number="filters.providerId" size="sm">
            <option :value="0">全部</option>
            <option v-for="p in providers" :key="p.id" :value="p.id">
              {{ p.name }}
            </option>
          </Select>
        </Field>
        <Field label="端点" as="div" class="min-w-48">
          <Select v-model="filters.endpointPath" size="sm">
            <option value="">全部</option>
            <option v-for="e in endpoints" :key="e.path" :value="e.path">
              {{ e.path }}
            </option>
          </Select>
        </Field>
        <Field label="模型" as="div" class="min-w-48">
          <Select v-model="filters.model" size="sm">
            <option value="">全部</option>
            <option v-for="m in models" :key="m.name" :value="m.name">
              {{ m.name }}
            </option>
          </Select>
        </Field>
      </div>
      <div class="flex items-center gap-2">
        <span class="text-xs text-ink-faint tabular-nums">
          {{ requests.length }} 条<span v-if="hasMore">（还有更多）</span>
        </span>
        <IconButton title="刷新" aria-label="刷新" @click="resetCursorAndReload">
          <Icon name="refresh" :size="13" />
        </IconButton>
      </div>
    </div>

    <StateText v-if="loading && !requests.length">加载中…</StateText>
    <DataCard v-else-if="requests.length">
      <AutoDataTable
        :columns="columns"
        :items="requests"
        :row-key="rowKey"
        :selected="rowSelected"
        :on-row-click="(r) => openDetails(r)"
      >
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
          <span v-if="row.model" class="font-mono">{{ row.model }}</span>
          <span v-else class="text-ink-faint">—</span>
        </template>
        <template #cell-status="{ row }">
          <div class="inline-flex items-center gap-1.5">
            <span
              class="inline-flex items-center px-1.5 py-0.5 rounded-[5px] font-mono text-2xs leading-[1.2] border border-transparent"
              :class="{
                'bg-ok-faint text-ok-ink': statusVariant(row.statusCode) === 'ok',
                'bg-warn-faint text-warn-ink': statusVariant(row.statusCode) === 'warn',
                'bg-err-faint text-err-ink': statusVariant(row.statusCode) === 'err',
              }"
            >{{ row.statusCode || 'ERR' }}</span>
          </div>
        </template>
        <template #cell-tokens="{ row }">
          <div class="flex items-center gap-1.5 text-xs">
            <span class="font-mono tabular-nums text-ink">
              {{ row.inputTokens ?? 0 }}<span class="text-ink-faint">/</span>{{ row.outputTokens ?? 0 }}
            </span>
            <Tag v-if="row.cacheReadTokens" variant="muted">r {{ row.cacheReadTokens }}</Tag>
            <Tag v-if="row.cacheWriteTokens" variant="muted">w {{ row.cacheWriteTokens }}</Tag>
          </div>
        </template>
        <template #cell-timeSpentMs="{ row }">
          <span class="font-mono tabular-nums text-ink">{{ formatTimeSpent(row.timeSpentMs) }}</span>
        </template>
      </AutoDataTable>
    </DataCard>
    <StateText v-else>暂无请求</StateText>

    <div v-if="hasMore" class="flex justify-center py-1">
      <Button variant="ghost" :disabled="loading" @click="fetchRequests(nextCursor)">
        {{ loading ? '加载中…' : '加载更多' }}
      </Button>
    </div>
  </div>
</template>
