<script setup lang="ts">
import { ref, onMounted, inject, computed } from 'vue'
import api from '@/api'
import type { ModelView } from '@/api'
import ModelForm from '@/components/ModelForm.vue'

const overlay = inject('overlay') as { open: (c: any, p?: Record<string, any>) => void; close: () => void }

const models = ref<ModelView[]>([])
const loading = ref(true)
const count = computed(() => models.value.length)

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
    <div class="view-toolbar">
      <span class="view-toolbar__meta">{{ count }} 个模型</span>
      <div class="view-toolbar__actions">
        <button class="btn-primary" @click="openCreate">
          <svg viewBox="0 0 24 24" width="14" height="14" fill="none" stroke="currentColor" stroke-width="2.2" stroke-linecap="round" aria-hidden="true"><path d="M12 5v14M5 12h14" /></svg>
          <span>新增模型</span>
        </button>
      </div>
    </div>
    <div v-if="loading" class="state-text">加载中…</div>
    <div v-else-if="models.length" class="data-card">
      <table class="data-table">
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
            <td class="muted">{{ m.developer }}</td>
            <td><span class="tag">{{ m.series }}</span></td>
          </tr>
        </tbody>
      </table>
    </div>
    <div v-else class="state-text">暂无模型，点击右上角按钮新增</div>
  </div>
</template>
