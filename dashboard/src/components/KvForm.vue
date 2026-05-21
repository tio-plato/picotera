<script setup lang="ts">
import { ref } from 'vue'
import { useMutation, useQueryClient } from '@tanstack/vue-query'
import { SidePanel, Button, Input, Field, CodeEditor } from '@/ui'
import type { KvEntryView } from '@/api'
import { upsertKvEntry, invalidateKv } from '@/api/client'

const emit = defineEmits<{ close: [] }>()
const props = defineProps<{ entry?: KvEntryView; onSave?: () => void }>()
const queryClient = useQueryClient()

const isEdit = !!props.entry
const form = ref({
  key: props.entry?.key ?? '',
  value: props.entry?.value ?? '',
  ttlSeconds: props.entry && props.entry.ttl >= 0 ? String(props.entry.ttl) : '',
})
const saving = ref(false)
const error = ref('')
const jsonWarning = ref('')

const saveMutation = useMutation({
  mutationFn: (body: { key: string; value: string; ttlSeconds?: number }) =>
    upsertKvEntry(body.key, { value: body.value, ttlSeconds: body.ttlSeconds }),
  onSuccess: () => invalidateKv(queryClient),
})

function validateJson() {
  jsonWarning.value = ''
  if (!form.value.value.trim()) return
  try {
    JSON.parse(form.value.value)
  } catch {
    jsonWarning.value = '不是合法的 JSON（仅警告，仍可保存）'
  }
}

async function submit() {
  saving.value = true
  error.value = ''
  try {
    const ttl = form.value.ttlSeconds ? Number(form.value.ttlSeconds) : undefined
    await saveMutation.mutateAsync({
      key: form.value.key,
      value: form.value.value,
      ttlSeconds: ttl && ttl > 0 ? ttl : undefined,
    })
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
    :title="isEdit ? form.key || 'KV 条目' : '新增 KV 条目'"
    :kicker="isEdit ? '编辑 KV' : 'KV'"
    @close="emit('close')"
  >
    <form id="kv-form" class="flex flex-col gap-4" @submit.prevent="submit">
      <Field v-if="isEdit" label="Key">
        <Input :model-value="form.key" readonly />
      </Field>
      <Field v-else label="Key">
        <Input v-model="form.key" required placeholder="例如 my-key" />
      </Field>
      <Field label="Value" :error="jsonWarning">
        <CodeEditor
          v-model="form.value"
          language="javascript"
          min-height="200px"
          max-height="50vh"
          @blur="validateJson"
        />
      </Field>
      <Field label="TTL（秒）" help="留空或 0 表示永不过期">
        <Input v-model="form.ttlSeconds" type="number" min="0" placeholder="永不过期" />
      </Field>
    </form>

    <template v-if="error" #error>{{ error }}</template>

    <template #footer>
      <Button variant="ghost" @click="emit('close')">取消</Button>
      <Button type="submit" form="kv-form" :disabled="saving || !form.key">
        {{ saving ? '保存中…' : isEdit ? '更新' : '创建' }}
      </Button>
    </template>
  </SidePanel>
</template>
