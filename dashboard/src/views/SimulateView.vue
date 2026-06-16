<script setup lang="ts">
import { computed, ref } from 'vue'
import { useMutation, useQuery } from '@tanstack/vue-query'
import { Button, Field, Input, SegmentedControl, Select, Textarea, Icon } from '@/ui'
import { useSidePanel } from '@/composables/useSidePanel'
import { listApiKeys, listEndpoints, listModels, simulateDispatch } from '@/api/client'
import { queryKeys } from '@/api/queryKeys'
import type { SimulateDispatchRequestBody } from '@/api'
import SimulateResultPanel from '@/components/SimulateResultPanel.vue'

type Kind = 'path' | 'unified'

const UNIFIED_FORMATS = [
  { value: 'anthropicMessages', label: 'Anthropic Messages' },
  { value: 'openaiChatCompletions', label: 'OpenAI Chat Completions' },
  { value: 'openaiResponses', label: 'OpenAI Responses' },
  { value: 'geminiGenerateContent', label: 'Gemini generateContent' },
  { value: 'geminiStreamGenerateContent', label: 'Gemini streamGenerateContent' },
] as const

const kindOptions = [
  { value: 'path', label: '配置端点' },
  { value: 'unified', label: '统一路由' },
]

const endpointsQuery = useQuery({ queryKey: queryKeys.endpoints.all, queryFn: listEndpoints })
const apiKeysQuery = useQuery({ queryKey: queryKeys.apiKeys.all, queryFn: listApiKeys })
const modelsQuery = useQuery({ queryKey: queryKeys.models.all, queryFn: listModels })

const kind = ref<Kind>('path')
const selectedPath = ref('')
const selectedFormat = ref<(typeof UNIFIED_FORMATS)[number]['value']>('anthropicMessages')
const apiKeyId = ref(0)
const model = ref('')
const bodyText = ref('')
const pathVars = ref<Record<string, string>>({})

const bodyError = ref('')

const endpoints = computed(() => endpointsQuery.data.value ?? [])
const apiKeys = computed(() => apiKeysQuery.data.value ?? [])
const modelNames = computed(() => (modelsQuery.data.value ?? []).map((m) => m.name))

const modelOptions = computed(() => [
  { value: '', label: '请选择模型', disabled: true },
  ...modelNames.value.map((name) => ({ value: name, label: name })),
])

const pathOptions = computed(() => [
  { value: '', label: '请选择端点', disabled: true },
  ...endpoints.value.map((e) => ({
    value: e.path,
    label: e.path + (e.name ? ` — ${e.name}` : ''),
  })),
])

const apiKeyOptions = computed(() => [
  { value: 0, label: '请选择 API Key', disabled: true },
  ...apiKeys.value.map((k) => ({
    value: k.id,
    label: `${k.name} (#${k.id})${k.disabled ? ' — 已禁用' : ''}`,
  })),
])

const selectedEndpoint = computed(
  () => endpoints.value.find((e) => e.path === selectedPath.value) ?? null,
)

const pathVarNames = computed(() => {
  if (kind.value !== 'path') return []
  const path = selectedEndpoint.value?.path ?? ''
  const names: string[] = []
  const re = /\{([A-Za-z_][A-Za-z0-9_]*)\}/g
  let m: RegExpExecArray | null
  while ((m = re.exec(path)) !== null) {
    if (m[1]) names.push(m[1])
  }
  return names
})

function setPathVar(name: string, value: string) {
  pathVars.value = { ...pathVars.value, [name]: value }
}

function formatBody() {
  if (!bodyText.value.trim()) return
  try {
    bodyText.value = JSON.stringify(JSON.parse(bodyText.value), null, 2)
    bodyError.value = ''
  } catch (e) {
    bodyError.value = e instanceof Error ? e.message : 'JSON 解析失败'
  }
}

const panel = useSidePanel()

const simulate = useMutation({
  mutationFn: (body: SimulateDispatchRequestBody) => simulateDispatch(body),
  onSuccess: (data) => {
    panel.open(SimulateResultPanel, { result: data }, { key: 'simulate:result', width: '640px' })
  },
})

const errorMessage = computed(() => {
  if (bodyError.value) return bodyError.value
  const err = simulate.error.value
  if (!err) return ''
  return err instanceof Error ? err.message : '模拟失败'
})

function canSubmit() {
  if (!apiKeyId.value) return false
  if (!model.value.trim()) return false
  if (kind.value === 'path' && !selectedPath.value) return false
  if (kind.value === 'unified' && !selectedFormat.value) return false
  return true
}

async function submit() {
  bodyError.value = ''
  let normalized = ''
  if (bodyText.value.trim() !== '') {
    try {
      normalized = JSON.stringify(JSON.parse(bodyText.value))
    } catch (e) {
      bodyError.value = e instanceof Error ? e.message : '请求体不是合法 JSON'
      return
    }
  }
  const payload: SimulateDispatchRequestBody = {
    endpoint:
      kind.value === 'path'
        ? { kind: 'path', path: selectedPath.value }
        : { kind: 'unified', format: selectedFormat.value },
    apiKeyId: apiKeyId.value,
    model: model.value.trim(),
    body: normalized,
  }
  if (kind.value === 'path' && pathVarNames.value.length > 0) {
    const used: Record<string, string> = {}
    for (const n of pathVarNames.value) {
      const v = pathVars.value[n]
      if (v) used[n] = v
    }
    if (Object.keys(used).length > 0) payload.pathVars = used
  }
  await simulate.mutateAsync(payload)
}
</script>

<template>
  <form class="flex flex-col gap-4" @submit.prevent="submit">
    <Field label="端点种类" as="div">
      <SegmentedControl v-model="kind" :options="kindOptions" />
    </Field>

    <Field v-if="kind === 'path'" label="端点路径">
      <Select v-model="selectedPath" :options="pathOptions" />
    </Field>
    <Field v-else label="统一路由格式">
      <Select v-model="selectedFormat" :options="UNIFIED_FORMATS" />
    </Field>

    <Field v-if="kind === 'path' && pathVarNames.length > 0" label="路径变量" as="div">
      <div class="flex flex-col gap-2">
        <label v-for="name in pathVarNames" :key="name" class="flex items-center gap-2 text-xs">
          <span class="font-mono text-ink-muted shrink-0 w-32 truncate">{{ name }}</span>
          <Input
            :model-value="pathVars[name] ?? ''"
            @update:model-value="(v: string | number) => setPathVar(name, String(v))"
          />
        </label>
      </div>
    </Field>

    <Field label="API Key">
      <Select v-model="apiKeyId" :options="apiKeyOptions" />
    </Field>

    <Field label="模型">
      <Select v-model="model" :options="modelOptions" />
    </Field>

    <Field label="请求体" :error="bodyError">
      <div class="flex flex-col gap-1.5">
        <Textarea v-model="bodyText" mono rows="12" placeholder='{"messages":[...]}' />
        <div class="flex">
          <Button type="button" variant="ghost" size="sm" @click="formatBody">格式化</Button>
        </div>
      </div>
    </Field>

    <div
      v-if="errorMessage"
      class="border border-err/30 bg-err/5 text-err text-sm rounded-md px-3 py-2 font-mono break-all"
    >
      {{ errorMessage }}
    </div>

    <div class="flex justify-end">
      <Button type="submit" :disabled="!canSubmit() || simulate.isPending.value">
        <Icon name="bolt" :size="14" :stroke-width="2.2" />
        <span>{{ simulate.isPending.value ? '模拟中…' : '模拟' }}</span>
      </Button>
    </div>
  </form>
</template>
