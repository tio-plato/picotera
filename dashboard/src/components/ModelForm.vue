<script setup lang="ts">
import { ref } from 'vue'
import { useApi } from '@/composables/useApi'
import { SidePanel, Button, Input, Field } from '@/ui'
import PricingEditor from '@/components/PricingEditor.vue'
import AnnotationsEditor from '@/components/AnnotationsEditor.vue'
import type { ModelView, Pricing } from '@/api'

const emit = defineEmits<{ close: [] }>()
const props = defineProps<{
  model?: ModelView
  defaultName?: string
  lockedName?: boolean
  onSave?: () => void
}>()
const api = useApi()

const isEdit = !!props.model
const form = ref({
  name: props.model?.name ?? props.defaultName ?? '',
  title: props.model?.title ?? '',
  developer: props.model?.developer ?? '',
  series: props.model?.series ?? '',
  disabled: props.model?.disabled ?? false,
  pricing: (props.model?.pricing ?? null) as Pricing | null,
  annotations: { ...props.model?.annotations } as Record<string, string>,
})
const saving = ref(false)
const error = ref('')

async function submit() {
  saving.value = true
  error.value = ''
  const body = {
    name: form.value.name,
    title: form.value.title,
    developer: form.value.developer,
    series: form.value.series,
    disabled: form.value.disabled,
    annotations: form.value.annotations,
    ...(form.value.pricing ? { pricing: form.value.pricing } : {}),
  }
  const { error: err } = await api.PUT('/api/picotera/models', { body })
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
    :kicker="isEdit ? '编辑模型' : lockedName ? '新增模型 · 来自上游' : '模型'"
    @close="emit('close')"
  >
    <form id="model-form" class="flex flex-col gap-4" @submit.prevent="submit">
      <Field label="名称">
        <Input
          v-model="form.name"
          required
          placeholder="例如 gpt-4o"
          :disabled="isEdit || lockedName"
        />
      </Field>
      <Field label="标题">
        <Input v-model="form.title" required placeholder="例如 GPT-4o" />
      </Field>
      <Field label="开发者">
        <Input v-model="form.developer" required placeholder="例如 OpenAI" />
      </Field>
      <Field label="系列">
        <Input v-model="form.series" required placeholder="例如 GPT" />
      </Field>
      <Field label="定价" as="div">
        <PricingEditor v-model="form.pricing" />
      </Field>
      <Field label="状态" as="div">
        <label class="inline-flex items-center gap-2 text-sm cursor-pointer">
          <input v-model="form.disabled" type="checkbox" class="cursor-pointer" />
          <span>禁用此模型（不参与调度）</span>
        </label>
      </Field>
      <Field label="标注" as="div">
        <AnnotationsEditor v-model="form.annotations" />
      </Field>
    </form>

    <template v-if="error" #error>{{ error }}</template>

    <template #footer>
      <Button variant="ghost" @click="emit('close')">取消</Button>
      <Button type="submit" form="model-form" :disabled="saving">
        {{ saving ? '保存中…' : isEdit ? '更新' : '创建' }}
      </Button>
    </template>
  </SidePanel>
</template>
