import type { App } from 'vue'
import createClient from 'openapi-fetch'
import type { paths } from '@/openapi-types'
import { useImpersonationStore } from '@/stores/impersonation'

export type ApiClient = ReturnType<typeof createClient<paths>>

export function createApi(baseURL = '/') {
  const client = createClient<paths>({ baseUrl: baseURL })
  // Inject the impersonation header on every management request when an admin
  // is impersonating. Requests happen after app mount, so pinia is active.
  // The raw-`fetch` test requests don't go through this client, so they stay
  // on the real identity.
  client.use({
    onRequest({ request }) {
      const store = useImpersonationStore()
      if (store.target) {
        request.headers.set('X-PicoTera-Impersonation-User-Id', String(store.target.userId))
      }
      return request
    },
  })
  return client
}

export const api = createApi()

export const apiPlugin = {
  install(app: App, options?: { baseURL?: string }) {
    app.provide('api', options?.baseURL ? createApi(options.baseURL) : api)
  },
}
