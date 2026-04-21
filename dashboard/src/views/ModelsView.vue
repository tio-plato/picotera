<script setup lang="ts">
import { ref, onMounted, inject } from 'vue'
import api from '@/api'
import type { ModelView } from '@/api'
import ModelForm from '@/components/ModelForm.vue'

const overlay = inject('overlay') as { open: (c: any, p?: Record<string, any>) => void; close: () => void }

const models = ref<ModelView[]>([])
const loading = ref(true)

async function fetchModels() {
  loading.value = true
  const { data, error } = await api.GET('/api/picotera/models')
  if (!error && data) models.value = data as ModelView[]
  loading.value = false
}

onMounted(fetchModels)

function openCreate() {
  overlay.open(ModelForm, { onSave: fetchModels })
}
</script>

<template>
  <div class="view">
    <div class="toolbar">
      <button class="btn-primary" @click="openCreate">+ 新增模型</button>
    </div>
    <div v-if="loading" class="state-text">加载中…</div>
    <table v-else-if="models.length" class="data-table">
      <thead>
        <tr>
          <th>名称</th>
          <th>标题</th>
          <th>开发者</th>
          <th>系列</th>
        </tr>
      </thead>
      <tbody>
        <tr v-for="m in models" :key="m.name">
          <td class="mono font-medium">{{ m.name }}</td>
          <td>{{ m.title }}</td>
          <td>{{ m.developer }}</td>
          <td><span class="badge">{{ m.series }}</span></td>
        </tr>
      </tbody>
    </table>
    <div v-else class="state-text">暂无模型，点击上方按钮新增</div>
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
.badge {
  display: inline-block;
  padding: 0.125rem 0.5rem;
  background: var(--color-surface-100);
  border-radius: 0.25rem;
  font-size: 0.75rem;
  color: var(--color-ink-muted);
}
.state-text {
  text-align: center;
  padding: 3rem 1rem;
  color: var(--color-ink-faint);
  font-size: 0.875rem;
}
</style>
