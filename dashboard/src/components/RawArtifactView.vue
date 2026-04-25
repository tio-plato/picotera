<script setup lang="ts">
import { ref, watch } from 'vue'
import { StateText, DataTable, Th, Td, Tr, Field } from '@/ui'

interface ArtifactPayload {
  method?: string
  url?: string
  statusCode?: number
  headers?: Record<string, string[]>
  body?: string
  bodyEncoding?: 'utf8' | 'base64'
}

const props = defineProps<{ url?: string; kind: 'request' | 'response' }>()

const loading = ref(false)
const error = ref('')
const payload = ref<ArtifactPayload | null>(null)

async function load() {
  payload.value = null
  error.value = ''
  if (!props.url) return
  loading.value = true
  try {
    const res = await fetch(props.url)
    if (!res.ok) {
      if (res.status === 404) {
        error.value = 'artifact 不可用'
      } else {
        error.value = `加载失败 (${res.status})`
      }
      return
    }
    payload.value = await res.json()
  } catch (e) {
    error.value = e instanceof Error ? e.message : '加载失败'
  } finally {
    loading.value = false
  }
}

watch(() => props.url, load, { immediate: true })

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
</script>

<template>
  <StateText v-if="!url" :dashed="false" compact>未启用 artifact 记录</StateText>
  <StateText v-else-if="loading" :dashed="false" compact>加载中…</StateText>
  <StateText v-else-if="error" :dashed="false" compact>{{ error }}</StateText>
  <div v-else-if="payload" class="flex flex-col gap-3">
    <div class="grid grid-cols-2 gap-2.5">
      <Field v-if="kind === 'request' && payload.method" label="Method" as="div">
        <span class="font-mono text-sm">{{ payload.method }}</span>
      </Field>
      <Field v-if="kind === 'request' && payload.url" label="URL" as="div" class="col-span-2">
        <span class="font-mono text-xs break-all">{{ payload.url }}</span>
      </Field>
      <Field v-if="kind === 'response' && payload.statusCode" label="Status" as="div">
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
      <span class="text-2xs font-medium text-ink-muted uppercase tracking-[0.04em]">Body</span>
      <div v-if="payload.bodyEncoding === 'base64'" class="flex items-center gap-3 text-xs text-ink-faint">
        <span>[binary, {{ payload.body?.length ?? 0 }} bytes]</span>
        <a :href="url" download class="text-accent-ink underline hover:no-underline">下载原始数据</a>
      </div>
      <pre
        v-else
        class="font-mono text-xs whitespace-pre-wrap break-all bg-surface-50 border border-line-soft rounded-md p-3 m-0 text-ink overflow-auto max-h-[480px]"
      >{{ bodyDisplay(payload.body, payload.bodyEncoding) }}</pre>
    </section>
  </div>
</template>
