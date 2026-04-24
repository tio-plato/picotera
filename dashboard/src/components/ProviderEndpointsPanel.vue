<script setup lang="ts">
import { ref, reactive, computed, onMounted, watch } from 'vue'
import { useApi } from '@/composables/useApi'
import type { EndpointView, ProviderEndpointView } from '@/api'
import {
  SidePanel,
  Button,
  IconButton,
  Input,
  Select,
  Field,
  StateText,
  Icon,
} from '@/ui'

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
  <SidePanel
    :title="providerName"
    kicker="端点绑定"
    @close="emit('close')"
  >
    <section class="flex flex-col gap-2">
      <div class="flex items-baseline justify-between">
        <span class="text-xs font-medium text-ink-muted uppercase tracking-[0.03em]">已绑定</span>
        <span class="text-xs text-ink-faint tabular-nums">{{ providerEndpoints.length }}</span>
      </div>
      <StateText v-if="loading" :dashed="false" compact>加载中…</StateText>
      <StateText v-else-if="!providerEndpoints.length" compact>暂无绑定，下方选择端点添加</StateText>
      <ul v-else class="list-none m-0 p-0 flex flex-col gap-2">
        <li
          v-for="pe in providerEndpoints"
          :key="pe.endpointPath"
          class="flex flex-col gap-1 px-2.5 py-2 border border-line rounded-md bg-surface-0"
        >
          <div class="font-mono text-sm text-ink overflow-hidden text-ellipsis whitespace-nowrap">
            {{ pe.endpointPath }}
          </div>
          <div class="flex gap-1.5 items-center">
            <Input
              v-model="drafts[pe.endpointPath]"
              size="sm"
              class="flex-1 min-w-0"
              placeholder="上游 URL"
              :title="drafts[pe.endpointPath]"
              @keydown="onDraftKeydown($event, pe.endpointPath)"
              @blur="saveDraft(pe.endpointPath)"
            />
            <IconButton
              variant="danger"
              title="删除绑定"
              :aria-label="`删除 ${pe.endpointPath} 绑定`"
              @click="deleteBinding(pe.endpointPath)"
            >
              <Icon name="trash" :size="13" />
            </IconButton>
          </div>
        </li>
      </ul>
    </section>

    <section class="flex flex-col gap-2">
      <div class="flex items-baseline justify-between">
        <span class="text-xs font-medium text-ink-muted uppercase tracking-[0.03em]">新增绑定</span>
      </div>
      <form class="flex flex-col gap-2" @submit.prevent="addBinding">
        <Field label="端点">
          <Select v-model="form.endpointPath" size="sm" :disabled="!availableEndpoints.length">
            <option value="" disabled>
              {{ availableEndpoints.length ? '选择端点' : '该渠道暂无可绑定端点' }}
            </option>
            <option v-for="e in availableEndpoints" :key="e.path" :value="e.path">
              {{ e.path }} — {{ e.name }}
            </option>
          </Select>
        </Field>
        <Field label="上游 URL">
          <Input
            v-model="form.upstreamUrl"
            size="sm"
            placeholder="https://api.example.com/v1/…"
            :disabled="!availableEndpoints.length"
          />
        </Field>
        <div class="flex justify-end">
          <Button
            type="submit"
            size="sm"
            :disabled="saving || !form.endpointPath || !form.upstreamUrl"
          >
            {{ saving ? '添加中…' : '添加' }}
          </Button>
        </div>
      </form>
    </section>

    <template v-if="error" #error>{{ error }}</template>
  </SidePanel>
</template>
