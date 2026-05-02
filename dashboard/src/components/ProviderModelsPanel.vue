<script setup lang="ts">
import { ref, computed, onMounted, watch } from 'vue'
import { useApi } from '@/composables/useApi'
import AnnotationsEditor from '@/components/AnnotationsEditor.vue'
import type { ProviderView, ProviderModelEntry, ProviderEndpointView } from '@/api'
import {
  SidePanel,
  Button,
  IconButton,
  Input,
  Select,
  Field,
  StateText,
  Tag,
  TagList,
  Icon,
} from '@/ui'

type Row = {
  uid: number
  modelName: string
  upstreamModelName: string
  endpoints: string[]
  priority: number
  annotations: Record<string, string>
  expanded: boolean
}

const props = defineProps<{ providerId: number; providerName: string; onSave?: () => void }>()
const emit = defineEmits<{ close: [] }>()
const api = useApi()

const provider = ref<ProviderView | null>(null)
const providerEndpoints = ref<ProviderEndpointView[]>([])
const rows = ref<Row[]>([])
const loading = ref(true)
const saving = ref(false)
const error = ref('')
const newModelName = ref('')
const fetchEndpointPath = ref('')
const fetching = ref(false)
const fetchSummary = ref<{ added: number; missing: string[] } | null>(null)
const pendingDeletions = ref<Record<string, boolean>>({})

let nextUid = 0

function entryToRow(modelName: string, entry: ProviderModelEntry | undefined): Row {
  return {
    uid: nextUid++,
    modelName,
    upstreamModelName: entry?.upstreamModelName ?? '',
    endpoints: [...(entry?.endpoints ?? [])],
    priority: entry?.priority ?? 0,
    annotations: { ...entry?.annotations },
    expanded: false,
  }
}

function rowsFromProvider(p: ProviderView): Row[] {
  const obj = (p.providerModels ?? {}) as Record<string, ProviderModelEntry>
  return Object.keys(obj)
    .sort()
    .map((name) => entryToRow(name, obj[name]))
}

function rowsToObject(list: Row[]): Record<string, ProviderModelEntry> {
  const out: Record<string, ProviderModelEntry> = {}
  for (const row of list) {
    const name = row.modelName.trim()
    if (!name) continue
    const entry: ProviderModelEntry = {}
    if (row.upstreamModelName.trim()) entry.upstreamModelName = row.upstreamModelName.trim()
    if (row.endpoints.length) entry.endpoints = [...row.endpoints]
    if (row.priority) entry.priority = row.priority
    if (Object.keys(row.annotations).length) entry.annotations = { ...row.annotations }
    out[name] = entry
  }
  return out
}

const modelCount = computed(() => rows.value.length)

const availableEndpointPaths = computed(() => providerEndpoints.value.map((pe) => pe.endpointPath))

async function load() {
  loading.value = true
  error.value = ''
  const [{ data: pData, error: pErr }, { data: peData, error: peErr }] = await Promise.all([
    api.GET('/api/picotera/providers/{id}', { params: { path: { id: props.providerId } } }),
    api.GET('/api/picotera/provider-endpoints', { params: { query: { providerId: props.providerId } } }),
  ])
  loading.value = false
  if (pErr) {
    error.value = pErr.message ?? '加载渠道失败'
    return
  }
  if (peErr) {
    error.value = peErr.message ?? '加载端点失败'
    return
  }
  provider.value = pData as ProviderView
  providerEndpoints.value = (peData as ProviderEndpointView[]) ?? []
  rows.value = rowsFromProvider(provider.value)
  if (!fetchEndpointPath.value && providerEndpoints.value.length) {
    fetchEndpointPath.value = providerEndpoints.value[0]!.endpointPath
  }
}

onMounted(load)
watch(() => props.providerId, load)

function addModel() {
  const name = newModelName.value.trim()
  if (!name) return
  if (rows.value.some((r) => r.modelName === name)) {
    error.value = `模型「${name}」已存在`
    return
  }
  rows.value.unshift(entryToRow(name, undefined))
  newModelName.value = ''
  error.value = ''
}

function removeRow(uid: number) {
  const i = rows.value.findIndex((r) => r.uid === uid)
  if (i >= 0) rows.value.splice(i, 1)
}

function toggleEndpoint(row: Row, path: string) {
  const idx = row.endpoints.indexOf(path)
  if (idx >= 0) row.endpoints.splice(idx, 1)
  else row.endpoints.push(path)
}

