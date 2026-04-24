<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import { useConfirm } from '@/composables/useConfirm'
import { useApi } from '@/composables/useApi'
import type { ModelProviderEndpointView } from '@/api'
import MappingForm from '@/components/MappingForm.vue'
import { useSidePanel } from '@/composables/useSidePanel'
import {
  Button,
  IconButton,
  DataCard,
  DataTable,
  Th,
  Td,
  Tr,
  StateText,
  Badge,
  Tag,
  TagList,
  Icon,
} from '@/ui'

const panel = useSidePanel()
const confirm = useConfirm()
const api = useApi()

const mappings = ref<ModelProviderEndpointView[]>([])
const loading = ref(true)
const hasMore = ref(false)
const nextCursor = ref('')
const count = computed(() => mappings.value.length)

async function fetchMappings(cursor?: string) {
  loading.value = true
  const { data, error } = await api.GET('/api/picotera/model-provider-endpoints', {
    params: { query: { limit: 50, cursor: cursor || undefined } },
  })
  if (!error && data) {
    const body = data
    if (cursor) {
      mappings.value.push(...(body.items ?? []))
    } else {
      mappings.value = body.items ?? []
    }
    hasMore.value = body.pagination.hasMore
    nextCursor.value = body.pagination.nextCursor ?? ''
  }
  loading.value = false
}

onMounted(() => fetchMappings())

function mappingKey(m: ModelProviderEndpointView) {
  return `mapping:${m.modelName}:${m.providerId}:${m.endpointPath}`
}

function openCreate() {
  panel.open(MappingForm, { onSave: () => fetchMappings() }, { key: 'mapping:new' })
}

function openEdit(m: ModelProviderEndpointView) {
  panel.open(MappingForm, { mapping: m, onSave: () => fetchMappings() }, { key: mappingKey(m) })
}

function confirmDeleteMapping(_event: Event, m: ModelProviderEndpointView) {
  confirm.require({
    message: `确定要删除模型「${m.modelName}」的映射吗？此操作不可撤销。`,
    accept: async () => {
      await api.POST('/api/picotera/model-provider-endpoints/delete', {
        body: { modelName: m.modelName, providerId: m.providerId, endpointPath: m.endpointPath },
      })
      fetchMappings()
    },
  })
}
</script>

<template>
  <div class="flex flex-col gap-3.5">
    <div class="flex items-center justify-between gap-3">
      <span class="text-xs text-ink-faint tabular-nums">
        {{ count }} 条映射<span v-if="hasMore">（还有更多）</span>
      </span>
      <div class="flex items-center gap-2">
        <Button @click="openCreate">
          <Icon name="plus" :size="14" :stroke-width="2.2" />
          <span>新增映射</span>
        </Button>
      </div>
    </div>
    <StateText v-if="loading && !mappings.length">加载中…</StateText>
    <DataCard v-else-if="mappings.length">
      <DataTable>
        <thead>
          <tr>
            <Th>模型</Th>
            <Th>渠道</Th>
            <Th>端点</Th>
            <Th>上游模型</Th>
            <Th>优先级</Th>
            <Th>标注</Th>
            <Th actions />
          </tr>
        </thead>
        <tbody>
          <Tr
            v-for="m in mappings"
            :key="`${m.modelName}-${m.providerId}-${m.endpointPath}`"
            :selected="panel.isActive(mappingKey(m))"
          >
            <Td><span class="font-mono font-medium">{{ m.modelName }}</span></Td>
            <Td><span class="font-mono text-ink-faint">{{ m.providerId }}</span></Td>
            <Td><span class="font-mono text-ink-faint">{{ m.endpointPath }}</span></Td>
            <Td><span class="font-mono">{{ m.upstreamModelName || '—' }}</span></Td>
            <Td><Badge>{{ m.priority }}</Badge></Td>
            <Td>
              <TagList>
                <Tag v-for="(v, k) in m.annotations" :key="k">{{ k }}={{ v }}</Tag>
              </TagList>
            </Td>
            <Td actions>
              <div class="inline-flex gap-1 opacity-55 group-hover:opacity-100 transition-opacity">
                <IconButton
                  :active="panel.isActive(mappingKey(m))"
                  title="编辑"
                  aria-label="编辑"
                  @click="openEdit(m)"
                >
                  <Icon name="edit" :size="13" />
                </IconButton>
                <IconButton
                  variant="danger"
                  title="删除"
                  aria-label="删除"
                  @click="(ev: Event) => confirmDeleteMapping(ev, m)"
                >
                  <Icon name="trash" :size="13" />
                </IconButton>
              </div>
            </Td>
          </Tr>
        </tbody>
      </DataTable>
    </DataCard>
    <StateText v-else>暂无映射，点击右上角按钮新增</StateText>
    <div v-if="hasMore" class="flex justify-center py-1">
      <Button variant="ghost" @click="fetchMappings(nextCursor)">加载更多</Button>
    </div>
  </div>
</template>
