<script setup lang="ts">
import { ref, onMounted, inject } from 'vue'
import api from '@/api'
import type { EndpointView } from '@/api'
import EndpointForm from '@/components/EndpointForm.vue'

const overlay = inject('overlay') as { open: (c: any, p?: Record<string, any>) => void; close: () => void }

const endpoints = ref<EndpointView[]>([])
const loading = ref(true)

async function fetchEndpoints() {
  loading.value = true
  const { data, error } = await api.GET('/api/picotera/endpoints')
  if (!error && data) endpoints.value = data as EndpointView[]
  loading.value = false
}

onMounted(fetchEndpoints)

function openCreate() {
  overlay.open(EndpointForm, { onSave: fetchEndpoints })
}

function openEdit(ep: EndpointView) {
  overlay.open(EndpointForm, { endpoint: ep, onSave: fetchEndpoints })
}

async function deleteEndpoint(path: string) {
  await api.POST('/api/picotera/endpoints/delete', { body: { path } })
  fetchEndpoints()
}
</script>

<template>
  <div class="view">
    <div class="toolbar">
      <button class="btn-primary" @click="openCreate">+ 新增端点</button>
    </div>
    <div v-if="loading" class="state-text">加载中…</div>
    <table v-else-if="endpoints.length" class="data-table">
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
        <tr v-for="e in endpoints" :key="e.path">
          <td class="mono font-medium">{{ e.path }}</td>
          <td>{{ e.name }}</td>
          <td class="mono">{{ e.modelPath }}</td>
          <td>
            <span class="tag" :class="e.credentialsResolver === 'generalApiKey' ? 'tag-ok' : 'tag-muted'">
              {{ e.credentialsResolver }}
            </span>
          </td>
          <td class="col-actions">
            <button class="btn-icon" title="编辑" @click="openEdit(e)">✎</button>
            <button class="btn-icon btn-icon-danger" title="删除" @click="deleteEndpoint(e.path)">✕</button>
          </td>
        </tr>
      </tbody>
    </table>
    <div v-else class="state-text">暂无端点，点击上方按钮新增</div>
  </div>
</template>

<style scoped>
.view { display: flex; flex-direction: column; gap: 1rem; }
.toolbar { display: flex; justify-content: flex-end; }
.btn-primary {
  padding: 0.375rem 0.875rem;
  background: var(--color-accent);
  color: #fff;
  border: none;
  border-radius: 0.375rem;
  font-size: 0.8125rem;
  font-weight: 500;
  cursor: pointer;
  transition: opacity 0.1s;
}
.btn-primary:hover { opacity: 0.9; }
.data-table {
  width: 100%;
  border-collapse: collapse;
  background: var(--color-card-bg);
  border: 1px solid var(--color-card-border);
  border-radius: 0.5rem;
  overflow: hidden;
  font-size: 0.8125rem;
}
.data-table th {
  text-align: left;
  padding: 0.625rem 1rem;
  background: var(--color-surface-50);
  color: var(--color-ink-muted);
  font-weight: 550;
  font-size: 0.75rem;
  text-transform: uppercase;
  letter-spacing: 0.04em;
  border-bottom: 1px solid var(--color-card-border);
}
.data-table td {
  padding: 0.625rem 1rem;
  border-bottom: 1px solid oklch(0.95 0.003 250);
  vertical-align: middle;
}
.data-table tbody tr:hover { background: var(--color-surface-50); }
.mono { font-family: var(--font-mono); font-size: 0.75rem; }
.font-medium { font-weight: 500; }
.col-actions { width: 5rem; }
.tag {
  padding: 0.0625rem 0.375rem;
  border-radius: 0.25rem;
  font-family: var(--font-mono);
  font-size: 0.6875rem;
  white-space: nowrap;
}
.tag-ok { background: oklch(0.93 0.04 145); color: oklch(0.35 0.12 145); }
.tag-muted { background: var(--color-surface-100); color: var(--color-ink-faint); }
.btn-icon {
  background: none; border: none; cursor: pointer; font-size: 0.875rem;
  color: var(--color-ink-faint); padding: 0.125rem 0.25rem; border-radius: 0.25rem;
}
.btn-icon:hover { background: var(--color-surface-100); color: var(--color-ink); }
.btn-icon-danger:hover { color: var(--color-indicator-err); }
.state-text {
  text-align: center;
  padding: 3rem 1rem;
  color: var(--color-ink-faint);
  font-size: 0.875rem;
}
</style>
