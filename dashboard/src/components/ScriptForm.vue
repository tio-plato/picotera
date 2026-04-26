<script setup lang="ts">
import { ref } from 'vue'
import { useApi } from '@/composables/useApi'
import { SidePanel, Button, Input, Field, Textarea } from '@/ui'
import type { ScriptView } from '@/api'

const emit = defineEmits<{ close: [] }>()
const props = defineProps<{ script?: ScriptView; onSave?: () => void }>()
const api = useApi()

const isEdit = !!props.script
const form = ref({
  name: props.script?.name ?? '',
  source: props.script?.source ?? '',
  enabled: props.script?.enabled ?? true,
})
const saving = ref(false)
const error = ref('')

async function submit() {
  saving.value = true
  error.value = ''
  if (isEdit) {
    const { error: err } = await api.PUT('/api/picotera/scripts/{id}', {
      params: { path: { id: props.script!.id } },
      body: { name: form.value.name, source: form.value.source, enabled: form.value.enabled },
    })
    if (err) error.value = err.message ?? '操作失败'
    else { props.onSave?.(); emit('close') }
  } else {
    const { error: err } = await api.POST('/api/picotera/scripts', {
      body: { name: form.value.name, source: form.value.source, enabled: form.value.enabled },
    })
    if (err) error.value = err.message ?? '操作失败'
    else { props.onSave?.(); emit('close') }
  }
  saving.value = false
}
</script>

<template>
  <SidePanel
    :title="isEdit ? (form.name || '脚本') : '新增脚本'"
    :kicker="isEdit ? '编辑脚本' : '脚本'"
    @close="emit('close')"
  >
    <form id="script-form" class="flex flex-col gap-4" @submit.prevent="submit">
      <Field v-if="isEdit" label="ID">
        <Input :model-value="props.script!.id" readonly />
      </Field>
      <Field label="名称">
        <Input v-model="form.name" required placeholder="例如 reverse-providers" />
      </Field>
      <Field label="启用">
        <label class="inline-flex items-center gap-2">
          <input v-model="form.enabled" type="checkbox" class="rounded border-line" />
          <span class="text-sm text-ink-muted">enabled</span>
        </label>
      </Field>
      <Field label="源代码">
        <Textarea v-model="form.source" :rows="20" class="font-mono text-sm" required />
      </Field>
    </form>

    <template v-if="error" #error>{{ error }}</template>

    <template #footer>
      <Button variant="ghost" @click="emit('close')">取消</Button>
      <Button type="submit" form="script-form" :disabled="saving">
        {{ saving ? '保存中…' : isEdit ? '更新' : '创建' }}
      </Button>
    </template>
  </SidePanel>
</template>
