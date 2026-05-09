import type { App } from 'vue'
import createClient from 'openapi-fetch'
import type { paths } from '@/openapi-types'

export type ApiClient = ReturnType<typeof createClient<paths>>

export function createApi(baseURL = '/') {
  return createClient<paths>({ baseUrl: baseURL })
}

export const api = createApi()

export const apiPlugin = {
  install(app: App, options?: { baseURL?: string }) {
    app.provide('api', options?.baseURL ? createApi(options.baseURL) : api)
  },
}
