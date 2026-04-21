<script setup lang="ts">
import { ref } from 'vue'
import api from '@/api'

const emit = defineEmits<{ close: []; saved: [] }>()
const props = defineProps<{ onSave?: () => void }>()

const form = ref({
  name: '',
  credentials: '',
  priority: 0,
  providerModels: '',
  annotations: '' as string,
})
const saving = ref(false)
const error = ref('')

async function submit() {
  saving.value = true
  error.value = ''
  const body = {
    name: form.value.name,
    credentials: form.value.credentials,
    priority: form.value.priority,
    providerModels: form.value.providerModels ? form.value.providerModels.split(',').map(s => s.trim()) : [],
    annotations: form.value.annotations ? Object.fromEntries(form.value.annotations.split(',').map(s => { const [k, v] = s.split('='); return [k.trim(), (v ?? '').trim()] })) : {},
  }
  const { error: err } = await api.POST('/api/picotera/providers', { body })
  if (err) {
    error.value = err.message ?? '创建失败'
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
      <h2 class="form-title">新增渠道</h2>
      <button class="close-btn" @click="emit('close')">&times;</button>
    </div>
    <form @submit.prevent="submit" class="form-body">
      <label class="field">
        <span class="field-label">名称</span>
        <input v-model="form.name" class="input" required placeholder="例如 openai" />
      </label>
      <label class="field">
        <span class="field-label">凭证</span>
        <input v-model="form.credentials" class="input" required placeholder="API Key 或密钥" />
      </label>
      <label class="field">
        <span class="field-label">优先级</span>
        <input v-model.number="form.priority" type="number" class="input" required />
      </label>
      <label class="field">
        <span class="field-label">模型列表</span>
        <input v-model="form.providerModels" class="input" placeholder="逗号分隔，如 gpt-4o, gpt-3.5-turbo" />
      </label>
      <label class="field">
        <span class="field-label">标注</span>
        <input v-model="form.annotations" class="input" placeholder="key=value, 逗号分隔" />
      </label>
      <div v-if="error" class="form-error">{{ error }}</div>
      <div class="form-actions">
        <button type="button" class="btn-ghost" @click="emit('close')">取消</button>
        <button type="submit" class="btn-primary" :disabled="saving">{{ saving ? '保存中…' : '创建' }}</button>
      </div>
    </form>
  </div>
</template>

<style scoped>
.form-panel { padding: 0; }
.form-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 1rem 1.5rem;
  border-bottom: 1px solid var(--color-card-border);
}
.form-title {
  font-size: 1rem;
  font-weight: 600;
  margin: 0;
}
.close-btn {
  background: none;
  border: none;
  font-size: 1.25rem;
  color: var(--color-ink-faint);
  cursor: pointer;
  line-height: 1;
  padding: 0.25rem;
}
.close-btn:hover { color: var(--color-ink); }
.form-body {
  padding: 1.25rem 1.5rem;
  display: flex;
  flex-direction: column;
  gap: 1rem;
}
.field { display: flex; flex-direction: column; gap: 0.25rem; }
.field-label {
  font-size: 0.75rem;
  font-weight: 550;
  color: var(--color-ink-muted);
  text-transform: uppercase;
  letter-spacing: 0.03em;
}
.input {
  padding: 0.5rem 0.75rem;
  border: 1px solid var(--color-card-border);
  border-radius: 0.375rem;
  font-size: 0.8125rem;
  font-family: var(--font-sans);
  background: var(--color-surface-0);
  transition: border-color 0.1s;
}
.input:focus { outline: none; border-color: var(--color-accent); }
.form-error { color: var(--color-indicator-err); font-size: 0.8125rem; }
.form-actions { display: flex; justify-content: flex-end; gap: 0.5rem; padding-top: 0.5rem; }
.btn-ghost {
  padding: 0.375rem 0.875rem;
  background: none;
  border: 1px solid var(--color-card-border);
  border-radius: 0.375rem;
  font-size: 0.8125rem;
  cursor: pointer;
  color: var(--color-ink-muted);
}
.btn-ghost:hover { background: var(--color-surface-50); }
.btn-primary {
  padding: 0.375rem 0.875rem;
  background: var(--color-accent);
  color: #fff;
  border: none;
  border-radius: 0.375rem;
  font-size: 0.8125rem;
  font-weight: 500;
  cursor: pointer;
}
.btn-primary:hover { opacity: 0.9; }
.btn-primary:disabled { opacity: 0.5; cursor: not-allowed; }
</style>
