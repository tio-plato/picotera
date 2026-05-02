import { inject } from 'vue'
import type { ApiClient } from '@/api/plugin'

export function useApi(): ApiClient {
  const api = inject<ApiClient>('api')
  if (!api) throw new Error('API client not provided. Did you forget to install the plugin?')
  return api
}

export type { ProviderView, ProviderModelEntry, CreateProviderRequestBody, ModelView, EndpointView, ProviderEndpointView, RequestView } from '@/api'
