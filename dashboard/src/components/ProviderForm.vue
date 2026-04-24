<script setup lang="ts">
import { ref } from 'vue'
import { useApi } from '@/composables/useApi'
import AnnotationsEditor from '@/components/AnnotationsEditor.vue'
import ModelListEditor from '@/components/ModelListEditor.vue'
import { SidePanel, Button, Input, Field } from '@/ui'
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
    <form id="provider-form" class="flex flex-col gap-4" @submit.prevent="submit">
      <Field label="名称">
        <Input v-model="form.name" required placeholder="例如 openai" />
      </Field>
      <Field label="凭证">
        <Input v-model="form.credentials" required placeholder="API Key 或密钥" />
      </Field>
      <Field label="优先级">
        <Input v-model.number="form.priority" type="number" required />
      </Field>
      <Field label="模型列表" as="div">
        <ModelListEditor v-model="form.providerModels" />
      </Field>
      <Field label="标注" as="div">
        <AnnotationsEditor v-model="form.annotations" />
      </Field>
    </form>

    <template v-if="error" #error>{{ error }}</template>

    <template #footer>
      <Button variant="ghost" @click="emit('close')">取消</Button>
      <Button type="submit" form="provider-form" :disabled="saving">
        {{ saving ? '保存中…' : isEdit ? '更新' : '创建' }}
      </Button>
    </template>
  </SidePanel>
</template>
