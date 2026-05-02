<script setup lang="ts">
import { SidePanel, Button, StateText, Tag, Icon } from '@/ui'

export type Upstream = {
  providerId: number
  providerName: string
  upstreamModelName: string
  endpointPaths: string[]
  priority: number
  expandedFromProvider: boolean
  providerDisabled: boolean
  entryDisabled: boolean
}

defineProps<{ modelName: string; modelDisabled: boolean; upstreams: Upstream[] }>()
const emit = defineEmits<{ close: [] }>()
</script>

<template>
  <SidePanel :title="modelName" kicker="上游" @close="emit('close')">
    <section class="flex flex-col gap-2">
      <div class="flex items-baseline justify-between">
        <span class="text-xs font-medium text-ink-muted uppercase tracking-[0.03em]">上游列表</span>
        <span class="text-xs text-ink-faint tabular-nums">{{ upstreams.length }}</span>
      </div>
      <div v-if="modelDisabled" class="px-2.5 py-1.5 border border-line rounded-md bg-surface-50 flex items-center gap-1.5 text-xs text-ink-muted">
        <Icon name="eye-off" :size="13" />
        <span>此模型已禁用，所有上游均不参与调度</span>
      </div>
      <StateText v-if="!upstreams.length" compact>该模型暂无上游</StateText>
      <ul v-else class="list-none m-0 p-0 flex flex-col gap-2">
        <li
          v-for="(u, i) in upstreams"
          :key="`${u.providerId}:${i}`"
          class="flex flex-col gap-1.5 px-2.5 py-2 border border-line rounded-md bg-surface-0"
          :class="(u.providerDisabled || u.entryDisabled) ? 'opacity-55' : ''"
        >
          <div class="flex items-center gap-1.5 flex-wrap">
            <span class="text-sm font-semibold text-ink">{{ u.providerName }}</span>
            <Tag v-if="u.providerDisabled" variant="muted">渠道已禁用</Tag>
            <Icon name="chevron-down" :size="12" class="-rotate-90 text-ink-faint" />
            <Tag variant="accent">{{ u.upstreamModelName }}</Tag>
            <Tag v-if="u.priority > 0" variant="more">P{{ u.priority }}</Tag>
            <Tag v-if="u.entryDisabled" variant="muted">上游已禁用</Tag>
          </div>
        </li>
      </ul>
    </section>

    <template #footer>
      <Button variant="ghost" @click="emit('close')">关闭</Button>
    </template>
  </SidePanel>
</template>
