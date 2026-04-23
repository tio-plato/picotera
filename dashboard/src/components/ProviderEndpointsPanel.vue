<script setup lang="ts">
import { ref, reactive, computed, onMounted, watch } from 'vue'
import { useApi } from '@/composables/useApi'
import type { EndpointView, ProviderEndpointView } from '@/api'

const props = defineProps<{ providerId: number; providerName: string }>()
const emit = defineEmits<{ close: [] }>()
const api = useApi()

const providerEndpoints = ref<ProviderEndpointView[]>([])
const endpoints = ref<EndpointView[]>([])
const loading = ref(false)
const error = ref('')
const form = ref({ endpointPath: '', upstreamUrl: '' })
const drafts = reactive<Record<string, string>>({})
const saving = ref(false)

const availableEndpoints = computed(() =>
  endpoints.value.filter(
    (e) => !providerEndpoints.value.some((pe) => pe.endpointPath === e.path),
  ),
)

async function fetchEndpoints() {
  const { data, error: err } = await api.GET('/api/picotera/endpoints')
  if (err) {
    error.value = err.message ?? '加载端点失败'
    return
  }
  endpoints.value = (data as EndpointView[]) ?? []
}

async function fetchBindings() {
  loading.value = true
  error.value = ''
  const { data, error: err } = await api.GET('/api/picotera/provider-endpoints', {
    params: { query: { providerId: props.providerId } },
  })
  loading.value = false
  if (err) {
    error.value = err.message ?? '加载绑定失败'
    return
  }
  providerEndpoints.value = (data as ProviderEndpointView[]) ?? []
  for (const key of Object.keys(drafts)) delete drafts[key]
  for (const pe of providerEndpoints.value) drafts[pe.endpointPath] = pe.upstreamUrl
}

onMounted(() => {
  Promise.all([fetchEndpoints(), fetchBindings()])
})

watch(
  () => props.providerId,
  () => {
    form.value.endpointPath = ''
    form.value.upstreamUrl = ''
    fetchBindings()
  },
)

async function addBinding() {
  if (!form.value.endpointPath || !form.value.upstreamUrl) return
  saving.value = true
  error.value = ''
  const { error: err } = await api.PUT('/api/picotera/provider-endpoints', {
    body: {
      providerId: props.providerId,
      endpointPath: form.value.endpointPath,
      upstreamUrl: form.value.upstreamUrl,
    },
  })
  saving.value = false
  if (err) {
    error.value = err.message ?? '添加绑定失败'
    return
  }
  form.value.endpointPath = ''
  form.value.upstreamUrl = ''
  await fetchBindings()
}

async function saveDraft(path: string) {
  const pe = providerEndpoints.value.find((p) => p.endpointPath === path)
  if (!pe) return
  const next = drafts[path]
  if (next === undefined || next === pe.upstreamUrl) return
  if (!next) {
    drafts[path] = pe.upstreamUrl
    return
  }
  error.value = ''
  const { error: err } = await api.PUT('/api/picotera/provider-endpoints', {
    body: {
      providerId: props.providerId,
      endpointPath: path,
      upstreamUrl: next,
    },
  })
  if (err) {
    error.value = err.message ?? '更新绑定失败'
    drafts[path] = pe.upstreamUrl
    return
  }
  await fetchBindings()
}

async function deleteBinding(path: string) {
  error.value = ''
  const { error: err } = await api.POST('/api/picotera/provider-endpoints/delete', {
    body: { providerId: props.providerId, endpointPath: path },
  })
  if (err) {
    error.value = err.message ?? '删除绑定失败'
    return
  }
  await fetchBindings()
}

function onDraftKeydown(e: KeyboardEvent, path: string) {
  if (e.key === 'Enter') {
    e.preventDefault()
    ;(e.target as HTMLInputElement).blur()
    saveDraft(path)
  }
}
</script>

