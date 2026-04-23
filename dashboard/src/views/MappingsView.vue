<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import { useApi } from '@/composables/useApi'
import type { ModelProviderEndpointView } from '@/api'
import MappingForm from '@/components/MappingForm.vue'
import ConfirmDialog from '@/components/ConfirmDialog.vue'
import { useOverlay } from '@/composables/useOverlay'

const overlay = useOverlay()
const api = useApi()

const mappings = ref<ModelProviderEndpointView[]>([])
const loading = ref(true)
const hasMore = ref(false)
const nextCursor = ref('')
const count = computed(() => mappings.value.length)

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

function confirmDeleteMapping(m: ModelProviderEndpointView) {
  overlay.open(ConfirmDialog, {
    title: '删除映射',
    message: `确定要删除模型「${m.modelName}」的映射吗？此操作不可撤销。`,
    onConfirm: async () => {
      await api.POST('/api/picotera/model-provider-endpoints/delete', {
        body: { modelName: m.modelName, providerId: m.providerId, endpointPath: m.endpointPath },
      })
      fetchMappings()
    },
  })
}
</script>

<template>
  <div class="view">
    <div class="view-toolbar">
      <span class="view-toolbar__meta">
        {{ count }} 条映射<span v-if="hasMore">（还有更多）</span>
      </span>
      <div class="view-toolbar__actions">
        <button class="btn-primary" @click="openCreate">
          <svg viewBox="0 0 24 24" width="14" height="14" fill="none" stroke="currentColor" stroke-width="2.2" stroke-linecap="round" aria-hidden="true"><path d="M12 5v14M5 12h14" /></svg>
          <span>新增映射</span>
        </button>
      </div>
    </div>
    <div v-if="loading && !mappings.length" class="state-text">加载中…</div>
    <div v-else-if="mappings.length" class="data-card">
      <table class="data-table">
        <thead>
          <tr>
            <th>模型</th>
            <th>渠道</th>
            <th>端点</th>
            <th>上游模型</th>
            <th>优先级</th>
            <th>标注</th>
            <th class="col-actions"></th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="m in mappings" :key="`${m.modelName}-${m.providerId}-${m.endpointPath}`">
            <td class="mono font-medium">{{ m.modelName }}</td>
            <td class="mono muted">{{ m.providerId }}</td>
            <td class="mono muted">{{ m.endpointPath }}</td>
            <td class="mono">{{ m.upstreamModelName || '—' }}</td>
            <td><span class="badge">{{ m.priority }}</span></td>
            <td>
              <div class="tag-list">
                <span v-for="(v, k) in m.annotations" :key="k" class="tag">{{ k }}={{ v }}</span>
              </div>
            </td>
            <td class="col-actions">
              <div class="col-actions-cell">
                <button class="btn-icon" title="编辑" aria-label="编辑" @click="openEdit(m)">
                  <svg viewBox="0 0 24 24" width="13" height="13" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><path d="M4 20h4L20 8l-4-4L4 16v4z" /><path d="M14 6l4 4" /></svg>
                </button>
                <button class="btn-icon btn-icon--danger" title="删除" aria-label="删除" @click="confirmDeleteMapping(m)">
                  <svg viewBox="0 0 24 24" width="13" height="13" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><path d="M4 7h16" /><path d="M10 11v6M14 11v6" /><path d="M6 7l1 12a2 2 0 0 0 2 2h6a2 2 0 0 0 2-2l1-12" /><path d="M9 7V5a2 2 0 0 1 2-2h2a2 2 0 0 1 2 2v2" /></svg>
                </button>
              </div>
            </td>
          </tr>
        </tbody>
      </table>
    </div>
    <div v-else class="state-text">暂无映射，点击右上角按钮新增</div>
    <div v-if="hasMore" class="load-more">
      <button class="btn-ghost" @click="fetchMappings(nextCursor)">加载更多</button>
    </div>
  </div>
</template>
