<script setup lang="ts">
import { ref } from 'vue'
import { useApi } from '@/composables/useApi'
import { SidePanel, Button, Input, Select, Field } from '@/ui'
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
    <form id="endpoint-form" class="flex flex-col gap-4" @submit.prevent="submit">
      <Field label="路径">
        <Input v-model="form.path" required placeholder="例如 /api/v1/chat/completions" :disabled="isEdit" />
      </Field>
      <Field label="名称">
        <Input v-model="form.name" required placeholder="例如 Chat Completions" />
      </Field>
      <Field label="模型字段路径">
        <Input v-model="form.modelPath" required placeholder="例如 body.model" />
      </Field>
      <Field label="凭证解析">
        <Select v-model="form.credentialsResolver">
          <option value="generalApiKey">通用 API Key</option>
          <option value="bearerToken">Bearer Token</option>
          <option value="xApiKey">X-Api-Key</option>
        </Select>
      </Field>
    </form>

    <template v-if="error" #error>{{ error }}</template>

    <template #footer>
      <Button variant="ghost" @click="emit('close')">取消</Button>
      <Button type="submit" form="endpoint-form" :disabled="saving">
        {{ saving ? '保存中…' : isEdit ? '更新' : '创建' }}
      </Button>
    </template>
  </SidePanel>
</template>
