import { computed, type MaybeRefOrGetter, toValue } from 'vue'
import { useQuery } from '@tanstack/vue-query'
import type { ArtifactPayload } from '@/components/artifactTypes'
import { queryKeys } from '@/api/queryKeys'

async function fetchArtifact(url: string): Promise<ArtifactPayload> {
  const res = await fetch(url)
  if (!res.ok) {
    if (res.status === 404) throw new Error('artifact 不可用')
    throw new Error(`加载失败 (${res.status})`)
  }
  return res.json() as Promise<ArtifactPayload>
}

export function useArtifact(url: MaybeRefOrGetter<string | undefined>) {
  const currentUrl = computed(() => toValue(url) ?? '')
  return useQuery({
    queryKey: computed(() => queryKeys.artifacts.detail(currentUrl.value)),
    queryFn: () => fetchArtifact(currentUrl.value),
    enabled: computed(() => !!currentUrl.value),
  })
}
