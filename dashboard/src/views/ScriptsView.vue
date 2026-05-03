<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import { useConfirm } from '@/composables/useConfirm'
import { useApi } from '@/composables/useApi'
import { useSidePanel } from '@/composables/useSidePanel'
import type { ScriptView } from '@/api'
import ScriptForm from '@/components/ScriptForm.vue'
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

const scripts = ref<ScriptView[]>([])
const loading = ref(true)
const count = computed(() => scripts.value.length)

async function fetchScripts() {
  loading.value = true
  const { data, error } = await api.GET('/api/picotera/scripts')
  if (!error && data) scripts.value = data as ScriptView[]
  loading.value = false
}

onMounted(fetchScripts)

function openCreate() {
  panel.open(ScriptForm, { onSave: fetchScripts }, { key: 'script:new', width: '600px' })
}

function openEdit(s: ScriptView) {
  panel.open(ScriptForm, { script: s, onSave: fetchScripts }, { key: `script:${s.id}`, width: '600px' })
}

function confirmDelete(_event: Event, id: string) {
  confirm.require({
    message: `确定要删除脚本「${id}」吗？此操作不可撤销。`,
    accept: async () => {
      await api.POST('/api/picotera/scripts/delete', { body: { id } })
      fetchScripts()
    },
  })
}

async function toggle(s: ScriptView) {
  await api.PUT('/api/picotera/scripts/{id}', {
    params: { path: { id: s.id } },
    body: { name: s.name, source: s.source, enabled: !s.enabled },
  })
  fetchScripts()
}
</script>

<template>
  <div class="flex flex-col gap-3.5">
    <div class="flex items-center justify-between gap-3">
      <span class="text-xs text-ink-faint tabular-nums">{{ count }} 个脚本</span>
      <div class="flex items-center gap-2">
        <Button @click="openCreate">
          <Icon name="plus" :size="14" :stroke-width="2.2" />
          <span>新增脚本</span>
        </Button>
      </div>
    </div>
    <StateText v-if="loading">加载中…</StateText>
    <DataCard v-else-if="scripts.length">
      <DataTable>
        <thead>
          <tr>
            <Th>名称</Th>
            <Th actions />
          </tr>
        </thead>
        <tbody>
          <Tr
            v-for="s in scripts"
            :key="s.id"
            :selected="panel.isActive(`script:${s.id}`)"
            :class="!s.enabled ? 'opacity-55' : ''"
          >
            <Td>
              <span class="font-medium">{{ s.name }}</span>
              <Tag v-if="!s.enabled" variant="muted" class="ml-1.5">已禁用</Tag>
              <span class="block font-mono text-2xs text-ink-faint">{{ s.id }}</span>
            </Td>
            <Td actions>
              <div class="inline-flex gap-1 opacity-55 group-hover:opacity-100 transition-opacity">
                <IconButton
                  :title="s.enabled ? '禁用脚本' : '启用脚本'"
                  :aria-label="s.enabled ? '禁用脚本' : '启用脚本'"
                  @click="toggle(s)"
                >
                  <Icon :name="s.enabled ? 'puzzle' : 'puzzle-off'" :size="13" />
                </IconButton>
                <IconButton
                  :active="panel.isActive(`script:${s.id}`)"
                  title="编辑"
                  aria-label="编辑"
                  @click="openEdit(s)"
                >
                  <Icon name="edit" :size="13" />
                </IconButton>
                <IconButton
                  variant="danger"
                  title="删除"
                  aria-label="删除"
                  @click="(ev: Event) => confirmDelete(ev, s.id)"
                >
                  <Icon name="trash" :size="13" />
                </IconButton>
              </div>
            </Td>
          </Tr>
        </tbody>
      </DataTable>
    </DataCard>
    <StateText v-else>暂无脚本，点击右上角按钮新增</StateText>
  </div>
</template>
