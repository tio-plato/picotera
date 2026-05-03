<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import { useConfirm } from '@/composables/useConfirm'
import { useApi } from '@/composables/useApi'
import { useSidePanel } from '@/composables/useSidePanel'
import type { ApiKeyView } from '@/api'
import ApiKeyForm from '@/components/ApiKeyForm.vue'
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
  Icon,
} from '@/ui'

const panel = useSidePanel()
const confirm = useConfirm()
const api = useApi()

const apiKeys = ref<ApiKeyView[]>([])
const loading = ref(true)
const count = computed(() => apiKeys.value.length)
const copiedId = ref<number | null>(null)
let copyTimer: ReturnType<typeof setTimeout> | null = null

async function fetchApiKeys() {
  loading.value = true
  const { data, error } = await api.GET('/api/picotera/api-keys')
  if (!error && data) apiKeys.value = data as ApiKeyView[]
  loading.value = false
}

onMounted(fetchApiKeys)

function openCreate() {
  panel.open(ApiKeyForm, { onSave: fetchApiKeys }, { key: 'apiKey:new', width: '560px' })
}

function openEdit(k: ApiKeyView) {
  panel.open(
    ApiKeyForm,
    { apiKey: k, onSave: fetchApiKeys },
    { key: `apiKey:${k.id}`, width: '560px' },
  )
}

function confirmDelete(_event: Event, k: ApiKeyView) {
  confirm.require({
    message: `确定要删除 API Key「${k.name || k.id}」吗？此操作不可撤销。`,
    accept: async () => {
      await api.POST('/api/picotera/api-keys/delete', { body: { id: k.id } })
      fetchApiKeys()
    },
  })
}

async function toggle(k: ApiKeyView) {
  await api.PUT('/api/picotera/api-keys/{id}', {
    params: { path: { id: k.id } },
    body: {
      name: k.name,
      key: k.key,
      disabled: !k.disabled,
      annotations: k.annotations ?? {},
    },
  })
  fetchApiKeys()
}

async function copyKey(k: ApiKeyView) {
  try {
    await navigator.clipboard.writeText(k.key)
    copiedId.value = k.id
    if (copyTimer) clearTimeout(copyTimer)
    copyTimer = setTimeout(() => {
      copiedId.value = null
    }, 1500)
  } catch {
    // clipboard unavailable — silently ignore
  }
}
</script>

<template>
  <div class="flex flex-col gap-3.5">
    <div class="flex items-center justify-between gap-3">
      <span class="text-xs text-ink-faint tabular-nums">{{ count }} 个 API Key</span>
      <div class="flex items-center gap-2">
        <Button @click="openCreate">
          <Icon name="plus" :size="14" :stroke-width="2.2" />
          <span>新增 API Key</span>
        </Button>
      </div>
    </div>
    <StateText v-if="loading">加载中…</StateText>
    <DataCard v-else-if="apiKeys.length">
      <DataTable>
        <thead>
          <tr>
            <Th>名称</Th>
            <Th>Key</Th>
            <Th actions />
          </tr>
        </thead>
        <tbody>
          <Tr
            v-for="k in apiKeys"
            :key="k.id"
            :selected="panel.isActive(`apiKey:${k.id}`)"
            :class="k.disabled ? 'opacity-55' : ''"
          >
            <Td>
              <span class="font-medium">{{ k.name }}</span>
              <Tag v-if="k.disabled" variant="muted" class="ml-1.5">已禁用</Tag>
            </Td>
            <Td>
              <div class="inline-flex items-center gap-1.5">
                <code class="font-mono text-2xs text-ink-muted break-all">{{ k.key }}</code>
                <IconButton
                  :title="copiedId === k.id ? '已复制' : '复制 Key'"
                  :aria-label="copiedId === k.id ? '已复制' : '复制 Key'"
                  @click="copyKey(k)"
                >
                  <Icon :name="copiedId === k.id ? 'check' : 'copy'" :size="13" />
                </IconButton>
              </div>
            </Td>
            <Td actions>
              <div class="inline-flex gap-1 opacity-55 group-hover:opacity-100 transition-opacity">
                <IconButton
                  :title="k.disabled ? '启用' : '禁用'"
                  :aria-label="k.disabled ? '启用' : '禁用'"
                  @click="toggle(k)"
                >
                  <Icon :name="k.disabled ? 'eye-off' : 'eye'" :size="13" />
                </IconButton>
                <IconButton
                  :active="panel.isActive(`apiKey:${k.id}`)"
                  title="编辑"
                  aria-label="编辑"
                  @click="openEdit(k)"
                >
                  <Icon name="edit" :size="13" />
                </IconButton>
                <IconButton
                  variant="danger"
                  title="删除"
                  aria-label="删除"
                  @click="(ev: Event) => confirmDelete(ev, k)"
                >
                  <Icon name="trash" :size="13" />
                </IconButton>
              </div>
            </Td>
          </Tr>
        </tbody>
      </DataTable>
    </DataCard>
    <StateText v-else>暂无 API Key，点击右上角按钮新增</StateText>
  </div>
</template>
