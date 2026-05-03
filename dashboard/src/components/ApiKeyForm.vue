<script setup lang="ts">
import { ref } from 'vue'
import { useApi } from '@/composables/useApi'
import AnnotationsEditor from '@/components/AnnotationsEditor.vue'
import { SidePanel, Button, IconButton, Input, Field, Icon } from '@/ui'
import type { ApiKeyView } from '@/api'

const emit = defineEmits<{ close: [] }>()
const props = defineProps<{ apiKey?: ApiKeyView; onSave?: () => void }>()
const api = useApi()

const isEdit = !!props.apiKey
const form = ref({
  name: props.apiKey?.name ?? '',
  key: props.apiKey?.key ?? generateKey(),
  disabled: props.apiKey?.disabled ?? false,
  annotations: { ...props.apiKey?.annotations } as Record<string, string>,
})
const saving = ref(false)
const error = ref('')

function generateKey(): string {
  const buf = new Uint8Array(16)
  crypto.getRandomValues(buf)
  const hex = Array.from(buf, (b) => b.toString(16).padStart(2, '0')).join('')
  return `sk_pt_${hex}`
}

function regenerate() {
  form.value.key = generateKey()
}

async function submit() {
  saving.value = true
  error.value = ''
  const body = {
    name: form.value.name,
    key: form.value.key,
    disabled: form.value.disabled,
    annotations: form.value.annotations,
  }
  if (isEdit) {
    const { error: err } = await api.PUT('/api/picotera/api-keys/{id}', {
      params: { path: { id: props.apiKey!.id } },
      body,
    })
    if (err) error.value = err.message ?? '操作失败'
    else {
      props.onSave?.()
      emit('close')
    }
  } else {
    const { error: err } = await api.POST('/api/picotera/api-keys', { body })
    if (err) error.value = err.message ?? '操作失败'
    else {
      props.onSave?.()
      emit('close')
    }
  }
  saving.value = false
}
</script>

<template>
  <SidePanel
    :title="isEdit ? form.name || 'API Key' : '新增 API Key'"
    :kicker="isEdit ? '编辑 API Key' : 'API Key'"
    @close="emit('close')"
  >
    <form id="api-key-form" class="flex flex-col gap-4" @submit.prevent="submit">
      <Field label="名称">
        <Input v-model="form.name" required placeholder="例如 internal-cli" />
      </Field>
      <Field label="Key">
        <div class="flex items-center gap-2">
          <Input v-model="form.key" required placeholder="sk_pt_..." class="font-mono" />
          <IconButton type="button" title="随机生成" aria-label="随机生成" @click="regenerate">
            <Icon name="refresh" :size="13" />
          </IconButton>
        </div>
      </Field>
      <Field label="状态" as="div">
        <label class="inline-flex items-center gap-2 text-sm cursor-pointer">
          <input v-model="form.disabled" type="checkbox" class="cursor-pointer" />
          <span>禁用此 Key（拒绝该 Key 的网关请求）</span>
        </label>
      </Field>
      <Field label="标注" as="div">
        <AnnotationsEditor v-model="form.annotations" />
      </Field>
    </form>

    <template v-if="error" #error>{{ error }}</template>

    <template #footer>
      <Button variant="ghost" @click="emit('close')">取消</Button>
      <Button type="submit" form="api-key-form" :disabled="saving">
        {{ saving ? '保存中…' : isEdit ? '更新' : '创建' }}
      </Button>
    </template>
  </SidePanel>
</template>
