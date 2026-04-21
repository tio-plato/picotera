<script setup lang="ts">
import { ref } from 'vue'
import api from '@/api'
import type { ModelView } from '@/api'

const emit = defineEmits<{ close: [] }>()
const props = defineProps<{ model?: ModelView; onSave?: () => void }>()

const isEdit = !!props.model
const form = ref({
  name: props.model?.name ?? '',
  title: props.model?.title ?? '',
  developer: props.model?.developer ?? '',
  series: props.model?.series ?? '',
})
const saving = ref(false)
const error = ref('')

async function submit() {
  saving.value = true
  error.value = ''
  const { error: err } = await api.PUT('/api/picotera/models', { body: form.value })
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
      <h2 class="form-title">{{ isEdit ? '编辑模型' : '新增模型' }}</h2>
      <button class="close-btn" @click="emit('close')">&times;</button>
    </div>
    <form @submit.prevent="submit" class="form-body">
      <label class="field">
        <span class="field-label">名称</span>
        <input v-model="form.name" class="input" required placeholder="例如 gpt-4o" :disabled="isEdit" />
      </label>
      <label class="field">
        <span class="field-label">标题</span>
        <input v-model="form.title" class="input" required placeholder="例如 GPT-4o" />
      </label>
      <label class="field">
        <span class="field-label">开发者</span>
        <input v-model="form.developer" class="input" required placeholder="例如 OpenAI" />
      </label>
      <label class="field">
        <span class="field-label">系列</span>
        <input v-model="form.series" class="input" required placeholder="例如 GPT" />
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
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 1rem 1.5rem;
  border-bottom: 1px solid var(--color-card-border);
}
.form-title { font-size: 1rem; font-weight: 600; margin: 0; }
.close-btn {
  background: none; border: none; font-size: 1.25rem; color: var(--color-ink-faint); cursor: pointer; line-height: 1; padding: 0.25rem;
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
