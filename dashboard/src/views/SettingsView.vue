<script setup lang="ts">
import { ref, watch } from 'vue'
import { useMutation, useQuery, useQueryClient } from '@tanstack/vue-query'
import { getUserSetting, upsertUserSetting, invalidateUserSettings } from '@/api/client'
import { queryKeys } from '@/api/queryKeys'
import { Button, Field, StateText } from '@/ui'

const queryClient = useQueryClient()

const autoCreateProjects = ref(false)
const saved = ref(false)

const autoCreateQuery = useQuery({
  queryKey: queryKeys.userSettings.detail('project.autoCreate'),
  queryFn: () => getUserSetting('project.autoCreate'),
  retry: false,
  // If the setting doesn't exist (404), return null instead of throwing.
  throwOnError: false,
})

watch(
  () => autoCreateQuery.data.value,
  (data) => {
    autoCreateProjects.value = data?.value === true
  },
  { immediate: true },
)

const saveMutation = useMutation({
  mutationFn: async () => {
    await upsertUserSetting({
      key: 'project.autoCreate',
      value: autoCreateProjects.value,
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
    <StateText v-if="autoCreateQuery.isLoading.value">加载中…</StateText>
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
      <div class="flex items-center gap-3">
        <Button :disabled="saveMutation.isPending.value" @click="saveMutation.mutateAsync()">
          {{ saveMutation.isPending.value ? '保存中…' : '保存' }}
        </Button>
        <span v-if="saved" class="text-sm text-ok">已保存</span>
      </div>
    </template>
  </div>
</template>
