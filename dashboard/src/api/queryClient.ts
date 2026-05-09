import { QueryClient } from '@tanstack/vue-query'

export const MANAGEMENT_STALE_TIME = 30_000
export const OPERATIONAL_STALE_TIME = 5_000

export const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      refetchOnWindowFocus: false,
      retry: 1,
      staleTime: MANAGEMENT_STALE_TIME,
    },
    mutations: {
      retry: 0,
    },
  },
})
