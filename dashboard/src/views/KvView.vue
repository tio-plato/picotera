<script setup lang="ts">
import { ref, computed } from 'vue'
import { useMutation, useQuery, useQueryClient } from '@tanstack/vue-query'
import { useConfirm } from '@/composables/useConfirm'
import { useSidePanel } from '@/composables/useSidePanel'
import type { KvEntryView } from '@/api'
import { deleteKvEntry, invalidateKv, listKvEntries } from '@/api/client'
import { queryKeys } from '@/api/queryKeys'
import KvForm from '@/components/KvForm.vue'
import {
  Button,
  IconButton,
  DataCard,
  DataTable,
  Th,
  Td,
  Tr,
  StateText,
  Tag,
  Input,
  Icon,
} from '@/ui'

const panel = useSidePanel()
const confirm = useConfirm()
const queryClient = useQueryClient()

const pattern = ref('')
const cursorStack = ref<number[]>([])
const currentCursor = computed(() => cursorStack.value.length > 0 ? cursorStack.value[cursorStack.value.length - 1] : undefined)

const kvQuery = useQuery({
  queryKey: computed(() => queryKeys.kv.list({ pattern: pattern.value || undefined, cursor: currentCursor.value })),
  queryFn: () => listKvEntries({ pattern: pattern.value || undefined, cursor: currentCursor.value }),
})
const entries = computed(() => kvQuery.data.value?.items ?? [])
const loading = computed(() => kvQuery.isLoading.value)
const hasMore = computed(() => kvQuery.data.value?.hasMore ?? false)
const nextCursor = computed(() => kvQuery.data.value?.nextCursor)

const deleteMutation = useMutation({
  mutationFn: deleteKvEntry,
  onSuccess: () => invalidateKv(queryClient),
})

function openCreate() {
  panel.open(KvForm, {}, { key: 'kv:new', width: '600px' })
}

function openEdit(entry: KvEntryView) {
  panel.open(KvForm, { entry }, { key: `kv:${entry.key}`, width: '600px' })
}

function confirmDelete(_event: Event, key: string) {
  confirm.require({
    message: `确定要删除键「${key}」吗？此操作不可撤销。`,
    accept: async () => {
      await deleteMutation.mutateAsync(key)
    },
  })
}

function loadMore() {
  if (nextCursor.value) {
    cursorStack.value.push(Number(nextCursor.value))
  }
}

function goBack() {
  cursorStack.value.pop()
}

function onSearch() {
  cursorStack.value = []
}

function formatTTL(ttl: number): string {
  if (ttl < 0) return ''
  if (ttl === 0) return '0s'
  const m = Math.floor(ttl / 60)
  const s = ttl % 60
  if (m > 0) return `${m}m ${s}s`
  return `${s}s`
}

function truncateValue(value: string, max = 80): string {
  if (value.length <= max) return value
  return value.slice(0, max) + '…'
}
</script>

<template>
  <div class="flex flex-col gap-3.5">
    <div class="flex items-center justify-between gap-3">
      <div class="flex items-center gap-2">
        <span class="text-xs text-ink-faint tabular-nums">{{ entries.length }} 个条目</span>
        <div class="relative">
          <Input
            v-model="pattern"
            placeholder="搜索前缀…"
            class="w-48 pl-8"
            @keydown.enter="onSearch"
          />
          <Icon name="search" :size="14" class="absolute left-2.5 top-1/2 -translate-y-1/2 text-ink-faint" />
        </div>
      </div>
      <div class="flex items-center gap-2">
        <Button @click="openCreate">
          <Icon name="plus" :size="14" :stroke-width="2.2" />
          <span>新增</span>
        </Button>
      </div>
    </div>
    <StateText v-if="loading">加载中…</StateText>
    <DataCard v-else-if="entries.length">
      <DataTable>
        <thead>
          <tr>
            <Th>Key</Th>
            <Th>Value</Th>
            <Th>TTL</Th>
            <Th actions />
          </tr>
        </thead>
        <tbody>
          <Tr
            v-for="e in entries"
            :key="e.key"
            :selected="panel.isActive(`kv:${e.key}`)"
            class="cursor-pointer"
            @click="openEdit(e)"
          >
            <Td>
              <span class="font-mono text-sm font-medium">{{ e.key }}</span>
            </Td>
            <Td>
              <span class="font-mono text-xs text-ink-muted">{{ truncateValue(e.value) }}</span>
            </Td>
            <Td>
              <Tag v-if="e.ttl < 0" variant="muted">永不过期</Tag>
              <span v-else class="text-xs tabular-nums text-ink-muted">{{ formatTTL(e.ttl) }}</span>
            </Td>
            <Td actions>
              <div class="inline-flex gap-1 opacity-55 group-hover:opacity-100 transition-opacity">
                <IconButton
                  :active="panel.isActive(`kv:${e.key}`)"
                  title="编辑"
                  aria-label="编辑"
                  @click.stop="openEdit(e)"
                >
                  <Icon name="edit" :size="13" />
                </IconButton>
                <IconButton
                  variant="danger"
                  title="删除"
                  aria-label="删除"
                  @click.stop="(ev: Event) => confirmDelete(ev, e.key)"
                >
                  <Icon name="trash" :size="13" />
                </IconButton>
              </div>
            </Td>
          </Tr>
        </tbody>
      </DataTable>
      <div v-if="hasMore || cursorStack.length > 0" class="flex items-center justify-between px-4 py-2 border-t border-line">
        <Button v-if="cursorStack.length > 0" variant="ghost" size="sm" @click="goBack">
          <Icon name="arrow-left" :size="14" />
          <span>上一页</span>
        </Button>
        <span v-else />
        <Button v-if="hasMore" variant="ghost" size="sm" @click="loadMore">
          <span>加载更多</span>
        </Button>
      </div>
    </DataCard>
    <StateText v-else>暂无 KV 条目，点击右上角按钮新增</StateText>
  </div>
</template>
