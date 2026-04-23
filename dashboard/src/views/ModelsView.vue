<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import { useConfirm } from 'primevue/useconfirm'
import { useApi } from '@/composables/useApi'
import type { ModelView } from '@/api'
import ModelForm from '@/components/ModelForm.vue'
import { useSidePanel } from '@/composables/useSidePanel'

const panel = useSidePanel()
const confirm = useConfirm()
const api = useApi()

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
  panel.open(ModelForm, { onSave: fetchModels }, { key: 'model:new' })
}

function openEdit(m: ModelView) {
  panel.open(ModelForm, { model: m, onSave: fetchModels }, { key: `model:${m.name}` })
}

function confirmDelete(event: Event, m: ModelView) {
  confirm.require({
    target: event.currentTarget as HTMLElement,
    message: `确定要删除模型「${m.name}」吗？此操作不可撤销。`,
    icon: 'pi pi-exclamation-triangle',
    rejectProps: { label: '取消', severity: 'secondary', outlined: true },
    acceptProps: { label: '删除', severity: 'danger' },
    accept: async () => {
      await api.POST('/api/picotera/models/delete', { body: { name: m.name } })
      fetchModels()
    },
  })
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
            <th class="col-actions"></th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="m in models" :key="m.name" :class="{ selected: panel.isActive(`model:${m.name}`) }">
            <td class="mono font-medium">{{ m.name }}</td>
            <td>{{ m.title }}</td>
            <td class="muted">{{ m.developer }}</td>
            <td><span class="tag">{{ m.series }}</span></td>
            <td class="col-actions">
              <div class="col-actions-cell">
                <button
                  class="btn-icon"
                  :class="{ 'btn-icon--active': panel.isActive(`model:${m.name}`) }"
                  title="编辑"
                  aria-label="编辑"
                  @click="openEdit(m)"
                >
                  <svg viewBox="0 0 24 24" width="13" height="13" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><path d="M4 20h4L20 8l-4-4L4 16v4z" /><path d="M14 6l4 4" /></svg>
                </button>
                <button class="btn-icon btn-icon--danger" title="删除" aria-label="删除" @click="(e) => confirmDelete(e, m)">
                  <svg viewBox="0 0 24 24" width="13" height="13" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><path d="M4 7h16" /><path d="M10 11v6M14 11v6" /><path d="M6 7l1 12a2 2 0 0 0 2 2h6a2 2 0 0 0 2-2l1-12" /><path d="M9 7V5a2 2 0 0 1 2-2h2a2 2 0 0 1 2 2v2" /></svg>
                </button>
              </div>
            </td>
          </tr>
        </tbody>
      </table>
    </div>
    <div v-else class="state-text">暂无模型，点击右上角按钮新增</div>
  </div>
</template>

