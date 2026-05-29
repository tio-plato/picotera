import { computed, watch } from 'vue'
import { useQuery } from '@tanstack/vue-query'
import { getGlobalSetting } from '@/api/client'
import { queryKeys } from '@/api/queryKeys'

const DEFAULT_TITLE = 'PicoTera'

export function useAppTitle() {
  const query = useQuery({
    queryKey: queryKeys.globalSettings.detail('app.title'),
    queryFn: () => getGlobalSetting('app.title'),
    retry: false,
    // If the setting doesn't exist (404), return null instead of throwing.
    throwOnError: false,
  })

  const appTitle = computed(() => {
    if (!query.data.value) return DEFAULT_TITLE
    const val = query.data.value.value
    if (typeof val === 'string') {
      return val.trim() || DEFAULT_TITLE
    }
    return DEFAULT_TITLE
  })

  // Keep document.title in sync.
  watch(
    appTitle,
    (title) => {
      document.title = title
    },
    { immediate: true },
  )

  return {
    appTitle,
    query,
  }
}
