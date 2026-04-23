<script setup lang="ts">
import { ref } from 'vue'
import { useApi } from '@/composables/useApi'
import AnnotationsEditor from '@/components/AnnotationsEditor.vue'
import ModelListEditor from '@/components/ModelListEditor.vue'
import SidePanel from '@/components/SidePanel.vue'
import type { ProviderView } from '@/api'

const emit = defineEmits<{ close: [] }>()
const props = defineProps<{ provider?: ProviderView; onSave?: () => void }>()
const api = useApi()

const isEdit = !!props.provider
const form = ref({
  name: props.provider?.name ?? '',
  credentials: props.provider?.credentials ?? '',
  priority: props.provider?.priority ?? 0,
  providerModels: [...(props.provider?.providerModels ?? [])] as string[],
  annotations: { ...(props.provider?.annotations ?? {}) } as Record<string, string>,
})
const saving = ref(false)
const error = ref('')

async function submit() {
  saving.value = true
  error.value = ''
  const body = {
    id: props.provider?.id ?? 0,
    name: form.value.name,
    credentials: form.value.credentials,
    priority: form.value.priority,
    providerModels: form.value.providerModels,
    annotations: form.value.annotations,
  }
  const { error: err } = await api.PUT('/api/picotera/providers', { body })
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
    :title="isEdit ? (form.name || '渠道') : '新增渠道'"
    :kicker="isEdit ? '编辑渠道' : '渠道'"
    @close="emit('close')"
  >
    <form id="provider-form" class="form-body" @submit.prevent="submit">
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
      <div class="field">
        <span class="field-label">模型列表</span>
        <ModelListEditor v-model="form.providerModels" />
      </div>
      <div class="field">
        <span class="field-label">标注</span>
        <AnnotationsEditor v-model="form.annotations" />
      </div>
    </form>

    <template v-if="error" #error>{{ error }}</template>

    <template #footer>
      <button type="button" class="btn-ghost" @click="emit('close')">取消</button>
      <button type="submit" form="provider-form" class="btn-primary" :disabled="saving">
        {{ saving ? '保存中…' : isEdit ? '更新' : '创建' }}
      </button>
    </template>
  </SidePanel>
</template>

<style scoped>
.form-body { display: flex; flex-direction: column; gap: 1rem; }
</style>
