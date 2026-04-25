<script setup lang="ts">
import { ref, onMounted, watch, computed } from 'vue'
import { useApi } from '@/composables/useApi'
import type { RequestView, ProviderView } from '@/api'
import { SidePanel, StateText, Field, Tag, IconButton, Icon } from '@/ui'

const props = defineProps<{ requestId: string; providers?: ProviderView[] }>()
const emit = defineEmits<{ close: [] }>()
const api = useApi()

const spans = ref<RequestView[]>([])
const selectedId = ref<string>('')
const loading = ref(false)
const error = ref('')

const providersMap = computed(() => {
  const m = new Map<number, ProviderView>()
  for (const p of props.providers ?? []) m.set(p.id, p)
  return m
})

const meta = computed(() => spans.value.find(s => s.id === s.spanId) ?? null)
const upstreams = computed(() =>
  spans.value.filter(s => s.id !== s.spanId),
)
const selected = computed(
  () => spans.value.find(s => s.id === selectedId.value) ?? null,
)

async function fetchSpans() {
  loading.value = true
  error.value = ''
  const { data, error: err } = await api.GET(
    '/api/picotera/requests/{id}/spans',
    { params: { path: { id: props.requestId } } },
  )
  loading.value = false
  if (err) {
    error.value = err.message ?? '加载请求失败'
    return
  }
  const items = (data as RequestView[] | undefined) ?? []
  const sorted = [...items].sort((a, b) => {
    const aMeta = a.id === a.spanId ? 0 : 1
    const bMeta = b.id === b.spanId ? 0 : 1
    if (aMeta !== bMeta) return aMeta - bMeta
    const at = a.createdAt || ''
    const bt = b.createdAt || ''
    return at < bt ? -1 : at > bt ? 1 : 0
  })
  spans.value = sorted
  if (!selectedId.value || !sorted.find(s => s.id === selectedId.value)) {
    selectedId.value = sorted[0]?.id ?? ''
  }
}

onMounted(fetchSpans)
watch(() => props.requestId, () => {
  selectedId.value = ''
  fetchSpans()
})

const shortId = computed(() => {
  const id = props.requestId
  return id.length > 10 ? id.slice(0, 10) : id
})

function formatTime(iso: string | undefined) {
  if (!iso) return '—'
  const d = new Date(iso)
  if (Number.isNaN(d.getTime())) return iso
  return d.toLocaleString()
}

function formatTimeSpent(ms: number | undefined | null) {
  if (ms === undefined || ms === null) return '—'
  if (ms < 1000) return `${ms} ms`
  return `${(ms / 1000).toFixed(2)} s`
}

function fmtNum(n: number | undefined | null) {
  return n === undefined || n === null ? '—' : String(n)
}

function providerLabel(id: number | undefined | null) {
  if (!id) return '—'
  const p = providersMap.value.get(id)
  return p ? p.name : `#${id}`
}

function statusVariantTag(code: number | undefined | null): 'ok' | 'default' | 'muted' | 'accent' {
  if (!code) return 'muted'
  if (code >= 200 && code < 300) return 'ok'
  return 'default'
}

function statusCodeClass(code: number | undefined | null) {
  const c = code ?? 0
  if (c >= 200 && c < 300) return 'bg-ok-faint text-ok-ink'
  if (c >= 400 && c < 500) return 'bg-warn-faint text-warn-ink'
  return 'bg-err-faint text-err-ink'
}

function typeLabel(t: number) {
  return t === 0 ? 'META' : 'UPSTREAM'
}

function statusLabel(s: number) {
  switch (s) {
    case 0: return 'pending'
    case 1: return 'header'
    case 2: return 'completed'
    case 3: return 'failed'
    default: return String(s)
  }
}
</script>

