import { ref, type Ref } from 'vue'
import api from '@/api'

export function useFetch<T>(fn: () => Promise<T>) {
  const data: Ref<T | null> = ref(null)
  const loading = ref(false)
  const error: Ref<string | null> = ref(null)

  async function execute() {
    loading.value = true
    error.value = null
    try {
      data.value = await fn()
    } catch (e: any) {
      error.value = e?.message || String(e)
    } finally {
      loading.value = false
    }
  }

  return { data, loading, error, execute }
}

export async function listProviders() {
  const { data, error } = await api.GET('/api/picotera/providers')
  if (error) throw new Error(error.message ?? 'Failed to fetch providers')
  return data as ProviderView[]
}

// Re-export types for convenience
export type { ProviderView, CreateProviderRequestBody, ModelView, EndpointView, ModelProviderEndpointView, PaginatedBody } from '@/api'
