<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import { useConfirm } from '@/composables/useConfirm'
import { useApi } from '@/composables/useApi'
import type { ModelView } from '@/api'
import ModelForm from '@/components/ModelForm.vue'
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
  Tag,
  Icon,
} from '@/ui'

const panel = useSidePanel()
const confirm = useConfirm()
const api = useApi()

const models = ref<ModelView[]>([])
const loading = ref(true)
const count = computed(() => models.value.length)

async function fetchModels() {
  loading.value = true
  const { data, error } = await api.GET('/api/picotera/models')
  if (!error && data) models.value = data as ModelView[]
  loading.value = false
}

onMounted(fetchModels)

function openCreate() {
  panel.open(ModelForm, { onSave: fetchModels }, { key: 'model:new' })
}

function openEdit(m: ModelView) {
  panel.open(ModelForm, { model: m, onSave: fetchModels }, { key: `model:${m.name}` })
}

function confirmDelete(_event: Event, m: ModelView) {
  confirm.require({
    message: `确定要删除模型「${m.name}」吗？此操作不可撤销。`,
    accept: async () => {
      await api.POST('/api/picotera/models/delete', { body: { name: m.name } })
      fetchModels()
    },
  })
}
</script>

<template>
  <div class="flex flex-col gap-3.5">
    <div class="flex items-center justify-between gap-3">
      <span class="text-xs text-ink-faint tabular-nums">{{ count }} 个模型</span>
      <div class="flex items-center gap-2">
        <Button @click="openCreate">
          <Icon name="plus" :size="14" :stroke-width="2.2" />
          <span>新增模型</span>
        </Button>
      </div>
    </div>
    <StateText v-if="loading">加载中…</StateText>
    <DataCard v-else-if="models.length">
      <DataTable>
        <thead>
          <tr>
            <Th>名称</Th>
            <Th>标题</Th>
            <Th>开发者</Th>
            <Th>系列</Th>
            <Th actions />
          </tr>
        </thead>
        <tbody>
          <Tr v-for="m in models" :key="m.name" :selected="panel.isActive(`model:${m.name}`)">
            <Td><span class="font-mono font-medium">{{ m.name }}</span></Td>
            <Td>{{ m.title }}</Td>
            <Td><span class="text-ink-faint">{{ m.developer }}</span></Td>
            <Td><Tag>{{ m.series }}</Tag></Td>
            <Td actions>
              <div class="inline-flex gap-1 opacity-55 group-hover:opacity-100 transition-opacity">
                <IconButton
                  :active="panel.isActive(`model:${m.name}`)"
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
                  @click="(e: Event) => confirmDelete(e, m)"
                >
                  <Icon name="trash" :size="13" />
                </IconButton>
              </div>
            </Td>
          </Tr>
        </tbody>
      </DataTable>
    </DataCard>
    <StateText v-else>暂无模型，点击右上角按钮新增</StateText>
  </div>
</template>
