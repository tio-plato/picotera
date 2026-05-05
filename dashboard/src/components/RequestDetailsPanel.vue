<script setup lang="ts">
import type { ProviderView } from '@/api'
import { SidePanel } from '@/ui'
import RequestDetailsContent from './RequestDetailsContent.vue'

defineProps<{ requestId: string; providers?: ProviderView[] }>()
const emit = defineEmits<{ close: [] }>()
</script>

<template>
  <SidePanel title="请求详情" :kicker="requestId" @close="emit('close')">
    <RequestDetailsContent :request-id="requestId" :providers="providers" />
  </SidePanel>
</template>
