import { ref, computed } from 'vue'
import { useApi } from '@/composables/useApi'
import type { ProviderView } from '@/api'

const providers = ref<ProviderView[]>([])
let loaded = false

const providersMap = computed(() => {
  const m = new Map<number, ProviderView>()
  for (const p of providers.value) m.set(p.id, p)
  return m
})

export function useProvidersMap() {
  const api = useApi()

  async function fetchProviders() {
    if (loaded) return
    loaded = true
    const { data, error } = await api.GET('/api/picotera/providers')
    if (!error && data) {
      providers.value = data as ProviderView[]
    }
  }

  function providerLabel(id: number): string {
    const p = providersMap.value.get(id)
    return p ? p.name : `#${id}`
  }

  return { providers, providersMap, providerLabel, fetchProviders }
}
