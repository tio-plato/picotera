<script setup lang="ts">
import { ref, computed, watch } from 'vue'
import { DataTable, Th, Td, Tr, Field, SegmentedControl, StateText } from '@/ui'
import {
  extractContentFromAggregated,
  formatAggregatedLabel,
  isSSEContentType,
  parseSSEEventsForDisplay,
  renderMarkdown,
} from '@/composables/useSSEParser'
import { isJsonContentType, parseJsonBody, rawBodyText } from './artifactBody'
import type { ArtifactPayload } from './artifactTypes'
import JsonArtifactViewer from './JsonArtifactViewer.vue'
import SSEEventsVirtualList from './SSEEventsVirtualList.vue'

const props = defineProps<{ payload: ArtifactPayload; url?: string }>()

type SubView = 'raw' | 'json' | 'aggregated' | 'rendered' | 'events'
const subView = ref<SubView>('raw')

const isSSE = computed(() => isSSEContentType(props.payload.headers))
const isBinary = computed(() => props.payload.bodyEncoding === 'base64')
const jsonBody = computed(() => {
  if (isBinary.value || !isJsonContentType(props.payload.headers)) {
    return { ok: false, value: null, error: '' }
  }
  return parseJsonBody(props.payload.body, props.payload.bodyEncoding)
})

const subViewOptions = computed(() => {
  const opts: Array<{ value: string; label: string }> = [{ value: 'raw', label: 'Raw' }]
  if (!isBinary.value && (isSSE.value || props.payload.aggregated)) {
    opts.push({ value: 'aggregated', label: '聚合' })
  }
  if (isSSE.value && !isBinary.value) {
    opts.push({ value: 'events', label: 'Events' })
  } else if (!isBinary.value && jsonBody.value.ok) {
    opts.push({ value: 'json', label: 'JSON' })
  }
  if (!isBinary.value) {
    opts.push({ value: 'rendered', label: '渲染' })
  }
  return opts
})

const sseEvents = computed(() => {
  if (!isSSE.value || !props.payload.body) return []
  return parseSSEEventsForDisplay(props.payload.body)
})

const content = computed(() => {
  return extractContentFromAggregated(props.payload.aggregated)
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
  return rawBodyText(body, encoding)
}

watch(
  subViewOptions,
  (opts) => {
    if (!opts.some((o) => o.value === subView.value)) {
      subView.value = opts.some((o) => o.value === 'json') ? 'json' : 'raw'
    }
  },
  { immediate: true },
)

watch(jsonBody, (parsed) => {
  if (!isSSE.value && parsed.ok) subView.value = 'json'
})
</script>

<template>
  <div class="flex flex-col gap-3">
    <div class="grid grid-cols-2 gap-2.5">
      <Field v-if="payload.statusCode" label="Status" as="div">
        <span class="font-mono text-sm">{{ payload.statusCode }}</span>
      </Field>
    </div>

    <details class="group flex flex-col gap-2">
      <summary
        class="flex items-center gap-1.5 cursor-pointer select-none list-none text-2xs font-medium text-ink-muted uppercase tracking-[0.04em] hover:text-ink"
      >
        Headers
        <span
          v-if="headerEntries(payload.headers).length"
          class="text-ink-faint normal-case tracking-normal"
          >({{ headerEntries(payload.headers).length }})</span
        >
        <svg
          class="w-3 h-3 transition-transform group-open:rotate-90"
          viewBox="0 0 16 16"
          fill="currentColor"
        >
          <path d="M6 3.5l5 4.5-5 4.5V3.5z" />
        </svg>
      </summary>
      <div class="mt-2">
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
      </div>
    </details>

    <section class="flex flex-col gap-2">
      <div class="flex items-center justify-between gap-3">
        <span class="text-2xs font-medium text-ink-muted uppercase tracking-[0.04em]">Body</span>
        <SegmentedControl v-if="!isBinary" v-model="subView" :options="subViewOptions" />
      </div>

      <div v-if="isBinary" class="flex items-center gap-3 text-xs text-ink-faint">
        <span>[binary, {{ payload.body?.length ?? 0 }} bytes]</span>
        <a :href="url" download class="text-accent-ink underline hover:no-underline"
          >下载原始数据</a
        >
      </div>

      <!-- Raw -->
      <template v-else-if="subView === 'raw'">
        <StateText
          v-if="isJsonContentType(payload.headers) && !jsonBody.ok"
          :dashed="false"
          compact
          class="mb-2"
        >
          {{ jsonBody.error }}
        </StateText>
        <pre
          class="font-mono text-xs whitespace-pre-wrap break-all bg-surface-50 border border-line-soft rounded-md p-3 m-0 text-ink overflow-auto max-h-[480px]"
          >{{ bodyDisplay(payload.body, payload.bodyEncoding) }}</pre
        >
      </template>

      <!-- JSON -->
      <template v-else-if="subView === 'json'">
        <JsonArtifactViewer v-if="jsonBody.ok" :value="jsonBody.value" />
        <StateText v-else :dashed="false" compact>{{ jsonBody.error }}</StateText>
      </template>

      <!-- Aggregated -->
      <template v-else-if="subView === 'aggregated'">
        <div v-if="payload.aggregated?.body !== undefined" class="flex flex-col gap-1.5">
          <span
            v-if="formatAggregatedLabel(payload.aggregated.format)"
            class="text-2xs text-ink-muted"
          >
            后端格式: {{ formatAggregatedLabel(payload.aggregated.format) }}
          </span>
          <JsonArtifactViewer :value="payload.aggregated.body" />
        </div>
        <StateText v-else-if="payload.aggregated?.error" :dashed="false" compact>
          {{ payload.aggregated.error }}
        </StateText>
        <StateText v-else :dashed="false" compact>无后端聚合结果</StateText>
      </template>

      <!-- Events -->
      <template v-else-if="subView === 'events'">
        <StateText v-if="!sseEvents.length" :dashed="false" compact>没有可解析 event</StateText>
        <SSEEventsVirtualList v-else :events="sseEvents" />
      </template>

      <!-- Rendered -->
      <template v-else-if="subView === 'rendered'">
        <div class="flex flex-col gap-3">
          <details v-if="content.thinking" class="group">
            <summary
              class="flex items-center gap-1.5 cursor-pointer text-xs font-medium text-ink-muted select-none hover:text-ink"
            >
              <svg
                class="w-3 h-3 transition-transform group-open:rotate-90"
                viewBox="0 0 16 16"
                fill="currentColor"
              >
                <path d="M6 3.5l5 4.5-5 4.5V3.5z" />
              </svg>
              思考过程
            </summary>
            <div
              class="mt-2 bg-surface-50 border border-line-soft rounded-md p-3 prose prose-sm max-w-none"
              v-html="thinkingHtml"
            />
          </details>
          <div v-if="content.reply" class="prose prose-sm max-w-none" v-html="replyHtml" />
          <StateText v-else-if="!content.thinking" :dashed="false" compact>无可渲染内容</StateText>
        </div>
      </template>
    </section>
  </div>
</template>
