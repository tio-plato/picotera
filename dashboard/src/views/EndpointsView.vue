<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import { useConfirm } from 'primevue/useconfirm'
import { useApi } from '@/composables/useApi'
import type { EndpointView } from '@/api'
import EndpointForm from '@/components/EndpointForm.vue'
import { useSidePanel } from '@/composables/useSidePanel'

const panel = useSidePanel()
const confirm = useConfirm()
const api = useApi()

const endpoints = ref<EndpointView[]>([])
const loading = ref(true)
const count = computed(() => endpoints.value.length)

async function fetchEndpoints() {
  loading.value = true
  const { data, error } = await api.GET('/api/picotera/endpoints')
  if (!error && data) endpoints.value = data as EndpointView[]
  loading.value = false
}

onMounted(fetchEndpoints)

function openCreate() {
  panel.open(EndpointForm, { onSave: fetchEndpoints }, { key: 'endpoint:new' })
}

function openEdit(ep: EndpointView) {
  panel.open(EndpointForm, { endpoint: ep, onSave: fetchEndpoints }, { key: `endpoint:${ep.path}` })
}

function confirmDeleteEndpoint(event: Event, path: string) {
  confirm.require({
    target: event.currentTarget as HTMLElement,
    message: `确定要删除端点「${path}」吗？此操作不可撤销。`,
    icon: 'pi pi-exclamation-triangle',
    rejectProps: { label: '取消', severity: 'secondary', outlined: true },
    acceptProps: { label: '删除', severity: 'danger' },
    accept: async () => {
      await api.POST('/api/picotera/endpoints/delete', { body: { path } })
      fetchEndpoints()
    },
  })
}
</script>

<template>
  <div class="view">
    <div class="view-toolbar">
      <span class="view-toolbar__meta">{{ count }} 个端点</span>
      <div class="view-toolbar__actions">
        <button class="btn-primary" @click="openCreate">
          <svg viewBox="0 0 24 24" width="14" height="14" fill="none" stroke="currentColor" stroke-width="2.2" stroke-linecap="round" aria-hidden="true"><path d="M12 5v14M5 12h14" /></svg>
          <span>新增端点</span>
        </button>
      </div>
    </div>
    <div v-if="loading" class="state-text">加载中…</div>
    <div v-else-if="endpoints.length" class="data-card">
      <table class="data-table">
        <thead>
          <tr>
            <th>路径</th>
            <th>名称</th>
            <th>模型字段</th>
            <th>凭证解析</th>
            <th class="col-actions"></th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="e in endpoints" :key="e.path" :class="{ selected: panel.isActive(`endpoint:${e.path}`) }">
            <td class="mono font-medium">{{ e.path }}</td>
            <td>{{ e.name }}</td>
            <td class="mono muted">{{ e.modelPath }}</td>
            <td>
              <span class="tag" :class="e.credentialsResolver === 'generalApiKey' ? 'tag--ok' : 'tag--muted'">
                {{ e.credentialsResolver }}
              </span>
            </td>
            <td class="col-actions">
              <div class="col-actions-cell">
                <button
                  class="btn-icon"
                  :class="{ 'btn-icon--active': panel.isActive(`endpoint:${e.path}`) }"
                  title="编辑"
                  aria-label="编辑"
                  @click="openEdit(e)"
                >
                  <svg viewBox="0 0 24 24" width="13" height="13" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><path d="M4 20h4L20 8l-4-4L4 16v4z" /><path d="M14 6l4 4" /></svg>
                </button>
                <button class="btn-icon btn-icon--danger" title="删除" aria-label="删除" @click="(ev) => confirmDeleteEndpoint(ev, e.path)">
                  <svg viewBox="0 0 24 24" width="13" height="13" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><path d="M4 7h16" /><path d="M10 11v6M14 11v6" /><path d="M6 7l1 12a2 2 0 0 0 2 2h6a2 2 0 0 0 2-2l1-12" /><path d="M9 7V5a2 2 0 0 1 2-2h2a2 2 0 0 1 2 2v2" /></svg>
                </button>
              </div>
            </td>
          </tr>
        </tbody>
      </table>
    </div>
    <div v-else class="state-text">暂无端点，点击右上角按钮新增</div>
  </div>
</template>
