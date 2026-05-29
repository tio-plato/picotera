<script setup lang="ts">
import { ref, watch } from 'vue'
import { useMutation, useQueryClient } from '@tanstack/vue-query'
import { getGlobalSetting, upsertGlobalSetting, invalidateGlobalSettings } from '@/api/client'
import { useAppTitle } from '@/composables/useAppTitle'
import { Button, Field, Input, StateText } from '@/ui'

const queryClient = useQueryClient()
const { appTitle, query } = useAppTitle()

const titleInput = ref('')
const saved = ref(false)

// Populate input when query resolves.
watch(
  () => query.data.value,
  (data) => {
    if (data) {
      const val = data.value
      titleInput.value = typeof val === 'string' ? val : ''
    }
  },
  { immediate: true },
)

const saveMutation = useMutation({
  mutationFn: async () => {
    const value = titleInput.value.trim()
    await upsertGlobalSetting({
      key: 'app.title',
      value,
    })
  },
  onSuccess: () => {
    invalidateGlobalSettings(queryClient)
    saved.value = true
    setTimeout(() => {
      saved.value = false
    }, 2000)
  },
})
</script>

<template>
  <div class="flex flex-col gap-6 max-w-md">
    <StateText v-if="query.isLoading.value">加载中…</StateText>
    <template v-else>
      <Field label="应用标题">
        <Input v-model="titleInput" placeholder="PicoTera" />
        <p class="text-xs text-ink-faint mt-1">
          设置后将替换侧边栏和浏览器标签页中显示的名称。留空则使用默认值「PicoTera」。
        </p>
      </Field>
      <div class="flex items-center gap-3">
        <Button :disabled="saveMutation.isPending.value" @click="saveMutation.mutateAsync()">
          {{ saveMutation.isPending.value ? '保存中…' : '保存' }}
        </Button>
        <span v-if="saved" class="text-sm text-ok">已保存</span>
      </div>
    </template>
  </div>
</template>
