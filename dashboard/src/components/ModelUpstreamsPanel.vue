<script setup lang="ts">
import { SidePanel, Button, StateText, Tag, TagList, Icon } from '@/ui'

export type Upstream = {
  providerId: number
  providerName: string
  upstreamModelName: string
  endpointPaths: string[]
  priority: number
  expandedFromProvider: boolean
}

defineProps<{ modelName: string; upstreams: Upstream[] }>()
const emit = defineEmits<{ close: [] }>()
</script>

<template>
  <SidePanel :title="modelName" kicker="上游" @close="emit('close')">
    <section class="flex flex-col gap-2">
      <div class="flex items-baseline justify-between">
        <span class="text-xs font-medium text-ink-muted uppercase tracking-[0.03em]">上游列表</span>
        <span class="text-xs text-ink-faint tabular-nums">{{ upstreams.length }}</span>
      </div>
      <StateText v-if="!upstreams.length" compact>该模型暂无上游</StateText>
      <ul v-else class="list-none m-0 p-0 flex flex-col gap-2">
        <li
          v-for="u in upstreams"
          :key="u.providerId"
          class="flex flex-col gap-1.5 px-2.5 py-2 border border-line rounded-md bg-surface-0"
        >
          <div class="flex items-center gap-1.5 flex-wrap">
            <span class="text-sm font-semibold text-ink">{{ u.providerName }}</span>
            <Icon name="chevron-down" :size="12" class="-rotate-90 text-ink-faint" />
            <Tag variant="accent">{{ u.upstreamModelName }}</Tag>
            <Tag v-if="u.priority > 0" variant="more">P{{ u.priority }}</Tag>
          </div>
          <div class="flex items-center gap-1.5 flex-wrap">
            <TagList v-if="u.endpointPaths.length">
              <Tag v-for="path in u.endpointPaths" :key="path">{{ path }}</Tag>
            </TagList>
            <span v-else class="text-2xs text-ink-faint">无端点</span>
            <span v-if="u.expandedFromProvider && u.endpointPaths.length" class="text-2xs text-ink-faint">
              全部端点
            </span>
          </div>
        </li>
      </ul>
    </section>

    <template #footer>
      <Button variant="ghost" @click="emit('close')">关闭</Button>
    </template>
  </SidePanel>
</template>