<template>
  <SidePanel title="请求详情" :kicker="shortId" @close="emit('close')">
    <StateText v-if="loading && !spans.length" :dashed="false" compact>加载中…</StateText>
    <template v-else-if="spans.length">
      <div class="flex items-start gap-2">
        <div class="flex gap-2 overflow-x-auto -mx-1 px-1 pb-1 flex-1 min-w-0">
          <button
            v-if="meta"
            type="button"
            class="flex flex-col gap-1 shrink-0 min-w-44 max-w-56 p-2.5 rounded-md border text-left transition-colors cursor-pointer"
            :class="
              selectedId === meta.id
                ? 'border-accent bg-accent-faint'
                : 'border-line hover:bg-surface-50'
            "
            @click="selectedId = meta.id"
          >
            <div class="flex items-center justify-between gap-2">
              <span class="text-2xs font-semibold text-accent-ink uppercase tracking-[0.04em]">META</span>
              <span
                class="inline-flex items-center px-1.5 py-0.5 rounded-[5px] font-mono text-2xs leading-[1.2]"
                :class="statusCodeClass(meta.statusCode)"
              >{{ meta.statusCode || '—' }}</span>
            </div>
            <div class="font-medium text-sm text-ink truncate">
              {{ providerLabel(meta.providerId) }}
            </div>
            <div class="font-mono tabular-nums text-2xs text-ink-faint">
              {{ formatTimeSpent(meta.timeSpentMs) }}
            </div>
          </button>
          <button
            v-for="(s, idx) in upstreams"
            :key="s.id"
            type="button"
            class="flex flex-col gap-1 shrink-0 min-w-44 max-w-56 p-2.5 rounded-md border text-left transition-colors cursor-pointer"
            :class="
              selectedId === s.id
                ? 'border-accent bg-accent-faint'
                : 'border-line hover:bg-surface-50'
            "
            @click="selectedId = s.id"
          >
            <div class="flex items-center justify-between gap-2">
              <span class="text-2xs font-semibold text-ink-muted uppercase tracking-[0.04em]">#{{ idx + 1 }}</span>
              <span
                class="inline-flex items-center px-1.5 py-0.5 rounded-[5px] font-mono text-2xs leading-[1.2]"
                :class="statusCodeClass(s.statusCode)"
              >{{ s.statusCode || '—' }}</span>
            </div>
            <div class="font-medium text-sm text-ink truncate">
              {{ providerLabel(s.providerId) }}
            </div>
            <div class="font-mono tabular-nums text-2xs text-ink-faint">
              {{ formatTimeSpent(s.timeSpentMs) }}
            </div>
          </button>
        </div>
        <IconButton title="刷新" aria-label="刷新" @click="fetchSpans">
          <Icon name="refresh" :size="13" />
        </IconButton>
      </div>

      <template v-if="selected">
        <section class="flex flex-col gap-2.5">
          <span class="text-2xs font-medium text-ink-muted uppercase tracking-[0.04em]">基本信息</span>
          <div class="grid grid-cols-2 gap-2.5">
            <Field label="ID" as="div" class="col-span-2">
              <span class="font-mono text-xs text-ink break-all">{{ selected.id }}</span>
            </Field>
            <Field label="类型" as="div">
              <Tag :variant="selected.type === 0 ? 'accent' : 'muted'">{{ typeLabel(selected.type) }}</Tag>
            </Field>
            <Field label="状态" as="div">
              <Tag :variant="statusVariantTag(selected.statusCode)">{{ statusLabel(selected.status) }}</Tag>
            </Field>
            <Field v-if="selected.spanId" label="Span" as="div" class="col-span-2">
              <span class="font-mono text-xs text-ink break-all">{{ selected.spanId }}</span>
            </Field>
            <Field v-if="selected.parentSpanId" label="Parent Span" as="div" class="col-span-2">
              <span class="font-mono text-xs text-ink break-all">{{ selected.parentSpanId }}</span>
            </Field>
            <Field label="渠道" as="div">
              <span class="font-mono text-sm">{{ providerLabel(selected.providerId) }}</span>
            </Field>
            <Field label="端点" as="div">
              <span class="font-mono text-sm">{{ selected.endpointPath || '—' }}</span>
            </Field>
            <Field label="模型" as="div" class="col-span-2">
              <span class="font-mono text-sm">{{ selected.model || '—' }}</span>
            </Field>
            <Field label="状态码" as="div">
              <span
                class="inline-flex items-center px-1.5 py-0.5 rounded-[5px] font-mono text-xs border border-transparent w-fit"
                :class="statusCodeClass(selected.statusCode)"
              >{{ selected.statusCode || '—' }}</span>
            </Field>
            <Field label="时间" as="div">
              <span class="font-mono text-xs">{{ formatTime(selected.createdAt) }}</span>
            </Field>
          </div>
        </section>

        <section class="flex flex-col gap-2.5">
          <span class="text-2xs font-medium text-ink-muted uppercase tracking-[0.04em]">性能</span>
          <div class="grid grid-cols-2 gap-2.5">
            <Field label="TTFT" as="div">
              <span class="font-mono tabular-nums text-sm">{{ formatTimeSpent(selected.ttftMs) }}</span>
            </Field>
            <Field label="总耗时" as="div">
              <span class="font-mono tabular-nums text-sm">{{ formatTimeSpent(selected.timeSpentMs) }}</span>
            </Field>
          </div>
        </section>

        <section class="flex flex-col gap-2.5">
          <span class="text-2xs font-medium text-ink-muted uppercase tracking-[0.04em]">Token</span>
          <div class="grid grid-cols-2 gap-2.5">
            <Field label="输入" as="div">
              <span class="font-mono tabular-nums text-sm">{{ fmtNum(selected.inputTokens) }}</span>
            </Field>
            <Field label="输出" as="div">
              <span class="font-mono tabular-nums text-sm">{{ fmtNum(selected.outputTokens) }}</span>
            </Field>
            <Field label="缓存读取" as="div">
              <span class="font-mono tabular-nums text-sm">{{ fmtNum(selected.cacheReadTokens) }}</span>
            </Field>
            <Field label="缓存写入" as="div">
              <span class="font-mono tabular-nums text-sm">{{ fmtNum(selected.cacheWriteTokens) }}</span>
            </Field>
          </div>
        </section>

        <section v-if="selected.errorMessage" class="flex flex-col gap-2.5">
          <span class="text-2xs font-medium text-ink-muted uppercase tracking-[0.04em]">错误信息</span>
          <pre class="font-mono text-xs whitespace-pre-wrap bg-surface-50 border border-line-soft rounded-md p-3 m-0 text-ink">{{ selected.errorMessage }}</pre>
        </section>
      </template>
    </template>

    <template v-if="error" #error>{{ error }}</template>
  </SidePanel>
</template>
