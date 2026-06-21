<script setup lang="ts">
import { ref, watch } from 'vue'
import { useMutation, useQuery, useQueryClient } from '@tanstack/vue-query'
import { getUserSetting, upsertUserSetting, invalidateUserSettings } from '@/api/client'
import { queryKeys } from '@/api/queryKeys'
import { Button, Field, SegmentedControl, StateText } from '@/ui'

const queryClient = useQueryClient()

type OtrMode = 'none' | 'body' | 'body-and-message'

const otrOptions: { value: OtrMode; label: string }[] = [
  { value: 'none', label: '完整记录' },
  { value: 'body', label: '不记录 body' },
  { value: 'body-and-message', label: '仅元数据' },
]

const autoCreateProjects = ref(false)
const otr = ref<OtrMode>('none')
const saved = ref(false)

const autoCreateQuery = useQuery({
  queryKey: queryKeys.userSettings.detail('project.autoCreate'),
  queryFn: () => getUserSetting('project.autoCreate'),
  retry: false,
  // If the setting doesn't exist (404), return null instead of throwing.
  throwOnError: false,
})

const otrQuery = useQuery({
  queryKey: queryKeys.userSettings.detail('request.otr'),
  queryFn: () => getUserSetting('request.otr'),
  retry: false,
  throwOnError: false,
})

watch(
  () => autoCreateQuery.data.value,
  (data) => {
    autoCreateProjects.value = data?.value === true
  },
  { immediate: true },
)

watch(
  () => otrQuery.data.value,
  (data) => {
    const value = data?.value
    otr.value = value === 'body' || value === 'body-and-message' ? value : 'none'
  },
  { immediate: true },
)

const saveMutation = useMutation({
  mutationFn: async () => {
    await upsertUserSetting({
      key: 'project.autoCreate',
      value: autoCreateProjects.value,
    })
    await upsertUserSetting({
      key: 'request.otr',
      value: otr.value,
    })
  },
  onSuccess: () => {
    invalidateUserSettings(queryClient)
    saved.value = true
    setTimeout(() => {
      saved.value = false
    }, 2000)
  },
})
</script>

<template>
  <div class="flex flex-col gap-6 max-w-md">
    <StateText v-if="autoCreateQuery.isLoading.value || otrQuery.isLoading.value">加载中…</StateText>
    <template v-else>
      <Field label="项目自动创建">
        <label class="flex items-center gap-2 text-sm">
          <input
            v-model="autoCreateProjects"
            type="checkbox"
            class="size-4 rounded border-line text-accent focus:ring-accent"
          />
          <span>允许自动创建项目</span>
        </label>
        <p class="text-xs text-ink-faint mt-1">
          启用后，当你的网关请求的工作目录未匹配到你名下任何项目时，将以该路径自动创建一个项目。
        </p>
      </Field>
      <Field label="数据记录">
        <SegmentedControl v-model="otr" :options="otrOptions" />
        <p class="text-xs text-ink-faint mt-1">
          控制网关记录哪些数据。完整记录：记录请求体、响应体与用户消息预览。不记录
          body：保留请求头、状态码与各项指标（耗时、token、费用等），但清空请求体、响应体、聚合内容与逐行时序。仅元数据：在「不记录
          body」基础上，额外不记录用户消息预览。可通过请求头 <code>X-PicoTera-OTR</code> 临时覆盖此设置。
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
