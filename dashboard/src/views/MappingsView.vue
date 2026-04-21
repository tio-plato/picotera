<script setup lang="ts">
import { ref, onMounted, inject } from 'vue'
import api from '@/api'
import type { ModelProviderEndpointView } from '@/api'
import MappingForm from '@/components/MappingForm.vue'

const overlay = inject('overlay') as { open: (c: any, p?: Record<string, any>) => void; close: () => void }

const mappings = ref<ModelProviderEndpointView[]>([])
const loading = ref(true)
const hasMore = ref(false)
const nextCursor = ref('')

async function fetchMappings(cursor?: string) {
  loading.value = true
  const { data, error } = await api.GET('/api/picotera/model-provider-endpoints', {
    params: { query: { limit: 50, cursor: cursor || undefined } },
  })
  if (!error && data) {
    const body = data
    if (cursor) {
      mappings.value.push(...(body.items ?? []))
    } else {
      mappings.value = body.items ?? []
    }
    hasMore.value = body.pagination.hasMore
    nextCursor.value = body.pagination.nextCursor ?? ''
  }
  loading.value = false
}

onMounted(() => fetchMappings())

function openCreate() {
  overlay.open(MappingForm, { onSave: () => fetchMappings() })
}

function openEdit(m: ModelProviderEndpointView) {
  overlay.open(MappingForm, { mapping: m, onSave: () => fetchMappings() })
}

async function deleteMapping(m: ModelProviderEndpointView) {
  await api.POST('/api/picotera/model-provider-endpoints/delete', {
    body: { modelName: m.modelName, providerId: m.providerId, endpointId: m.endpointId },
  })
  fetchMappings()
}
</script>

<template>
  <div class="view">
    <div class="toolbar">
      <button class="btn-primary" @click="openCreate">+ 新增映射</button>
    </div>
    <div v-if="loading && !mappings.length" class="state-text">加载中…</div>
    <table v-else-if="mappings.length" class="data-table">
      <thead>
        <tr>
          <th>模型</th>
          <th>渠道 ID</th>
          <th>端点 ID</th>
          <th>上游模型</th>
          <th>优先级</th>
          <th>标注</th>
          <th class="col-actions"></th>
        </tr>
      </thead>
      <tbody>
        <tr v-for="m in mappings" :key="`${m.modelName}-${m.providerId}-${m.endpointId}`">
          <td class="mono font-medium">{{ m.modelName }}</td>
          <td class="mono">{{ m.providerId }}</td>
          <td class="mono">{{ m.endpointId }}</td>
          <td class="mono">{{ m.upstreamModelName || '—' }}</td>
          <td><span class="badge">{{ m.priority }}</span></td>
          <td>
            <div class="tag-list">
              <span v-for="(v, k) in m.annotations" :key="k" class="tag">{{ k }}={{ v }}</span>
            </div>
          </td>
          <td class="col-actions">
            <button class="btn-icon" title="编辑" @click="openEdit(m)">✎</button>
            <button class="btn-icon btn-icon-danger" title="删除" @click="deleteMapping(m)">✕</button>
          </td>
        </tr>
      </tbody>
    </table>
    <div v-else class="state-text">暂无映射，点击上方按钮新增</div>
    <div v-if="hasMore" class="load-more">
      <button class="btn-ghost" @click="fetchMappings(nextCursor)">加载更多</button>
    </div>
  </div>
</template>

<style scoped>
.view { display: flex; flex-direction: column; gap: 1rem; }
.toolbar { display: flex; justify-content: flex-end; }
.btn-primary {
  padding: 0.375rem 0.875rem; background: var(--color-accent); color: #fff; border: none;
  border-radius: 0.375rem; font-size: 0.8125rem; font-weight: 500; cursor: pointer; transition: opacity 0.1s;
}
.btn-primary:hover { opacity: 0.9; }
.data-table {
  width: 100%; border-collapse: collapse; background: var(--color-card-bg);
  border: 1px solid var(--color-card-border); border-radius: 0.5rem; overflow: hidden; font-size: 0.8125rem;
}
.data-table th {
  text-align: left; padding: 0.625rem 1rem; background: var(--color-surface-50);
  color: var(--color-ink-muted); font-weight: 550; font-size: 0.75rem;
  text-transform: uppercase; letter-spacing: 0.04em; border-bottom: 1px solid var(--color-card-border);
}
.data-table td {
  padding: 0.625rem 1rem; border-bottom: 1px solid oklch(0.95 0.003 250); vertical-align: middle;
}
.data-table tbody tr:hover { background: var(--color-surface-50); }
.mono { font-family: var(--font-mono); font-size: 0.75rem; }
.font-medium { font-weight: 500; }
.col-actions { width: 5rem; }
.badge {
  display: inline-block; padding: 0.125rem 0.5rem; background: var(--color-surface-100);
  border-radius: 0.25rem; font-family: var(--font-mono); font-size: 0.75rem; color: var(--color-ink-muted);
}
.tag-list { display: flex; gap: 0.25rem; flex-wrap: wrap; }
.tag {
  padding: 0.0625rem 0.375rem; background: var(--color-surface-100); color: var(--color-ink-muted);
  border-radius: 0.25rem; font-family: var(--font-mono); font-size: 0.6875rem; white-space: nowrap;
}
.btn-icon {
  background: none; border: none; cursor: pointer; font-size: 0.875rem;
  color: var(--color-ink-faint); padding: 0.125rem 0.25rem; border-radius: 0.25rem;
}
.btn-icon:hover { background: var(--color-surface-100); color: var(--color-ink); }
.btn-icon-danger:hover { color: var(--color-indicator-err); }
.load-more { display: flex; justify-content: center; padding: 0.5rem; }
.btn-ghost {
  padding: 0.375rem 0.875rem; background: none; border: 1px solid var(--color-card-border);
  border-radius: 0.375rem; font-size: 0.8125rem; cursor: pointer; color: var(--color-ink-muted);
}
.btn-ghost:hover { background: var(--color-surface-50); }
.state-text {
  text-align: center; padding: 3rem 1rem; color: var(--color-ink-faint); font-size: 0.875rem;
}
</style>
