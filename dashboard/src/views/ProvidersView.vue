<script setup lang="ts">
import { ref, onMounted, inject } from 'vue'
import api from '@/api'
import type { ProviderView } from '@/api'
import ProviderForm from '@/components/ProviderForm.vue'

const overlay = inject('overlay') as { open: (c: any, p?: Record<string, any>) => void; close: () => void }

const providers = ref<ProviderView[]>([])
const loading = ref(true)

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
    <div class="toolbar">
      <button class="btn-primary" @click="openCreate">+ 新增渠道</button>
    </div>
    <div v-if="loading" class="state-text">加载中…</div>
    <table v-else-if="providers.length" class="data-table">
      <thead>
        <tr>
          <th>ID</th>
          <th>名称</th>
          <th>凭证</th>
          <th>优先级</th>
          <th>模型</th>
        </tr>
      </thead>
      <tbody>
        <tr v-for="p in providers" :key="p.id">
          <td class="mono">{{ p.id }}</td>
          <td class="font-medium">{{ p.name }}</td>
          <td class="mono muted">{{ p.credentials.slice(0, 12) }}…</td>
          <td>
            <span class="badge">{{ p.priority }}</span>
          </td>
          <td>
            <div class="tag-list">
              <span v-for="m in (p.providerModels ?? []).slice(0, 3)" :key="m" class="tag">{{ m }}</span>
              <span v-if="(p.providerModels ?? []).length > 3" class="tag tag-more">+{{ (p.providerModels ?? []).length - 3 }}</span>
            </div>
          </td>
        </tr>
      </tbody>
    </table>
    <div v-else class="state-text">暂无渠道，点击上方按钮新增</div>
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
.muted { color: var(--color-ink-faint); }
.font-medium { font-weight: 500; }
.badge {
  display: inline-block;
  padding: 0.125rem 0.5rem;
  background: var(--color-surface-100);
  border-radius: 0.25rem;
  font-family: var(--font-mono);
  font-size: 0.75rem;
  color: var(--color-ink-muted);
}
.tag-list { display: flex; gap: 0.25rem; flex-wrap: wrap; }
.tag {
  padding: 0.0625rem 0.375rem;
  background: var(--color-accent-faint);
  color: oklch(0.45 0.12 255);
  border-radius: 0.25rem;
  font-family: var(--font-mono);
  font-size: 0.6875rem;
  white-space: nowrap;
}
.tag-more { background: var(--color-surface-100); color: var(--color-ink-faint); }
.state-text {
  text-align: center;
  padding: 3rem 1rem;
  color: var(--color-ink-faint);
  font-size: 0.875rem;
}
</style>
