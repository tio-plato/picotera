<script setup lang="ts">
import { ref, onMounted, watch, computed } from 'vue'
import { useApi } from '@/composables/useApi'
import type { ModelProviderEndpointView, ModelView, ProviderEndpointView, ProviderView } from '@/api'
import AnnotationsEditor from '@/components/AnnotationsEditor.vue'

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
  <div class="form-panel">
    <div class="form-header">
      <h2 class="form-title">{{ isEdit ? '编辑映射' : '新增映射' }}</h2>
      <button class="close-btn" @click="emit('close')">&times;</button>
    </div>
    <form @submit.prevent="submit" class="form-body">
      <label class="field">
        <span class="field-label">模型</span>
        <select v-model="form.modelName" class="input" required :disabled="isEdit">
          <option value="" disabled>选择模型</option>
          <option v-for="m in models" :key="m.name" :value="m.name">{{ m.title }} ({{ m.name }})</option>
        </select>
      </label>
      <label class="field">
        <span class="field-label">渠道</span>
        <select v-model.number="form.providerId" class="input" required :disabled="isEdit">
          <option :value="0" disabled>选择渠道</option>
          <option v-for="p in providers" :key="p.id" :value="p.id">{{ p.name }} (ID: {{ p.id }})</option>
        </select>
      </label>
      <label class="field">
        <span class="field-label">端点</span>
        <select
          v-model="form.endpointPath"
          class="input"
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
        </select>
      </label>
      <label class="field">
        <span class="field-label">上游模型名称</span>
        <input v-model="form.upstreamModelName" class="input" placeholder="留空则使用模型名称" />
      </label>
      <label class="field">
        <span class="field-label">优先级</span>
        <input v-model.number="form.priority" type="number" class="input" required />
      </label>
      <div class="field">
        <span class="field-label">标注</span>
        <AnnotationsEditor v-model="form.annotations" />
      </div>
      <div v-if="error" class="form-error">{{ error }}</div>
      <div class="form-actions">
        <button type="button" class="btn-ghost" @click="emit('close')">取消</button>
        <button type="submit" class="btn-primary" :disabled="saving">{{ saving ? '保存中…' : isEdit ? '更新' : '创建' }}</button>
      </div>
    </form>
  </div>
</template>

<style scoped>
.form-panel { padding: 0; }
.form-header {
  display: flex; align-items: center; justify-content: space-between;
  padding: 1rem 1.5rem; border-bottom: 1px solid var(--color-card-border);
}
.form-title { font-size: 1rem; font-weight: 600; margin: 0; }
.close-btn {
  background: none; border: none; font-size: 1.25rem; color: var(--color-ink-faint);
  cursor: pointer; line-height: 1; padding: 0.25rem;
}
.close-btn:hover { color: var(--color-ink); }
.form-body { padding: 1.25rem 1.5rem; display: flex; flex-direction: column; gap: 1rem; }
.field { display: flex; flex-direction: column; gap: 0.25rem; }
.field-label {
  font-size: 0.75rem; font-weight: 550; color: var(--color-ink-muted);
  text-transform: uppercase; letter-spacing: 0.03em;
}
.input {
  padding: 0.5rem 0.75rem; border: 1px solid var(--color-card-border); border-radius: 0.375rem;
  font-size: 0.8125rem; font-family: var(--font-sans); background: var(--color-surface-0); transition: border-color 0.1s;
}
.input:focus { outline: none; border-color: var(--color-accent); }
.input:disabled { opacity: 0.5; }
.form-error { color: var(--color-indicator-err); font-size: 0.8125rem; }
.form-actions { display: flex; justify-content: flex-end; gap: 0.5rem; padding-top: 0.5rem; }
.btn-ghost {
  padding: 0.375rem 0.875rem; background: none; border: 1px solid var(--color-card-border);
  border-radius: 0.375rem; font-size: 0.8125rem; cursor: pointer; color: var(--color-ink-muted);
}
.btn-ghost:hover { background: var(--color-surface-50); }
.btn-primary {
  padding: 0.375rem 0.875rem; background: var(--color-accent); color: #fff; border: none;
  border-radius: 0.375rem; font-size: 0.8125rem; font-weight: 500; cursor: pointer;
}
.btn-primary:hover { opacity: 0.9; }
.btn-primary:disabled { opacity: 0.5; cursor: not-allowed; }
</style>
