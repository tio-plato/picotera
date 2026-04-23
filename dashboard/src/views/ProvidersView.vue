<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import { useConfirm } from 'primevue/useconfirm'
import { useApi } from '@/composables/useApi'
import type { ProviderView } from '@/api'
import ProviderForm from '@/components/ProviderForm.vue'
import ProviderEndpointsPanel from '@/components/ProviderEndpointsPanel.vue'
import { useSidePanel } from '@/composables/useSidePanel'

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

function confirmDelete(event: Event, p: ProviderView) {
  confirm.require({
    target: event.currentTarget as HTMLElement,
    message: `确定要删除渠道「${p.name}」吗？此操作不可撤销。`,
    icon: 'pi pi-exclamation-triangle',
    rejectProps: { label: '取消', severity: 'secondary', outlined: true },
    acceptProps: { label: '删除', severity: 'danger' },
    accept: async () => {
      await api.POST('/api/picotera/providers/delete', { body: { id: p.id } })
      if (panel.isActive(editKey(p.id)) || panel.isActive(bindingKey(p.id))) panel.close()
      fetchProviders()
    },
  })
}

function rowSelected(id: number) {
  return panel.isActive(editKey(id)) || panel.isActive(bindingKey(id))
}
</script>

<template>
  <div class="view">
    <div class="view-toolbar">
      <span class="view-toolbar__meta">{{ count }} 个渠道</span>
      <div class="view-toolbar__actions">
        <button class="btn-primary" @click="openCreate">
          <svg viewBox="0 0 24 24" width="14" height="14" fill="none" stroke="currentColor" stroke-width="2.2" stroke-linecap="round" aria-hidden="true"><path d="M12 5v14M5 12h14" /></svg>
          <span>新增渠道</span>
        </button>
      </div>
    </div>
    <div v-if="loading" class="state-text">加载中…</div>
    <div v-else-if="providers.length" class="data-card">
      <table class="data-table">
        <thead>
          <tr>
            <th>ID</th>
            <th>名称</th>
            <th>凭证</th>
            <th>优先级</th>
            <th>上游模型</th>
            <th class="col-actions"></th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="p in providers" :key="p.id" :class="{ selected: rowSelected(p.id) }">
            <td class="mono muted">{{ p.id }}</td>
            <td class="font-medium">{{ p.name }}</td>
            <td class="mono muted">{{ p.credentials.slice(0, 12) }}…</td>
            <td><span class="badge">{{ p.priority }}</span></td>
            <td>
              <div class="tag-list">
                <span v-for="m in (p.providerModels ?? []).slice(0, 3)" :key="m" class="tag tag--accent">{{ m }}</span>
                <span v-if="(p.providerModels ?? []).length > 3" class="tag tag--more">+{{ (p.providerModels ?? []).length - 3 }}</span>
              </div>
            </td>
            <td class="col-actions">
              <div class="col-actions-cell">
                <button
                  class="btn-icon"
                  :class="{ 'btn-icon--active': panel.isActive(bindingKey(p.id)) }"
                  title="端点绑定"
                  aria-label="端点绑定"
                  :aria-pressed="panel.isActive(bindingKey(p.id))"
                  @click="toggleBindings(p)"
                >
                  <svg viewBox="0 0 24 24" width="13" height="13" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><path d="M10 14a5 5 0 0 0 7 0l3-3a5 5 0 0 0-7-7l-1 1" /><path d="M14 10a5 5 0 0 0-7 0l-3 3a5 5 0 0 0 7 7l1-1" /></svg>
                </button>
                <button
                  class="btn-icon"
                  :class="{ 'btn-icon--active': panel.isActive(editKey(p.id)) }"
                  title="编辑"
                  aria-label="编辑"
                  @click="openEdit(p)"
                >
                  <svg viewBox="0 0 24 24" width="13" height="13" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><path d="M4 20h4L20 8l-4-4L4 16v4z" /><path d="M14 6l4 4" /></svg>
                </button>
                <button class="btn-icon btn-icon--danger" title="删除" aria-label="删除" @click="(ev) => confirmDelete(ev, p)">
                  <svg viewBox="0 0 24 24" width="13" height="13" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><path d="M4 7h16" /><path d="M10 11v6M14 11v6" /><path d="M6 7l1 12a2 2 0 0 0 2 2h6a2 2 0 0 0 2-2l1-12" /><path d="M9 7V5a2 2 0 0 1 2-2h2a2 2 0 0 1 2 2v2" /></svg>
                </button>
              </div>
            </td>
          </tr>
        </tbody>
      </table>
    </div>
    <div v-else class="state-text">暂无渠道，点击右上角按钮新增</div>
  </div>
</template>
