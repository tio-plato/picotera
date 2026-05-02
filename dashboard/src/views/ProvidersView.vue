<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import { useConfirm } from '@/composables/useConfirm'
import { useApi } from '@/composables/useApi'
import type { ProviderView } from '@/api'
import ProviderForm from '@/components/ProviderForm.vue'
import ProviderEndpointsPanel from '@/components/ProviderEndpointsPanel.vue'
import ProviderModelsPanel from '@/components/ProviderModelsPanel.vue'
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
  Badge,
  Tag,
  TagList,
  Icon,
} from '@/ui'

const panel = useSidePanel()
const confirm = useConfirm()
const api = useApi()

const providers = ref<ProviderView[]>([])
const loading = ref(true)
const count = computed(() => providers.value.length)

async function fetchProviders() {
  loading.value = true
  const { data, error } = await api.GET('/api/picotera/providers')
  if (!error && data) providers.value = data as ProviderView[]
  loading.value = false
}

onMounted(fetchProviders)

function editKey(id: number) {
  return `provider:${id}:edit`
}
function bindingKey(id: number) {
  return `provider:${id}:bindings`
}
function modelsKey(id: number) {
  return `provider:${id}:models`
}

function modelNames(p: ProviderView): string[] {
  const list = (p.providerModels ?? []) as { model?: string }[]
  return Array.from(new Set(list.map((e) => e.model).filter((m): m is string => !!m)))
}

function openCreate() {
  panel.open(ProviderForm, { onSave: fetchProviders }, { key: 'provider:new' })
}

function openEdit(p: ProviderView) {
  panel.open(ProviderForm, { provider: p, onSave: fetchProviders }, { key: editKey(p.id) })
}

function toggleBindings(p: ProviderView) {
  panel.toggle(
    ProviderEndpointsPanel,
    { providerId: p.id, providerName: p.name },
    { key: bindingKey(p.id) },
  )
}

function toggleModels(p: ProviderView) {
  panel.toggle(
    ProviderModelsPanel,
    { providerId: p.id, providerName: p.name, onSave: fetchProviders },
    { key: modelsKey(p.id) },
  )
}

async function toggleDisabled(p: ProviderView) {
  const body = {
    id: p.id,
    name: p.name,
    credentials: p.credentials,
    priority: p.priority,
    providerModels: p.providerModels,
    annotations: p.annotations,
    disabled: !p.disabled,
  }
  const { error } = await api.PUT('/api/picotera/providers', { body })
  if (!error) fetchProviders()
}

function confirmDelete(_event: Event, p: ProviderView) {
  confirm.require({
    message: `确定要删除渠道「${p.name}」吗？此操作不可撤销。`,
    accept: async () => {
      await api.POST('/api/picotera/providers/delete', { body: { id: p.id } })
      if (
        panel.isActive(editKey(p.id)) ||
        panel.isActive(bindingKey(p.id)) ||
        panel.isActive(modelsKey(p.id))
      )
        panel.close()
      fetchProviders()
    },
  })
}

function rowSelected(id: number) {
  return (
    panel.isActive(editKey(id)) ||
    panel.isActive(bindingKey(id)) ||
    panel.isActive(modelsKey(id))
  )
}
</script>

<template>
  <div class="flex flex-col gap-3.5">
    <div class="flex items-center justify-between gap-3">
      <span class="text-xs text-ink-faint tabular-nums">{{ count }} 个渠道</span>
      <div class="flex items-center gap-2">
        <Button @click="openCreate">
          <Icon name="plus" :size="14" :stroke-width="2.2" />
          <span>新增渠道</span>
        </Button>
      </div>
    </div>
    <StateText v-if="loading">加载中…</StateText>
    <DataCard v-else-if="providers.length">
      <DataTable>
        <thead>
          <tr>
            <Th>ID</Th>
            <Th>名称</Th>
            <Th>凭证</Th>
            <Th>优先级</Th>
            <Th>模型</Th>
            <Th actions />
          </tr>
        </thead>
        <tbody>
          <Tr v-for="p in providers" :key="p.id" :selected="rowSelected(p.id)" :class="p.disabled ? 'opacity-55' : ''">
            <Td><span class="font-mono text-ink-faint">{{ p.id }}</span></Td>
            <Td>
              <span class="font-medium">{{ p.name }}</span>
              <Tag v-if="p.disabled" variant="muted" class="ml-1.5">已禁用</Tag>
            </Td>
            <Td><span class="font-mono text-ink-faint">{{ p.credentials.slice(0, 12) }}…</span></Td>
            <Td><Badge>{{ p.priority }}</Badge></Td>
            <Td>
              <TagList>
                <Tag
                  v-for="m in modelNames(p).slice(0, 3)"
                  :key="m"
                  variant="accent"
                >{{ m }}</Tag>
                <Tag
                  v-if="modelNames(p).length > 3"
                  variant="more"
                >+{{ modelNames(p).length - 3 }}</Tag>
              </TagList>
            </Td>
            <Td actions>
              <div class="inline-flex gap-1 opacity-55 group-hover:opacity-100 transition-opacity">
                <IconButton
                  :title="p.disabled ? '启用渠道' : '禁用渠道'"
                  :aria-label="p.disabled ? '启用渠道' : '禁用渠道'"
                  @click="toggleDisabled(p)"
                >
                  <Icon :name="p.disabled ? 'eye-off' : 'eye'" :size="13" />
                </IconButton>
                <IconButton
                  :active="panel.isActive(modelsKey(p.id))"
                  title="模型"
                  aria-label="模型"
                  :aria-pressed="panel.isActive(modelsKey(p.id))"
                  @click="toggleModels(p)"
                >
                  <Icon name="cpu" :size="13" />
                </IconButton>
                <IconButton
                  :active="panel.isActive(bindingKey(p.id))"
                  title="端点绑定"
                  aria-label="端点绑定"
                  :aria-pressed="panel.isActive(bindingKey(p.id))"
                  @click="toggleBindings(p)"
                >
                  <Icon name="link" :size="13" />
                </IconButton>
                <IconButton
                  :active="panel.isActive(editKey(p.id))"
                  title="编辑"
                  aria-label="编辑"
                  @click="openEdit(p)"
                >
                  <Icon name="edit" :size="13" />
                </IconButton>
                <IconButton
                  variant="danger"
                  title="删除"
                  aria-label="删除"
                  @click="(ev: Event) => confirmDelete(ev, p)"
                >
                  <Icon name="trash" :size="13" />
                </IconButton>
              </div>
            </Td>
          </Tr>
        </tbody>
      </DataTable>
    </DataCard>
    <StateText v-else>暂无渠道，点击右上角按钮新增</StateText>
  </div>
</template>
