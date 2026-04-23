<script setup lang="ts">
import { ref } from 'vue'
import { useApi } from '@/composables/useApi'
import SidePanel from '@/components/SidePanel.vue'
import type { ModelView } from '@/api'

const emit = defineEmits<{ close: [] }>()
const props = defineProps<{ model?: ModelView; onSave?: () => void }>()
const api = useApi()

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
  <SidePanel
    :title="isEdit ? (form.title || form.name || '模型') : '新增模型'"
    :kicker="isEdit ? '编辑模型' : '模型'"
    @close="emit('close')"
  >
    <form id="model-form" class="form-body" @submit.prevent="submit">
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
    </form>

    <template v-if="error" #error>{{ error }}</template>

    <template #footer>
      <button type="button" class="btn-ghost" @click="emit('close')">取消</button>
      <button type="submit" form="model-form" class="btn-primary" :disabled="saving">
        {{ saving ? '保存中…' : isEdit ? '更新' : '创建' }}
      </button>
    </template>
  </SidePanel>
</template>

<style scoped>
.form-body { display: flex; flex-direction: column; gap: 1rem; }
</style>
