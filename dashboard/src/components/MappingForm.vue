<script setup lang="ts">
import { ref, onMounted, watch, computed } from 'vue'
import { useApi } from '@/composables/useApi'
import type { ModelProviderEndpointView, ModelView, ProviderEndpointView, ProviderView } from '@/api'
import AnnotationsEditor from '@/components/AnnotationsEditor.vue'
import { SidePanel, Button, Input, Select, Field } from '@/ui'

const emit = defineEmits<{ close: [] }>()
const props = defineProps<{ mapping?: ModelProviderEndpointView; onSave?: () => void }>()
const api = useApi()

const isEdit = !!props.mapping
const form = ref({
  modelName: props.mapping?.modelName ?? '',
  providerId: props.mapping?.providerId ?? 0,
  endpointPath: props.mapping?.endpointPath ?? '',
  upstreamModelName: props.mapping?.upstreamModelName ?? '',
  priority: props.mapping?.priority ?? 0,
  annotations: { ...(props.mapping?.annotations ?? {}) } as Record<string, string>,
})

const models = ref<ModelView[]>([])
const providers = ref<ProviderView[]>([])
const providerEndpoints = ref<ProviderEndpointView[]>([])
const loadingEndpoints = ref(false)

const hasDirtyEndpoint = computed(
  () =>
    isEdit &&
    !!form.value.endpointPath &&
    !providerEndpoints.value.some((pe) => pe.endpointPath === form.value.endpointPath),
)

async function fetchProviderEndpoints(pid: number) {
  if (!pid) {
    providerEndpoints.value = []
    return
  }
  loadingEndpoints.value = true
  const { data, error } = await api.GET('/api/picotera/provider-endpoints', {
    params: { query: { providerId: pid } },
  })
  loadingEndpoints.value = false
  if (error) return
  providerEndpoints.value = (data as ProviderEndpointView[]) ?? []
  if (
    !isEdit &&
    form.value.endpointPath &&
    !providerEndpoints.value.some((pe) => pe.endpointPath === form.value.endpointPath)
  ) {
    form.value.endpointPath = ''
  }
}

onMounted(async () => {
  const [m, p] = await Promise.all([
    api.GET('/api/picotera/models'),
    api.GET('/api/picotera/providers'),
  ])
  if (!m.error && m.data) models.value = m.data as ModelView[]
  if (!p.error && p.data) providers.value = p.data as ProviderView[]
  if (form.value.providerId) fetchProviderEndpoints(form.value.providerId)
})

watch(
  () => form.value.providerId,
  (pid, old) => {
    if (pid === old) return
    if (isEdit) return
    form.value.endpointPath = ''
    if (pid) fetchProviderEndpoints(pid)
    else providerEndpoints.value = []
  },
)

const saving = ref(false)
const error = ref('')

async function submit() {
  saving.value = true
  error.value = ''
  const body = {
    modelName: form.value.modelName,
    providerId: form.value.providerId,
    endpointPath: form.value.endpointPath,
    upstreamModelName: form.value.upstreamModelName || undefined,
    priority: form.value.priority,
    annotations: form.value.annotations,
  }
  const { error: err } = await api.PUT('/api/picotera/model-provider-endpoints', { body })
  if (err) {
    error.value = err.message ?? '操作失败'
  } else {
    props.onSave?.()
    emit('close')
  }
  saving.value = false
}
</script>

<template>
  <SidePanel
    :title="isEdit ? `${form.modelName} → #${form.providerId}` : '新增映射'"
    :kicker="isEdit ? '编辑映射' : '映射'"
    @close="emit('close')"
  >
    <form id="mapping-form" class="flex flex-col gap-4" @submit.prevent="submit">
      <Field label="模型">
        <Select v-model="form.modelName" required :disabled="isEdit">
          <option value="" disabled>选择模型</option>
          <option v-for="m in models" :key="m.name" :value="m.name">{{ m.title }} ({{ m.name }})</option>
        </Select>
      </Field>
      <Field label="渠道">
        <Select v-model.number="form.providerId" required :disabled="isEdit">
          <option :value="0" disabled>选择渠道</option>
          <option v-for="p in providers" :key="p.id" :value="p.id">{{ p.name }} (ID: {{ p.id }})</option>
        </Select>
      </Field>
      <Field label="端点">
        <Select
          v-model="form.endpointPath"
          required
          :disabled="isEdit || !form.providerId || (!providerEndpoints.length && !hasDirtyEndpoint)"
        >
          <option value="" disabled>
            {{ form.providerId
              ? (loadingEndpoints
                ? '加载中…'
                : (providerEndpoints.length ? '选择端点' : '该渠道暂无绑定端点'))
              : '先选择渠道' }}
          </option>
          <option v-for="pe in providerEndpoints" :key="pe.endpointPath" :value="pe.endpointPath">
            {{ pe.endpointPath }}
          </option>
          <option v-if="hasDirtyEndpoint" :value="form.endpointPath">
            {{ form.endpointPath }}（脏数据）
          </option>
        </Select>
      </Field>
      <Field label="上游模型名称">
        <Input v-model="form.upstreamModelName" placeholder="留空则使用模型名称" />
      </Field>
      <Field label="优先级">
        <Input v-model.number="form.priority" type="number" required />
      </Field>
      <Field label="标注" as="div">
        <AnnotationsEditor v-model="form.annotations" />
      </Field>
    </form>

    <template v-if="error" #error>{{ error }}</template>

    <template #footer>
      <Button variant="ghost" @click="emit('close')">取消</Button>
      <Button type="submit" form="mapping-form" :disabled="saving">
        {{ saving ? '保存中…' : isEdit ? '更新' : '创建' }}
      </Button>
    </template>
  </SidePanel>
</template>
