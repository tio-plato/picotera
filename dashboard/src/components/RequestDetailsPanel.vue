<script setup lang="ts">
import { ref, onMounted, watch, computed } from 'vue'
import { useApi } from '@/composables/useApi'
import type { RequestView } from '@/api'
import { SidePanel, StateText, Field } from '@/ui'

const props = defineProps<{ requestId: string }>()
const emit = defineEmits<{ close: [] }>()
const api = useApi()

const request = ref<RequestView | null>(null)
const loading = ref(false)
const error = ref('')

async function fetchRequest() {
  loading.value = true
  error.value = ''
  request.value = null
  const { data, error: err } = await api.GET('/api/picotera/requests/{id}', {
    params: { path: { id: props.requestId } },
  })
  loading.value = false
  if (err) {
    error.value = err.message ?? '加载请求失败'
    return
  }
  request.value = data as RequestView
}

onMounted(fetchRequest)
watch(() => props.requestId, fetchRequest)

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

function formatTimeSpent(ms: number | undefined) {
  if (ms === undefined || ms === null) return '—'
  if (ms < 1000) return `${ms} ms`
  return `${(ms / 1000).toFixed(2)} s`
}

function fmtNum(n: number | undefined) {
  return n === undefined || n === null ? '—' : String(n)
}
</script>

<template>
  <SidePanel title="请求详情" :kicker="shortId" @close="emit('close')">
    <StateText v-if="loading" :dashed="false" compact>加载中…</StateText>
    <template v-else-if="request">
      <section class="flex flex-col gap-2.5">
        <span class="text-2xs font-medium text-ink-muted uppercase tracking-[0.04em]">基本信息</span>
        <div class="grid grid-cols-2 gap-2.5">
          <Field label="ID" as="div" class="col-span-2">
            <span class="font-mono text-xs text-ink break-all">{{ request.id }}</span>
          </Field>
          <Field v-if="request.spanId" label="Span" as="div">
            <span class="font-mono text-xs text-ink break-all">{{ request.spanId }}</span>
          </Field>
          <Field v-if="request.parentSpanId" label="Parent Span" as="div">
            <span class="font-mono text-xs text-ink break-all">{{ request.parentSpanId }}</span>
          </Field>
          <Field label="渠道" as="div">
            <span class="font-mono text-sm">#{{ request.providerId }}</span>
          </Field>
          <Field label="端点" as="div">
            <span class="font-mono text-sm">{{ request.endpointPath }}</span>
          </Field>
          <Field label="模型" as="div" class="col-span-2">
            <span class="font-mono text-sm">{{ request.model || '—' }}</span>
          </Field>
          <Field label="状态码" as="div">
            <span
              class="inline-flex items-center px-1.5 py-0.5 rounded-[5px] font-mono text-xs border border-transparent w-fit"
              :class="{
                'bg-ok-faint text-ok-ink': request.statusCode >= 200 && request.statusCode < 300,
                'bg-warn-faint text-warn-ink': request.statusCode >= 400 && request.statusCode < 500,
                'bg-err-faint text-err-ink': request.statusCode === 0 || request.statusCode >= 500,
              }"
            >{{ request.statusCode || 'ERR' }}</span>
          </Field>
          <Field label="时间" as="div">
            <span class="font-mono text-xs">{{ formatTime(request.createdAt) }}</span>
          </Field>
        </div>
      </section>

      <section class="flex flex-col gap-2.5">
        <span class="text-2xs font-medium text-ink-muted uppercase tracking-[0.04em]">性能</span>
        <div class="grid grid-cols-2 gap-2.5">
          <Field label="TTFT" as="div">
            <span class="font-mono tabular-nums text-sm">{{ formatTimeSpent(request.ttftMs ?? undefined) }}</span>
          </Field>
          <Field label="总耗时" as="div">
            <span class="font-mono tabular-nums text-sm">{{ formatTimeSpent(request.timeSpentMs) }}</span>
          </Field>
        </div>
      </section>

      <section class="flex flex-col gap-2.5">
        <span class="text-2xs font-medium text-ink-muted uppercase tracking-[0.04em]">Token</span>
        <div class="grid grid-cols-2 gap-2.5">
          <Field label="输入" as="div">
            <span class="font-mono tabular-nums text-sm">{{ fmtNum(request.inputTokens ?? undefined) }}</span>
          </Field>
          <Field label="输出" as="div">
            <span class="font-mono tabular-nums text-sm">{{ fmtNum(request.outputTokens ?? undefined) }}</span>
          </Field>
          <Field label="缓存读取" as="div">
            <span class="font-mono tabular-nums text-sm">{{ fmtNum(request.cacheReadTokens ?? undefined) }}</span>
          </Field>
          <Field label="缓存写入" as="div">
            <span class="font-mono tabular-nums text-sm">{{ fmtNum(request.cacheWriteTokens ?? undefined) }}</span>
          </Field>
        </div>
      </section>

      <section v-if="request.errorMessage" class="flex flex-col gap-2.5">
        <span class="text-2xs font-medium text-ink-muted uppercase tracking-[0.04em]">错误信息</span>
        <pre class="font-mono text-xs whitespace-pre-wrap bg-surface-50 border border-line-soft rounded-md p-3 m-0 text-ink">{{ request.errorMessage }}</pre>
      </section>
    </template>

    <template v-if="error" #error>{{ error }}</template>
  </SidePanel>
</template>
