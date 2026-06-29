<script setup lang="ts">
import { computed, ref } from 'vue'
import { useMutation, useQuery, useQueryClient } from '@tanstack/vue-query'
import type { ModelView, PricingMatchCandidate } from '@/api'
import { invalidateModels, matchPricing, upsertModel } from '@/api/client'
import { queryKeys } from '@/api/queryKeys'
import { Button, DataTable, SidePanel, StateText, Td, Th, Tr, Icon } from '@/ui'

const props = defineProps<{
  model: ModelView
  onSave?: () => void
}>()

const emit = defineEmits<{ close: [] }>()

const queryClient = useQueryClient()
const error = ref('')
const selectedIndex = ref(0)
const candidatesQuery = useQuery({
  queryKey: queryKeys.pricingMatches.model(props.model.name),
  queryFn: () => matchPricing(props.model.name),
})
const saveMutation = useMutation({
  mutationFn: upsertModel,
  onSuccess: () => invalidateModels(queryClient),
})
const candidates = computed<PricingMatchCandidate[]>(() => candidatesQuery.data.value ?? [])
const loading = computed(() => candidatesQuery.isLoading.value || candidatesQuery.isFetching.value)
const saving = computed(() => saveMutation.isPending.value)

const selected = computed(() => candidates.value[selectedIndex.value] ?? null)

type DiffSegment = {
  text: string
  kind: 'same' | 'insert' | 'delete'
}

function diffModelName(candidateName: string): DiffSegment[] {
  const from = Array.from(props.model.name)
  const to = Array.from(candidateName)
  const width = to.length + 1
  const dp = Array((from.length + 1) * width).fill(0) as number[]
  const at = (i: number, j: number) => dp[i * width + j] ?? 0
  const set = (i: number, j: number, value: number) => {
    dp[i * width + j] = value
  }

  for (let i = 0; i <= from.length; i++) set(i, 0, i)
  for (let j = 0; j <= to.length; j++) set(0, j, j)

  for (let i = 1; i <= from.length; i++) {
    for (let j = 1; j <= to.length; j++) {
      if (from[i - 1] === to[j - 1]) {
        set(i, j, at(i - 1, j - 1))
      } else {
        set(i, j, Math.min(at(i - 1, j), at(i, j - 1)) + 1)
      }
    }
  }

  const out: DiffSegment[] = []
  let i = from.length
  let j = to.length
  while (i > 0 || j > 0) {
    const fromChar = i > 0 ? from[i - 1] : undefined
    const toChar = j > 0 ? to[j - 1] : undefined
    if (fromChar !== undefined && toChar !== undefined && fromChar === toChar) {
      pushDiff(out, fromChar, 'same')
      i--
      j--
    } else if (toChar !== undefined && (i === 0 || at(i, j - 1) <= at(i - 1, j))) {
      pushDiff(out, toChar, 'insert')
      j--
    } else if (fromChar !== undefined) {
      pushDiff(out, fromChar, 'delete')
      i--
    } else {
      break
    }
  }

  return out.reverse()
}

function pushDiff(out: DiffSegment[], text: string, kind: DiffSegment['kind']) {
  const last = out[out.length - 1]
  if (last?.kind === kind) {
    last.text = text + last.text
    return
  }
  out.push({ text, kind })
}

async function load() {
  error.value = ''
  try {
    const res = await candidatesQuery.refetch()
    if (res.error) throw res.error
    selectedIndex.value = candidates.value.length ? 0 : -1
  } catch (e: unknown) {
    error.value = e instanceof Error ? e.message : '匹配价格失败'
  }
}

async function save() {
  if (!selected.value) return
  error.value = ''
  const body = {
    name: props.model.name,
    disabled: props.model.disabled ?? false,
    annotations: props.model.annotations ?? {},
    pricing: selected.value.pricing,
  }
  try {
    await saveMutation.mutateAsync(body)
    props.onSave?.()
    emit('close')
  } catch (e: unknown) {
    error.value = e instanceof Error ? e.message : '保存价格失败'
  }
}
</script>

<template>
  <SidePanel :title="model.name" kicker="匹配价格" @close="emit('close')">
    <section class="flex flex-col gap-2">
      <div class="flex items-baseline justify-between">
        <span class="text-xs font-medium text-ink-muted uppercase tracking-[0.03em]">候选</span>
        <span class="text-xs text-ink-faint tabular-nums">{{ candidates.length }}</span>
      </div>

      <StateText v-if="loading" compact>匹配中…</StateText>
      <StateText v-else-if="!candidates.length" compact>没有找到可用价格候选</StateText>
      <DataTable v-else>
        <thead>
          <tr>
            <Th>模型</Th>
            <Th>供应商</Th>
          </tr>
        </thead>
        <tbody>
          <Tr
            v-for="(candidate, idx) in candidates"
            :key="`${candidate.providerId}:${candidate.modelId}`"
            :selected="idx === selectedIndex"
            class="cursor-pointer"
            @click="selectedIndex = idx"
          >
            <Td>
              <div class="font-mono text-xs text-ink whitespace-normal break-all leading-[1.55]">
                <span
                  v-for="(segment, segmentIndex) in diffModelName(candidate.modelId)"
                  :key="segmentIndex"
                  :class="[
                    segment.kind === 'insert' ? 'bg-ok-faint text-ok-ink px-0.5 rounded-xs' : '',
                    segment.kind === 'delete'
                      ? 'bg-err-faint text-err-ink line-through px-0.5 rounded-xs'
                      : '',
                  ]"
                  >{{ segment.text }}</span
                >
              </div>
            </Td>
            <Td>
              <span class="font-medium">{{ candidate.providerName }}</span>
            </Td>
          </Tr>
        </tbody>
      </DataTable>
    </section>

    <template v-if="error" #error>{{ error }}</template>

    <template #footer>
      <Button variant="ghost" @click="emit('close')">取消</Button>
      <Button variant="ghost" :disabled="loading || saving" @click="load">
        <Icon
          :name="loading ? 'loader' : 'refresh'"
          :size="13"
          :class="loading ? 'animate-spin' : ''"
        />
        <span>重新匹配</span>
      </Button>
      <Button :disabled="loading || saving || !selected" @click="save">
        {{ saving ? '保存中…' : '保存价格' }}
      </Button>
    </template>
  </SidePanel>
</template>
