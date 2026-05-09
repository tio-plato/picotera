<script setup lang="ts">
import { ref } from 'vue'
import { useMutation, useQueryClient } from '@tanstack/vue-query'
import { SidePanel, Button, IconButton, Input, Field, Icon } from '@/ui'
import type { ProjectView } from '@/api'
import { invalidateProjects, upsertProject } from '@/api/client'

const emit = defineEmits<{ close: [] }>()
const props = defineProps<{ project?: ProjectView; onSave?: () => void }>()
const queryClient = useQueryClient()

const isEdit = !!props.project
type PathRow = { id: number; value: string }
let nextId = 0
const initialPaths: PathRow[] = (props.project?.paths ?? []).map((p) => ({ id: nextId++, value: p }))
const form = ref({
  name: props.project?.name ?? '',
  paths: initialPaths.length ? initialPaths : [{ id: nextId++, value: '' }],
})
const saving = ref(false)
const error = ref('')

const saveMutation = useMutation({
  mutationFn: upsertProject,
  onSuccess: () => invalidateProjects(queryClient),
})

function addPath() {
  form.value.paths.push({ id: nextId++, value: '' })
}

function removePath(id: number) {
  form.value.paths = form.value.paths.filter((r) => r.id !== id)
  if (form.value.paths.length === 0) addPath()
}

async function submit() {
  saving.value = true
  error.value = ''
  const paths = form.value.paths.map((r) => r.value).filter((v) => v !== '')
  const body = {
    id: props.project?.id ?? 0,
    name: form.value.name,
    paths,
  }
  try {
    await saveMutation.mutateAsync(body)
    props.onSave?.()
    emit('close')
  } catch (e: unknown) {
    error.value = e instanceof Error ? e.message : '操作失败'
  }
  saving.value = false
}
</script>

<template>
  <SidePanel
    :title="isEdit ? form.name || '项目' : '新增项目'"
    :kicker="isEdit ? '编辑项目' : '项目'"
    @close="emit('close')"
  >
    <form id="project-form" class="flex flex-col gap-4" @submit.prevent="submit">
      <Field label="名称">
        <Input v-model="form.name" required placeholder="例如 picotera" />
      </Field>
      <Field label="路径前缀" as="div">
        <div class="flex flex-col gap-1.5">
          <div
            v-for="row in form.paths"
            :key="row.id"
            class="flex items-center gap-1"
          >
            <Input
              v-model="row.value"
              placeholder="/home/user/project"
              class="flex-1 min-w-0 font-mono"
            />
            <IconButton
              type="button"
              variant="danger"
              size="sm"
              class="shrink-0"
              aria-label="删除此条路径"
              @click="removePath(row.id)"
            >
              <Icon name="close" :size="11" :stroke-width="1.6" />
            </IconButton>
          </div>
          <button
            type="button"
            class="inline-flex items-center gap-2 self-start pl-2 pr-2 py-1 bg-transparent border border-dashed border-line rounded-[5px] text-xs text-ink-muted cursor-pointer transition-colors hover:bg-accent-faint hover:text-accent-ink hover:border-accent/40 hover:border-solid [&_svg]:opacity-70 hover:[&_svg]:opacity-100"
            @click="addPath"
          >
            <Icon name="plus" :size="11" :stroke-width="1.6" />
            添加路径
          </button>
        </div>
        <p class="text-2xs text-ink-faint pt-2">
          请求体中匹配到的路径将以最长前缀方式归入此项目。空白条目会被忽略。
        </p>
      </Field>
    </form>

    <template v-if="error" #error>{{ error }}</template>

    <template #footer>
      <Button variant="ghost" @click="emit('close')">取消</Button>
      <Button type="submit" form="project-form" :disabled="saving">
        {{ saving ? '保存中…' : isEdit ? '更新' : '创建' }}
      </Button>
    </template>
  </SidePanel>
</template>