<template>
  <aside class="panel">
    <header class="panel-header">
      <div class="panel-title-group">
        <span class="panel-kicker">端点绑定</span>
        <h2 class="panel-title">{{ providerName }}</h2>
      </div>
      <button class="btn-icon" title="关闭" aria-label="关闭侧边栏" @click="emit('close')">
        <svg viewBox="0 0 24 24" width="14" height="14" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M6 6l12 12M18 6l-12 12" /></svg>
      </button>
    </header>

    <div class="panel-body">
      <section class="panel-section">
        <div class="section-head">
          <span class="section-title">已绑定</span>
          <span class="section-count">{{ providerEndpoints.length }}</span>
        </div>
        <div v-if="loading" class="state-text state-text--sm">加载中…</div>
        <div v-else-if="!providerEndpoints.length" class="state-text state-text--sm">暂无绑定，下方选择端点添加</div>
        <ul v-else class="binding-list">
          <li v-for="pe in providerEndpoints" :key="pe.endpointPath" class="binding-item">
            <div class="binding-path mono">{{ pe.endpointPath }}</div>
            <div class="binding-row">
              <input
                v-model="drafts[pe.endpointPath]"
                class="input input--sm"
                placeholder="上游 URL"
                :title="drafts[pe.endpointPath]"
                @keydown="onDraftKeydown($event, pe.endpointPath)"
                @blur="saveDraft(pe.endpointPath)"
              />
              <button
                class="btn-icon btn-icon--danger"
                title="删除绑定"
                :aria-label="`删除 ${pe.endpointPath} 绑定`"
                @click="deleteBinding(pe.endpointPath)"
              >
                <svg viewBox="0 0 24 24" width="13" height="13" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><path d="M4 7h16" /><path d="M10 11v6M14 11v6" /><path d="M6 7l1 12a2 2 0 0 0 2 2h6a2 2 0 0 0 2-2l1-12" /><path d="M9 7V5a2 2 0 0 1 2-2h2a2 2 0 0 1 2 2v2" /></svg>
              </button>
            </div>
          </li>
        </ul>
      </section>

      <section class="panel-section">
        <div class="section-head">
          <span class="section-title">新增绑定</span>
        </div>
        <form class="add-form" @submit.prevent="addBinding">
          <label class="field">
            <span class="field-label">端点</span>
            <select v-model="form.endpointPath" class="input input--sm" :disabled="!availableEndpoints.length">
              <option value="" disabled>
                {{ availableEndpoints.length ? '选择端点' : '该渠道暂无可绑定端点' }}
              </option>
              <option v-for="e in availableEndpoints" :key="e.path" :value="e.path">
                {{ e.path }} — {{ e.name }}
              </option>
            </select>
          </label>
          <label class="field">
            <span class="field-label">上游 URL</span>
            <input
              v-model="form.upstreamUrl"
              class="input input--sm"
              placeholder="https://api.example.com/v1/…"
              :disabled="!availableEndpoints.length"
            />
          </label>
          <div class="add-actions">
            <button
              type="submit"
              class="btn-primary btn-primary--sm"
              :disabled="saving || !form.endpointPath || !form.upstreamUrl"
            >
              {{ saving ? '添加中…' : '添加' }}
            </button>
          </div>
        </form>
      </section>
    </div>

    <div v-if="error" class="panel-error">{{ error }}</div>
  </aside>
</template>

<style scoped>
.panel {
  background: var(--color-card-bg);
  border: 1px solid var(--color-line);
  border-radius: 0.625rem;
  box-shadow: var(--shadow-sm);
  display: flex;
  flex-direction: column;
  overflow: hidden;
  min-height: 0;
}
.panel-header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 0.5rem;
  padding: 0.875rem 1rem;
  border-bottom: 1px solid var(--color-line);
  background: var(--color-surface-50);
}
.panel-title-group { display: flex; flex-direction: column; gap: 0.125rem; min-width: 0; }
.panel-kicker {
  font-size: 0.6875rem;
  font-weight: 550;
  color: var(--color-ink-muted);
  text-transform: uppercase;
  letter-spacing: 0.04em;
}
.panel-title {
  font-size: 0.9375rem;
  font-weight: 600;
  margin: 0;
  color: var(--color-ink);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.panel-body {
  padding: 0.875rem 1rem 1rem;
  display: flex;
  flex-direction: column;
  gap: 1.125rem;
  overflow-y: auto;
  flex: 1 1 auto;
}
.panel-section { display: flex; flex-direction: column; gap: 0.5rem; }
.section-head {
  display: flex;
  align-items: baseline;
  justify-content: space-between;
}
.section-title {
  font-size: 0.75rem;
  font-weight: 550;
  color: var(--color-ink-muted);
  text-transform: uppercase;
  letter-spacing: 0.03em;
}
.section-count {
  font-size: 0.75rem;
  color: var(--color-ink-faint);
  font-variant-numeric: tabular-nums;
}
.state-text--sm { font-size: 0.8125rem; padding: 0.5rem 0; }
.binding-list { list-style: none; margin: 0; padding: 0; display: flex; flex-direction: column; gap: 0.5rem; }
.binding-item {
  display: flex;
  flex-direction: column;
  gap: 0.25rem;
  padding: 0.5rem 0.625rem;
  border: 1px solid var(--color-line);
  border-radius: 0.4375rem;
  background: var(--color-surface-0);
}
.binding-path {
  font-size: 0.8125rem;
  color: var(--color-ink);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.binding-row { display: flex; gap: 0.375rem; align-items: center; }
.binding-row .input { flex: 1 1 auto; min-width: 0; }

.add-form { display: flex; flex-direction: column; gap: 0.5rem; }
.field { display: flex; flex-direction: column; gap: 0.1875rem; }
.field-label {
  font-size: 0.6875rem;
  font-weight: 550;
  color: var(--color-ink-muted);
  text-transform: uppercase;
  letter-spacing: 0.03em;
}
.input {
  padding: 0.5rem 0.625rem;
  border: 1px solid var(--color-line);
  border-radius: 0.375rem;
  font-size: 0.8125rem;
  font-family: var(--font-sans);
  background: var(--color-surface-0);
  color: var(--color-ink);
  transition: border-color 0.12s ease;
}
.input:focus { outline: none; border-color: var(--color-accent); }
.input:disabled { opacity: 0.55; cursor: not-allowed; }
.input--sm { padding: 0.375rem 0.5rem; font-size: 0.8125rem; }
.add-actions { display: flex; justify-content: flex-end; }
.btn-primary--sm { padding: 0.375rem 0.75rem; font-size: 0.8125rem; }
.btn-primary:disabled { opacity: 0.5; cursor: not-allowed; }

.panel-error {
  padding: 0.5rem 1rem;
  background: var(--color-indicator-err-faint);
  color: var(--color-indicator-err-ink);
  font-size: 0.8125rem;
  border-top: 1px solid var(--color-line);
}
</style>