async function fetchFromUpstream() {
  if (!fetchEndpointPath.value) {
    error.value = '请选择一个端点作为来源'
    return
  }
  fetching.value = true
  error.value = ''
  fetchSummary.value = null
  pendingDeletions.value = {}
  const { data, error: err } = await api.POST('/api/picotera/provider-endpoints/fetch-models', {
    body: { providerId: props.providerId, endpointPath: fetchEndpointPath.value },
  })
  fetching.value = false
  if (err) {
    error.value = err.message ?? '拉取模型失败'
    return
  }
  const upstream = ((data as { models?: string[] })?.models ?? []).slice().sort()
  const localNames = new Set(rows.value.map((r) => r.modelName))
  const upstreamSet = new Set(upstream)

  let added = 0
  for (const name of upstream) {
    if (!localNames.has(name)) {
      rows.value.push(entryToRow(name, undefined))
      added++
    }
  }
  rows.value.sort((a, b) => a.modelName.localeCompare(b.modelName))

  const missing = rows.value.map((r) => r.modelName).filter((n) => !upstreamSet.has(n))
  fetchSummary.value = { added, missing }
}

function applyDeletions() {
  const toDelete = Object.entries(pendingDeletions.value)
    .filter(([, v]) => v)
    .map(([k]) => k)
  if (!toDelete.length) {
    fetchSummary.value = null
    pendingDeletions.value = {}
    return
  }
  rows.value = rows.value.filter((r) => !toDelete.includes(r.modelName))
  fetchSummary.value = null
  pendingDeletions.value = {}
}

function dismissSummary() {
  fetchSummary.value = null
  pendingDeletions.value = {}
}

async function save() {
  if (!provider.value) return
  saving.value = true
  error.value = ''
  const body = {
    id: provider.value.id,
    name: provider.value.name,
    credentials: provider.value.credentials,
    priority: provider.value.priority,
    providerModels: rowsToObject(rows.value),
    annotations: provider.value.annotations,
  }
  const { error: err } = await api.PUT('/api/picotera/providers', { body })
  saving.value = false
  if (err) {
    error.value = err.message ?? '保存失败'
    return
  }
  props.onSave?.()
  emit('close')
}
</script>

