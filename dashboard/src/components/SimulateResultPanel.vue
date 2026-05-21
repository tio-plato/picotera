<script setup lang="ts">
import { computed } from 'vue'
import { SidePanel, StateText, Tag } from '@/ui'
import type { SimulateDispatchResponseBody } from '@/api'

const props = defineProps<{ result: SimulateDispatchResponseBody }>()
const emit = defineEmits<{ close: [] }>()

const candidates = computed(() => props.result.candidates ?? [])
const logs = computed(() => props.result.logs ?? [])
const modelChanged = computed(() => props.result.originalModel !== props.result.resolvedModel)
</script>

<template>
  <SidePanel
    title="模拟结果"
    kicker="dispatch"
    :subtitle="`${candidates.length} 个候选项${logs.length > 0 ? ` · ${logs.length} 条日志` : ''}`"
    @close="emit('close')"
  >
    <div class="grid grid-cols-3 gap-3 text-xs">
      <div>
        <div class="text-ink-faint">模型</div>
        <div class="font-mono">
          <span v-if="modelChanged">{{ result.originalModel }} → {{ result.resolvedModel }}</span>
          <span v-else>{{ result.resolvedModel }}</span>
        </div>
      </div>
      <div>
        <div class="text-ink-faint">源格式</div>
        <div class="font-mono break-all">{{ result.sourceFormat }}</div>
      </div>
      <div>
        <div class="text-ink-faint">流式</div>
        <div class="font-mono">{{ result.stream ? 'true' : 'false' }}</div>
      </div>
    </div>

    <div class="flex flex-col gap-2">
      <div
        v-for="(c, idx) in candidates"
        :key="`${c.provider.id}|${c.mpe.endpointPath}`"
        class="border border-line rounded-md px-3 py-2.5 flex flex-col gap-2"
      >
        <div class="flex items-baseline gap-2 flex-wrap">
          <span class="text-xs text-ink-faint tabular-nums">{{ idx + 1 }}.</span>
          <span class="font-medium text-ink">{{ c.provider.name }}</span>
          <span class="text-xs text-ink-faint">#{{ c.provider.id }}</span>
          <span class="font-mono text-xs text-ink-muted break-all">{{ c.mpe.endpointPath }}</span>
          <Tag v-if="c.bridged" variant="accent">桥接 → {{ c.upstreamFormat }}</Tag>
          <Tag v-else variant="muted">{{ c.upstreamFormat }}</Tag>
          <Tag v-if="c.provider.disabled" variant="more">已禁用</Tag>
        </div>
        <div class="flex gap-x-6 gap-y-1 text-xs">
          <div class="min-w-0 flex-1">
            <div class="text-ink-faint">上游模型</div>
            <div class="font-mono break-all">{{ c.mpe.upstreamModelName || c.mpe.modelName }}</div>
          </div>
          <div v-if="c.bridged && c.outboundProfile" class="min-w-0 flex-1">
            <div class="text-ink-faint">桥接适配器</div>
            <div class="font-mono break-all">{{ c.outboundProfile.type }}</div>
          </div>
        </div>
        <div class="flex gap-x-6 gap-y-1 text-xs">
          <div class="flex-1">
            <div class="text-ink-faint">渠道优先级</div>
            <div class="font-mono tabular-nums">{{ c.provider.priority }}</div>
          </div>
          <div class="flex-1">
            <div class="text-ink-faint">模型优先级</div>
            <div class="font-mono tabular-nums">{{ c.mpe.priority }}</div>
          </div>
          <div class="flex-1">
            <div class="text-ink-faint">合并优先级</div>
            <div class="font-mono tabular-nums">{{ c.mpe.priority + c.provider.priority }}</div>
          </div>
        </div>
        <details v-if="c.bridged && c.outboundProfile" class="text-xs">
          <summary class="cursor-pointer text-ink-muted">
            桥接配置 ({{ Object.keys(c.outboundProfile.config ?? {}).length }})
          </summary>
          <pre
            v-if="Object.keys(c.outboundProfile.config ?? {}).length > 0"
            class="mt-1.5 font-mono text-2xs text-ink whitespace-pre-wrap break-all m-0 bg-surface-50 border border-line rounded-md px-2.5 py-1.5"
            >{{ JSON.stringify(c.outboundProfile.config, null, 2) }}</pre
          >
          <div v-else class="mt-1.5 text-ink-faint font-mono">{}</div>
        </details>
        <details v-if="Object.keys(c.mergedAnnotations).length > 0" class="text-xs">
          <summary class="cursor-pointer text-ink-muted">
            注解 ({{ Object.keys(c.mergedAnnotations).length }})
          </summary>
          <div class="mt-1.5 grid grid-cols-[auto_1fr] gap-x-3 gap-y-0.5 font-mono">
            <template v-for="(v, k) in c.mergedAnnotations" :key="k">
              <span class="text-ink-faint">{{ k }}</span>
              <span class="break-all">{{ v }}</span>
            </template>
          </div>
        </details>
      </div>
      <StateText v-if="candidates.length === 0">本次模拟未返回任何候选项。</StateText>
    </div>

    <div v-if="logs.length > 0" class="flex flex-col gap-1.5">
      <div class="text-2xs font-medium text-ink-muted uppercase tracking-[0.03em]">脚本日志</div>
      <ul class="flex flex-col gap-1 font-mono text-xs border border-line rounded-md px-3 py-2">
        <li
          v-for="(log, i) in logs"
          :key="i"
          class="grid grid-cols-[auto_auto_1fr] gap-2 leading-snug"
        >
          <span class="text-ink-faint tabular-nums">{{ log.ts.slice(11, 23) }}</span>
          <span
            class="uppercase shrink-0"
            :class="
              log.level === 'error'
                ? 'text-err'
                : log.level === 'warn'
                  ? 'text-warn-ink'
                  : 'text-ink-muted'
            "
            >{{ log.level }}</span
          >
          <span class="break-all whitespace-pre-wrap text-ink">{{ log.message }}</span>
        </li>
      </ul>
    </div>
  </SidePanel>
</template>
