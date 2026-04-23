<script setup lang="ts">
import { ref } from 'vue'
import { useApi } from '@/composables/useApi'
import SidePanel from '@/components/SidePanel.vue'
import type { EndpointView } from '@/api'

const emit = defineEmits<{ close: [] }>()
const props = defineProps<{ endpoint?: EndpointView; onSave?: () => void }>()
const api = useApi()

const isEdit = !!props.endpoint
const form = ref({
  name: props.endpoint?.name ?? '',
  path: props.endpoint?.path ?? '',
  modelPath: props.endpoint?.modelPath ?? '',
  credentialsResolver: props.endpoint?.credentialsResolver ?? ('generalApiKey' as const),
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
  <SidePanel
    :title="isEdit ? (form.name || form.path || '端点') : '新增端点'"
    :kicker="isEdit ? '编辑端点' : '端点'"
    @close="emit('close')"
  >
    <form id="endpoint-form" class="form-body" @submit.prevent="submit">
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
    </form>

    <template v-if="error" #error>{{ error }}</template>

    <template #footer>
      <button type="button" class="btn-ghost" @click="emit('close')">取消</button>
      <button type="submit" form="endpoint-form" class="btn-primary" :disabled="saving">
        {{ saving ? '保存中…' : isEdit ? '更新' : '创建' }}
      </button>
    </template>
  </SidePanel>
</template>

<style scoped>
.form-body { display: flex; flex-direction: column; gap: 1rem; }
</style>
