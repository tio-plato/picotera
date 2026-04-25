<script setup lang="ts">
import { ref, computed, watch } from 'vue'
import { DataTable, Th, Td, Tr, Field, SegmentedControl, StateText } from '@/ui'
import {
  aggregateSSE,
  extractContent,
  isSSEContentType,
  renderMarkdown,
  type SSEFormat,
} from '@/composables/useSSEParser'

interface ArtifactPayload {
  method?: string
  url?: string
  statusCode?: number
  headers?: Record<string, string[]>
  body?: string
  bodyEncoding?: 'utf8' | 'base64'
}

const props = defineProps<{ payload: ArtifactPayload; url?: string }>()

type SubView = 'raw' | 'aggregated' | 'rendered'
const subView = ref<SubView>('raw')

const isSSE = computed(() => isSSEContentType(props.payload.headers))
const isBinary = computed(() => props.payload.bodyEncoding === 'base64')

const subViewOptions = computed(() => {
  const opts: Array<{ value: string; label: string }> = [
    { value: 'raw', label: 'Raw' },
  ]
  if (isSSE.value && !isBinary.value) {
    opts.push({ value: 'aggregated', label: '聚合' })
  }
  if (!isBinary.value) {
    opts.push({ value: 'rendered', label: '渲染' })
  }
  return opts
})

const aggregated = computed(() => {
  if (!isSSE.value || !props.payload.body) return null
  return aggregateSSE(props.payload.body)
})

const content = computed(() => {
  if (!props.payload.body) return { thinking: null, reply: null }
  return extractContent(props.payload.body, isSSE.value)
})

const replyHtml = computed(() => {
  if (!content.value.reply) return ''
  return renderMarkdown(content.value.reply)
})

const thinkingHtml = computed(() => {
  if (!content.value.thinking) return ''
  return renderMarkdown(content.value.thinking)
})

function headerEntries(headers: Record<string, string[]> | undefined) {
  if (!headers) return []
  return Object.entries(headers).map(([k, v]) => ({ key: k, value: v.join(', ') }))
}

function bodyDisplay(body: string | undefined, encoding: string | undefined) {
  if (!body) return ''
  if (encoding === 'base64') return ''
  try {
    return JSON.stringify(JSON.parse(body), null, 2)
  } catch {
    return body
  }
}

function formatLabel(f: SSEFormat | null): string {
  switch (f) {
    case 'openai-chat': return 'OpenAI Chat'
    case 'anthropic': return 'Anthropic'
    case 'openai-responses': return 'OpenAI Responses'
    default: return ''
  }
}

watch(subViewOptions, (opts) => {
  if (!opts.some(o => o.value === subView.value)) {
    subView.value = 'raw'
  }
})
</script>

<template>
  <div class="flex flex-col gap-3">
    <div class="grid grid-cols-2 gap-2.5">
      <Field v-if="payload.statusCode" label="Status" as="div">
        <span class="font-mono text-sm">{{ payload.statusCode }}</span>
      </Field>
    </div>

    <section class="flex flex-col gap-2">
      <span class="text-2xs font-medium text-ink-muted uppercase tracking-[0.04em]">Headers</span>
      <div v-if="!headerEntries(payload.headers).length" class="text-xs text-ink-faint">—</div>
      <DataTable v-else>
        <thead>
          <Tr>
            <Th class="w-44">Header</Th>
            <Th>Value</Th>
          </Tr>
        </thead>
        <tbody>
          <Tr v-for="h in headerEntries(payload.headers)" :key="h.key">
            <Td class="font-mono text-2xs whitespace-nowrap">{{ h.key }}</Td>
            <Td class="font-mono text-2xs break-all">{{ h.value }}</Td>
          </Tr>
        </tbody>
      </DataTable>
    </section>

    <section class="flex flex-col gap-2">
      <div class="flex items-center justify-between gap-3">
        <span class="text-2xs font-medium text-ink-muted uppercase tracking-[0.04em]">Body</span>
        <SegmentedControl
          v-if="!isBinary"
          v-model="subView"
          :options="subViewOptions"
        />
      </div>

      <div v-if="isBinary" class="flex items-center gap-3 text-xs text-ink-faint">
        <span>[binary, {{ payload.body?.length ?? 0 }} bytes]</span>
        <a :href="url" download class="text-accent-ink underline hover:no-underline">下载原始数据</a>
      </div>

      <!-- Raw -->
      <pre
        v-else-if="subView === 'raw'"
        class="font-mono text-xs whitespace-pre-wrap break-all bg-surface-50 border border-line-soft rounded-md p-3 m-0 text-ink overflow-auto max-h-[480px]"
      >{{ bodyDisplay(payload.body, payload.bodyEncoding) }}</pre>

      <!-- Aggregated -->
      <template v-else-if="subView === 'aggregated'">
        <div v-if="aggregated?.json" class="flex flex-col gap-1.5">
          <span v-if="formatLabel(aggregated.format)" class="text-2xs text-ink-muted">
            检测格式: {{ formatLabel(aggregated.format) }}
          </span>
          <pre class="font-mono text-xs whitespace-pre-wrap break-all bg-surface-50 border border-line-soft rounded-md p-3 m-0 text-ink overflow-auto max-h-[480px]">{{ JSON.stringify(aggregated.json, null, 2) }}</pre>
        </div>
        <StateText v-else :dashed="false" compact>无法聚合 SSE 数据</StateText>
      </template>

      <!-- Rendered -->
      <template v-else-if="subView === 'rendered'">
        <div class="flex flex-col gap-3">
          <details v-if="content.thinking" class="group">
            <summary class="flex items-center gap-1.5 cursor-pointer text-xs font-medium text-ink-muted select-none hover:text-ink">
              <svg class="w-3 h-3 transition-transform group-open:rotate-90" viewBox="0 0 16 16" fill="currentColor"><path d="M6 3.5l5 4.5-5 4.5V3.5z"/></svg>
              思考过程
            </summary>
            <div class="mt-2 bg-surface-50 border border-line-soft rounded-md p-3 prose prose-sm max-w-none" v-html="thinkingHtml" />
          </details>
          <div v-if="content.reply" class="prose prose-sm max-w-none" v-html="replyHtml" />
          <StateText v-else-if="!content.thinking" :dashed="false" compact>无内容</StateText>
        </div>
      </template>
    </section>
  </div>
</template>
