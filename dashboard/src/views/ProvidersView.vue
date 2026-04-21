<script setup lang="ts">
import { ref, onMounted, inject, computed } from 'vue'
import api from '@/api'
import type { ProviderView } from '@/api'
import ProviderForm from '@/components/ProviderForm.vue'

const overlay = inject('overlay') as { open: (c: any, p?: Record<string, any>) => void; close: () => void }

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

function openCreate() {
  overlay.open(ProviderForm, { onSave: fetchProviders })
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
          </tr>
        </thead>
        <tbody>
          <tr v-for="p in providers" :key="p.id">
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
          </tr>
        </tbody>
      </table>
    </div>
    <div v-else class="state-text">暂无渠道，点击右上角按钮新增</div>
  </div>
</template>
