<script setup lang="ts">
import { computed, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import RequestDetailsContent from '@/components/RequestDetailsContent.vue'
import { useProvidersMap } from '@/composables/useProvidersMap'
import { Button, DataCard, Icon, StateText } from '@/ui'

const route = useRoute()
const router = useRouter()
const { providers } = useProvidersMap()

const requestId = computed(() => {
  const value = route.params.requestId
  return typeof value === 'string' ? value : ''
})
const selectedRequestId = ref('')
const displayRequestId = computed(() => selectedRequestId.value || requestId.value)

function backToRequests() {
  router.replace({ name: 'requests', query: route.query })
}

function replaceDetailUrl(nextRequestId: string) {
  selectedRequestId.value = nextRequestId
  const url = router.resolve({
    name: 'requestDetail',
    params: { requestId: nextRequestId },
    query: route.query,
  }).href
  window.history.replaceState(window.history.state, '', url)
}
</script>

<template>
  <div class="flex flex-col gap-3.5">
    <DataCard>
      <div class="flex items-start justify-between gap-3 px-4 py-3 border-b border-line">
        <div class="min-w-0 flex flex-col gap-1">
          <span class="text-2xs font-medium text-ink-muted uppercase tracking-[0.04em]">请求详情</span>
          <span class="font-mono text-sm text-ink break-all">{{ displayRequestId || '参数错误' }}</span>
        </div>
        <Button variant="ghost" @click="backToRequests">
          <Icon name="arrow-left" :size="14" />
          <span>返回</span>
        </Button>
      </div>
      <div class="p-4">
        <RequestDetailsContent
          v-if="requestId"
          :request-id="requestId"
          :providers="providers"
          @selected-request="replaceDetailUrl"
        />
        <StateText v-else :dashed="false" compact>请求 ID 参数无效</StateText>
      </div>
    </DataCard>
  </div>
</template>
