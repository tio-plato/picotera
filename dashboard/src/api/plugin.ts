import type { App } from 'vue'
import createClient from 'openapi-fetch'
import type { paths } from '@/api'

export type ApiClient = ReturnType<typeof createClient<paths>>

export function createApi(baseURL = '/') {
  return createClient<paths>({ baseUrl: baseURL })
}

export const apiPlugin = {
  install(app: App, options?: { baseURL?: string }) {
    const api = createApi(options?.baseURL)
    app.provide('api', api)
  },
}
