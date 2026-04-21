<script setup lang="ts">
import { ref } from 'vue'
import api from '@/api'
import type { EndpointView } from '@/api'

const emit = defineEmits<{ close: [] }>()
const props = defineProps<{ endpoint?: EndpointView; onSave?: () => void }>()

const isEdit = !!props.endpoint
const form = ref({
  name: props.endpoint?.name ?? '',
  path: props.endpoint?.path ?? '',
  modelPath: props.endpoint?.modelPath ?? '',
  credentialsResolver: props.endpoint?.credentialsResolver ?? 'generalApiKey' as const,
})
const saving = ref(false)
const error = ref('')

async function submit() {
  saving.value = true
  error.value = ''
  const { error: err } = await api.PUT('/api/picotera/endpoints', { body: form.value })
  if (err) {
    error.value = err.message ?? '操作失败'
  } else {
    props.onSave?.()
    emit('close')
  }
  saving.value = false
}
</script>

<template>
  <div class="form-panel">
    <div class="form-header">
      <h2 class="form-title">{{ isEdit ? '编辑端点' : '新增端点' }}</h2>
      <button class="close-btn" @click="emit('close')">&times;</button>
    </div>
    <form @submit.prevent="submit" class="form-body">
      <label class="field">
        <span class="field-label">路径</span>
        <input v-model="form.path" class="input" required placeholder="例如 /api/v1/chat/completions" :disabled="isEdit" />
      </label>
      <label class="field">
        <span class="field-label">名称</span>
        <input v-model="form.name" class="input" required placeholder="例如 Chat Completions" />
      </label>
      <label class="field">
        <span class="field-label">模型字段路径</span>
        <input v-model="form.modelPath" class="input" required placeholder="例如 body.model" />
      </label>
      <label class="field">
        <span class="field-label">凭证解析</span>
        <select v-model="form.credentialsResolver" class="input">
          <option value="generalApiKey">generalApiKey</option>
          <option value="unknown">unknown</option>
        </select>
      </label>
      <div v-if="error" class="form-error">{{ error }}</div>
      <div class="form-actions">
        <button type="button" class="btn-ghost" @click="emit('close')">取消</button>
        <button type="submit" class="btn-primary" :disabled="saving">{{ saving ? '保存中…' : isEdit ? '更新' : '创建' }}</button>
      </div>
    </form>
  </div>
</template>

<style scoped>
.form-panel { padding: 0; }
.form-header {
  display: flex; align-items: center; justify-content: space-between;
  padding: 1rem 1.5rem; border-bottom: 1px solid var(--color-card-border);
}
.form-title { font-size: 1rem; font-weight: 600; margin: 0; }
.close-btn {
  background: none; border: none; font-size: 1.25rem; color: var(--color-ink-faint);
  cursor: pointer; line-height: 1; padding: 0.25rem;
}
.close-btn:hover { color: var(--color-ink); }
.form-body { padding: 1.25rem 1.5rem; display: flex; flex-direction: column; gap: 1rem; }
.field { display: flex; flex-direction: column; gap: 0.25rem; }
.field-label {
  font-size: 0.75rem; font-weight: 550; color: var(--color-ink-muted);
  text-transform: uppercase; letter-spacing: 0.03em;
}
.input {
  padding: 0.5rem 0.75rem; border: 1px solid var(--color-card-border); border-radius: 0.375rem;
  font-size: 0.8125rem; font-family: var(--font-sans); background: var(--color-surface-0); transition: border-color 0.1s;
}
.input:focus { outline: none; border-color: var(--color-accent); }
.input:disabled { opacity: 0.5; }
.form-error { color: var(--color-indicator-err); font-size: 0.8125rem; }
.form-actions { display: flex; justify-content: flex-end; gap: 0.5rem; padding-top: 0.5rem; }
.btn-ghost {
  padding: 0.375rem 0.875rem; background: none; border: 1px solid var(--color-card-border);
  border-radius: 0.375rem; font-size: 0.8125rem; cursor: pointer; color: var(--color-ink-muted);
}
.btn-ghost:hover { background: var(--color-surface-50); }
.btn-primary {
  padding: 0.375rem 0.875rem; background: var(--color-accent); color: #fff; border: none;
  border-radius: 0.375rem; font-size: 0.8125rem; font-weight: 500; cursor: pointer;
}
.btn-primary:hover { opacity: 0.9; }
.btn-primary:disabled { opacity: 0.5; cursor: not-allowed; }
</style>
