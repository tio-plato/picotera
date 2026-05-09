<script setup lang="ts">
import { ref } from 'vue'
import { useMutation, useQueryClient } from '@tanstack/vue-query'
import AnnotationsEditor from '@/components/AnnotationsEditor.vue'
import { SidePanel, Button, Input, Field } from '@/ui'
import type { ProviderView } from '@/api'
import { invalidateProviders, upsertProvider } from '@/api/client'

const emit = defineEmits<{ close: [] }>()
const props = defineProps<{ provider?: ProviderView; onSave?: () => void }>()
const queryClient = useQueryClient()

const isEdit = !!props.provider
const form = ref({
  name: props.provider?.name ?? '',
  credentials: props.provider?.credentials ?? '',
  priority: props.provider?.priority ?? 0,
  annotations: { ...props.provider?.annotations } as Record<string, string>,
  disabled: props.provider?.disabled ?? false,
})
const saving = ref(false)
const error = ref('')
const saveMutation = useMutation({
  mutationFn: upsertProvider,
  onSuccess: () => invalidateProviders(queryClient),
})

async function submit() {
  saving.value = true
  error.value = ''
  const body = {
    id: props.provider?.id ?? 0,
    name: form.value.name,
    credentials: form.value.credentials,
    priority: form.value.priority,
    providerModels: props.provider?.providerModels ?? [],
    annotations: form.value.annotations,
    disabled: form.value.disabled,
  }
  try {
    await saveMutation.mutateAsync(body)
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
    :title="isEdit ? (form.name || '渠道') : '新增渠道'"
    :kicker="isEdit ? '编辑渠道' : '渠道'"
    @close="emit('close')"
  >
    <form id="provider-form" class="flex flex-col gap-4" @submit.prevent="submit">
      <Field label="名称">
        <Input v-model="form.name" required placeholder="例如 openai" />
      </Field>
      <Field label="凭证">
        <Input v-model="form.credentials" required placeholder="密钥或密钥" />
      </Field>
      <Field label="优先级">
        <Input v-model.number="form.priority" type="number" required />
      </Field>
      <Field label="状态" as="div">
        <label class="inline-flex items-center gap-2 text-sm cursor-pointer">
          <input v-model="form.disabled" type="checkbox" class="cursor-pointer" />
          <span>禁用此渠道（不参与调度）</span>
        </label>
      </Field>
      <Field label="标注" as="div">
        <AnnotationsEditor v-model="form.annotations" />
      </Field>
      <p v-if="!isEdit" class="text-xs text-ink-faint">
        保存后请在「模型」面板配置该渠道的模型列表。
      </p>
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
