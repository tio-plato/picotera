<script setup lang="ts">
import { computed } from 'vue'
import { StateText } from '@/ui'
import type { LogEntry } from './artifactTypes'
import { useArtifact } from '@/composables/useArtifact'

const props = defineProps<{ url?: string }>()

const artifactQuery = useArtifact(() => props.url)
const payload = computed(() => artifactQuery.data.value ?? null)
const loading = computed(() => artifactQuery.isLoading.value)
const error = computed(() => artifactQuery.error.value?.message ?? '')

const logs = computed<LogEntry[]>(() => payload.value?.logs ?? [])

function levelClass(level: string) {
  switch (level) {
    case 'error':
      return 'bg-err-faint text-err-ink'
    case 'warn':
      return 'bg-warn-faint text-warn-ink'
    default:
      return 'bg-surface-50 text-ink-muted'
  }
}

function formatTs(iso: string) {
  const d = new Date(iso)
  if (Number.isNaN(d.getTime())) return iso
  const hh = String(d.getHours()).padStart(2, '0')
  const mm = String(d.getMinutes()).padStart(2, '0')
  const ss = String(d.getSeconds()).padStart(2, '0')
  const ms = String(d.getMilliseconds()).padStart(3, '0')
  return `${hh}:${mm}:${ss}.${ms}`
}
</script>

<template>
  <StateText v-if="!url" :dashed="false" compact>未启用 artifact 记录</StateText>
  <StateText v-else-if="loading" :dashed="false" compact>加载中…</StateText>
  <StateText v-else-if="error" :dashed="false" compact>{{ error }}</StateText>
  <StateText v-else-if="!logs.length" :dashed="false" compact>无日志</StateText>
  <div v-else class="font-mono text-2xs flex flex-col">
    <div
      v-for="(l, i) in logs"
      :key="i"
      class="flex items-start gap-2 py-1.5 border-b border-line-soft last:border-0"
    >
      <span
        class="inline-flex items-center px-1.5 py-0.5 rounded-[5px] uppercase text-2xs leading-[1.2] shrink-0"
        :class="levelClass(l.level)"
        >{{ l.level }}</span
      >
      <span class="text-ink-faint shrink-0 tabular-nums">{{ formatTs(l.ts) }}</span>
      <span class="text-ink whitespace-pre-wrap break-all">{{ l.message }}</span>
    </div>
  </div>
</template>
