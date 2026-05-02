<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import { useConfirm } from '@/composables/useConfirm'
import { useApi } from '@/composables/useApi'
import type {
  ModelView,
  ProviderView,
  ProviderModelEntry,
  ProviderEndpointView,
} from '@/api'
import ModelForm from '@/components/ModelForm.vue'
import ModelUpstreamsPanel, { type Upstream } from '@/components/ModelUpstreamsPanel.vue'
import { useSidePanel } from '@/composables/useSidePanel'
import {
  Button,
  IconButton,
  DataCard,
  DataTable,
  Th,
  Td,
  Tr,
  StateText,
  Tag,
  TagList,
  Icon,
} from '@/ui'

const panel = useSidePanel()
const confirm = useConfirm()
const api = useApi()

const models = ref<ModelView[]>([])
const providers = ref<ProviderView[]>([])
const providerEndpoints = ref<ProviderEndpointView[]>([])
const loading = ref(true)
const orphanExpanded = ref(false)

async function fetchAll() {
  loading.value = true
  const [m, p, pe] = await Promise.all([
    api.GET('/api/picotera/models'),
    api.GET('/api/picotera/providers'),
    api.GET('/api/picotera/provider-endpoints'),
  ])
  if (!m.error && m.data) models.value = m.data as ModelView[]
  if (!p.error && p.data) providers.value = p.data as ProviderView[]
  if (!pe.error && pe.data) providerEndpoints.value = pe.data as ProviderEndpointView[]
  loading.value = false
}

onMounted(fetchAll)

const providerEndpointMap = computed(() => {
  const map: Record<number, string[]> = {}
  for (const pe of providerEndpoints.value) {
    ;(map[pe.providerId] ??= []).push(pe.endpointPath)
  }
  for (const arr of Object.values(map)) arr.sort()
  return map
})

const upstreamIndex = computed<Record<string, Upstream[]>>(() => {
  const out: Record<string, Upstream[]> = {}
  for (const provider of providers.value) {
    const obj = (provider.providerModels ?? {}) as Record<string, ProviderModelEntry>
    for (const [modelName, entry] of Object.entries(obj)) {
      const entryEndpoints = entry?.endpoints ?? []
      const expandedFromProvider = !entryEndpoints.length
      const endpointPaths = expandedFromProvider
        ? providerEndpointMap.value[provider.id] ?? []
        : [...entryEndpoints]
      const upstream: Upstream = {
        providerId: provider.id,
        providerName: provider.name,
        upstreamModelName: entry?.upstreamModelName?.trim() || modelName,
        endpointPaths,
        priority: entry?.priority ?? 0,
        expandedFromProvider,
        providerDisabled: provider.disabled ?? false,
        entryDisabled: entry?.disabled ?? false,
      }
      ;(out[modelName] ??= []).push(upstream)
    }
  }
  return out
})

const registeredNames = computed(() => new Set(models.value.map((m) => m.name)))

const orphanRows = computed<{ name: string; providerNames: string[] }[]>(() => {
  const out: { name: string; providerNames: string[] }[] = []
  for (const [name, list] of Object.entries(upstreamIndex.value)) {
    if (registeredNames.value.has(name)) continue
    out.push({ name, providerNames: list.map((u) => u.providerName) })
  }
  out.sort((a, b) => a.name.localeCompare(b.name))
  return out
})

const count = computed(() => models.value.length)

function openCreate() {
  panel.open(ModelForm, { onSave: fetchAll }, { key: 'model:new' })
}

function openEdit(m: ModelView) {
  panel.open(ModelForm, { model: m, onSave: fetchAll }, { key: `model:${m.name}` })
}

function openUpstreams(m: ModelView) {
  const list = upstreamIndex.value[m.name] ?? []
  if (!list.length) return
  panel.open(
    ModelUpstreamsPanel,
    { modelName: m.name, modelDisabled: m.disabled ?? false, upstreams: list },
    { key: `model-upstreams:${m.name}` },
  )
}

async function toggleDisabled(m: ModelView) {
  const body = {
    name: m.name,
    title: m.title,
    developer: m.developer,
    series: m.series,
    disabled: !m.disabled,
  }
  const { error } = await api.PUT('/api/picotera/models', { body })
  if (!error) fetchAll()
}

function openCreateFromOrphan(name: string) {
  panel.open(
    ModelForm,
    { defaultName: name, lockedName: true, onSave: fetchAll },
    { key: `model:new:${name}` },
  )
}

function confirmDelete(_event: Event, m: ModelView) {
  confirm.require({
    message: `确定要删除模型「${m.name}」吗？此操作不可撤销。`,
    accept: async () => {
      await api.POST('/api/picotera/models/delete', { body: { name: m.name } })
      fetchAll()
    },
  })
}
</script>