<template>
  <SidePanel
    :title="providerName"
    kicker="模型"
    @close="emit('close')"
  >
    <section v-if="loading" class="flex flex-col gap-2">
      <StateText compact>加载中…</StateText>
    </section>

    <template v-else>
      <section class="flex flex-col gap-2">
        <div class="flex items-baseline justify-between">
          <span class="text-xs font-medium text-ink-muted uppercase tracking-[0.03em]">从上游拉取</span>
        </div>
        <div class="flex gap-2">
          <Select v-model="fetchEndpointPath" size="sm" class="flex-1 min-w-0" :disabled="!providerEndpoints.length">
            <option value="" disabled>
              {{ providerEndpoints.length ? '选择来源端点' : '该渠道暂无已绑定端点' }}
            </option>
            <option v-for="pe in providerEndpoints" :key="pe.endpointPath" :value="pe.endpointPath">
              {{ pe.endpointPath }}
            </option>
          </Select>
          <Button
            size="sm"
            :disabled="fetching || !fetchEndpointPath"
            @click="fetchFromUpstream"
          >
            <Icon :name="fetching ? 'loader' : 'cloud-download'" :size="13" :class="fetching ? 'animate-spin' : ''" />
            <span>{{ fetching ? '拉取中…' : '拉取' }}</span>
          </Button>
        </div>

        <div
          v-if="fetchSummary"
          class="flex flex-col gap-2 px-2.5 py-2 border border-line rounded-md bg-surface-50"
        >
          <div class="text-xs text-ink">
            新增 {{ fetchSummary.added }} 项<span v-if="fetchSummary.missing.length">，本地有但上游缺失 {{ fetchSummary.missing.length }} 项</span>
          </div>
          <div v-if="fetchSummary.missing.length" class="flex flex-col gap-1.5">
            <div class="text-2xs text-ink-muted">勾选要删除的模型：</div>
            <ul class="list-none m-0 p-0 flex flex-col gap-1 max-h-40 overflow-y-auto">
              <li
                v-for="name in fetchSummary.missing"
                :key="name"
                class="flex items-center gap-2 text-xs"
              >
                <input
                  :id="`del-${name}`"
                  v-model="pendingDeletions[name]"
                  type="checkbox"
                  class="cursor-pointer"
                />
                <label :for="`del-${name}`" class="font-mono text-ink cursor-pointer">{{ name }}</label>
              </li>
            </ul>
            <div class="flex gap-2 justify-end">
              <Button variant="ghost" size="sm" @click="dismissSummary">忽略</Button>
              <Button variant="danger" size="sm" @click="applyDeletions">确认删除</Button>
            </div>
          </div>
          <div v-else class="flex justify-end">
            <Button variant="ghost" size="sm" @click="dismissSummary">关闭</Button>
          </div>
        </div>
      </section>

      <section class="flex flex-col gap-2">
        <div class="flex items-baseline justify-between">
          <span class="text-xs font-medium text-ink-muted uppercase tracking-[0.03em]">模型列表</span>
          <span class="text-xs text-ink-faint tabular-nums">{{ modelCount }}</span>
        </div>
        <form class="flex gap-2" @submit.prevent="addModel">
          <Input
            v-model="newModelName"
            size="sm"
            class="flex-1 min-w-0"
            placeholder="新增模型名（picotera 内部模型名）"
          />
          <Button type="submit" size="sm" :disabled="!newModelName.trim()">
            <Icon name="plus" :size="13" />
            <span>添加</span>
          </Button>
        </form>
        <StateText v-if="!rows.length" compact>暂无模型，添加或从上游拉取</StateText>
        <ul v-else class="list-none m-0 p-0 flex flex-col gap-2">
          <li
            v-for="row in rows"
            :key="row.uid"
            class="flex flex-col gap-2 px-2.5 py-2 border border-line rounded-md bg-surface-0"
          >
            <div class="flex items-center gap-2">
              <button
                type="button"
                class="flex-1 min-w-0 flex items-center gap-1.5 text-left bg-transparent border-0 cursor-pointer p-0"
                @click="row.expanded = !row.expanded"
              >
                <Icon :name="row.expanded ? 'chevron-down' : 'chevron-down'" :size="12" :class="row.expanded ? '' : '-rotate-90'" />
                <span class="font-mono text-sm text-ink overflow-hidden text-ellipsis whitespace-nowrap">{{ row.modelName }}</span>
              </button>
              <TagList v-if="!row.expanded">
                <Tag v-if="row.upstreamModelName" variant="accent">→ {{ row.upstreamModelName }}</Tag>
                <Tag v-if="row.priority" variant="more">P{{ row.priority }}</Tag>
                <Tag v-if="row.endpoints.length" variant="more">{{ row.endpoints.length }} 端点</Tag>
              </TagList>
              <IconButton
                variant="danger"
                title="删除"
                :aria-label="`删除模型 ${row.modelName}`"
                @click="removeRow(row.uid)"
              >
                <Icon name="trash" :size="13" />
              </IconButton>
            </div>
            <div v-if="row.expanded" class="flex flex-col gap-3 pl-4">
              <Field label="上游模型名（可选）">
                <Input v-model="row.upstreamModelName" size="sm" placeholder="保留为空 = 与本地名一致" />
              </Field>
              <Field label="优先级">
                <Input v-model.number="row.priority" type="number" size="sm" />
              </Field>
              <Field label="端点（不勾选 = 全部已绑定端点）" as="div">
                <div v-if="!availableEndpointPaths.length" class="text-2xs text-ink-faint">
                  渠道尚未绑定任何端点
                </div>
                <ul v-else class="list-none m-0 p-0 flex flex-col gap-1">
                  <li
                    v-for="path in availableEndpointPaths"
                    :key="path"
                    class="flex items-center gap-2 text-xs"
                  >
                    <input
                      :id="`ep-${row.uid}-${path}`"
                      type="checkbox"
                      class="cursor-pointer"
                      :checked="row.endpoints.includes(path)"
                      @change="toggleEndpoint(row, path)"
                    />
                    <label :for="`ep-${row.uid}-${path}`" class="font-mono text-ink cursor-pointer">{{ path }}</label>
                  </li>
                </ul>
              </Field>
              <Field label="标注" as="div">
                <AnnotationsEditor v-model="row.annotations" />
              </Field>
            </div>
          </li>
        </ul>
      </section>
    </template>

    <template v-if="error" #error>{{ error }}</template>

    <template #footer>
      <Button variant="ghost" @click="emit('close')">取消</Button>
      <Button :disabled="saving || loading" @click="save">
        {{ saving ? '保存中…' : '保存' }}
      </Button>
    </template>
  </SidePanel>
</template>
