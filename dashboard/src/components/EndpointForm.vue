<script setup lang="ts">
import { ref, computed, watch } from 'vue'
import { useMutation, useQueryClient } from '@tanstack/vue-query'
import { SidePanel, Button, Input, Select, Field } from '@/ui'
import type { EndpointView } from '@/api'
import { ENDPOINT_TYPE_LABELS } from '@/api'
import type { EndpointType } from '@/api'
import { invalidateEndpoints, upsertEndpoint } from '@/api/client'

const emit = defineEmits<{ close: [] }>()
const props = defineProps<{ endpoint?: EndpointView; onSave?: () => void }>()
const queryClient = useQueryClient()

const isEdit = !!props.endpoint
const form = ref({
  name: props.endpoint?.name ?? '',
  path: props.endpoint?.path ?? '',
  modelPath: props.endpoint?.modelPath ?? '',
  credentialsResolver: props.endpoint?.credentialsResolver ?? ('generalApiKey' as const),
  endpointType: (props.endpoint?.endpointType ?? 'general') as EndpointType,
})
const saving = ref(false)
const error = ref('')
const saveMutation = useMutation({
  mutationFn: upsertEndpoint,
  onSuccess: () => invalidateEndpoints(queryClient),
})

const isModelPathLocked = computed(() => form.value.endpointType === 'exaSearch')
watch(
  () => form.value.endpointType,
  (t) => {
    if (t === 'exaSearch') form.value.modelPath = ''
  },
)

const endpointTypeOptions = computed(() => {
  const entries = Object.entries(ENDPOINT_TYPE_LABELS).filter(([k]) => k !== 'unknown') as [EndpointType, string][]
  if (form.value.endpointType === 'unknown') entries.push(['unknown', ENDPOINT_TYPE_LABELS.unknown])
  return entries
})

async function submit() {
  saving.value = true
  error.value = ''
  try {
    await saveMutation.mutateAsync(form.value)
    props.onSave?.()
    emit('close')
  } catch (e: unknown) {
    error.value = e instanceof Error ? e.message : '操作失败'
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
      <Field label="类型">
        <Select v-model="form.endpointType">
          <option v-for="[value, label] in endpointTypeOptions" :key="value" :value="value">{{ label }}</option>
        </Select>
      </Field>
      <Field label="模型字段路径">
        <Input
          v-model="form.modelPath"
          :disabled="isModelPathLocked"
          :placeholder="isModelPathLocked ? 'Exa 搜索端点不解析模型' : '可选，留空表示该端点不解析模型'"
        />
      </Field>
      <Field label="凭证解析">
        <Select v-model="form.credentialsResolver">
          <option value="generalApiKey">通用密钥</option>
          <option value="bearerToken">Bearer Token</option>
          <option value="xApiKey">X-Api-Key</option>
          <option value="searchKey">Search Key (?key=)</option>
          <option value="googApiKey">X-Goog-Api-Key</option>
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