<template>
  <div class="flex flex-col gap-3.5">
    <div class="flex items-center justify-between gap-3">
      <span class="text-xs text-ink-faint tabular-nums">{{ count }} 个模型</span>
      <div class="flex items-center gap-2">
        <Button @click="openCreate">
          <Icon name="plus" :size="14" :stroke-width="2.2" />
          <span>新增模型</span>
        </Button>
      </div>
    </div>
    <StateText v-if="loading">加载中…</StateText>
    <template v-else>
      <DataCard v-if="models.length">
        <DataTable>
          <thead>
            <tr>
              <Th>名称</Th>
              <Th>标题</Th>
              <Th>开发者</Th>
              <Th>系列</Th>
              <Th>上游</Th>
              <Th actions />
            </tr>
          </thead>
          <tbody>
            <Tr v-for="m in models" :key="m.name" :selected="panel.isActive(`model:${m.name}`)" :class="m.disabled ? 'opacity-55' : ''">
              <Td>
	                <span class="font-mono font-medium">{{ m.name }}</span>
	                <span v-if="m.disabled" class="text-ink-faint ml-1.5">（已禁用）</span>
	              </Td>
              <Td>{{ m.title }}</Td>
              <Td><span class="text-ink-faint">{{ m.developer }}</span></Td>
              <Td><Tag>{{ m.series }}</Tag></Td>
              <Td>
                <span
                  v-if="(upstreamIndex[m.name]?.length ?? 0) > 0"
                  class="font-medium tabular-nums text-ink"
                >{{ upstreamIndex[m.name]!.length }}</span>
                <span v-else class="text-ink-faint">—</span>
              </Td>
              <Td actions>
                <div class="inline-flex gap-1 opacity-55 group-hover:opacity-100 transition-opacity">
                  <IconButton
                    :title="m.disabled ? '启用模型' : '禁用模型'"
                    :aria-label="m.disabled ? '启用模型' : '禁用模型'"
                    @click="toggleDisabled(m)"
                  >
                    <Icon :name="m.disabled ? 'eye-off' : 'eye'" :size="13" />
                  </IconButton>
                  <IconButton
                    :active="panel.isActive(`model-upstreams:${m.name}`)"
                    :disabled="(upstreamIndex[m.name]?.length ?? 0) === 0"
                    :title="(upstreamIndex[m.name]?.length ?? 0) === 0 ? '无上游' : '查看上游'"
                    aria-label="查看上游"
                    @click="openUpstreams(m)"
                  >
                    <Icon name="cloud-upload" :size="13" />
                  </IconButton>
                  <IconButton
                    :active="panel.isActive(`model:${m.name}`)"
                    title="编辑"
                    aria-label="编辑"
                    @click="openEdit(m)"
                  >
                    <Icon name="edit" :size="13" />
                  </IconButton>
                  <IconButton
                    variant="danger"
                    title="删除"
                    aria-label="删除"
                    @click="(e: Event) => confirmDelete(e, m)"
                  >
                    <Icon name="trash" :size="13" />
                  </IconButton>
                </div>
              </Td>
            </Tr>
          </tbody>
        </DataTable>
      </DataCard>
      <StateText v-else>暂无模型，点击右上角按钮新增</StateText>

      <section v-if="orphanRows.length" class="flex flex-col gap-2">
        <button
          type="button"
          class="flex items-center gap-1.5 text-left bg-transparent border-0 cursor-pointer p-0 text-xs font-medium text-ink-muted uppercase tracking-[0.03em]"
          @click="orphanExpanded = !orphanExpanded"
        >
          <Icon
            name="chevron-down"
            :size="12"
            :class="orphanExpanded ? '' : '-rotate-90'"
          />
          <span>未注册上游模型 ({{ orphanRows.length }})</span>
        </button>
        <DataCard v-if="orphanExpanded">
          <ul class="list-none m-0 p-0 flex flex-col">
            <li
              v-for="row in orphanRows"
              :key="row.name"
              class="group flex items-center gap-2 px-3 py-2 border-b border-line last:border-b-0"
            >
              <span class="font-mono text-sm text-ink flex-none">{{ row.name }}</span>
              <TagList class="flex-1 min-w-0">
                <Tag v-for="p in row.providerNames" :key="p">{{ p }}</Tag>
              </TagList>
              <IconButton
                :active="panel.isActive(`model:new:${row.name}`)"
                title="添加为模型"
                :aria-label="`添加 ${row.name} 为模型`"
                class="opacity-55 group-hover:opacity-100 transition-opacity"
                @click="openCreateFromOrphan(row.name)"
              >
                <Icon name="plus" :size="13" />
              </IconButton>
            </li>
          </ul>
        </DataCard>
      </section>
    </template>
  </div>
</template>
