<script setup lang="ts">
import { computed } from 'vue'
import { useRoute, RouterView } from 'vue-router'
import AppSidebar from '@/components/AppSidebar.vue'
import SidePanelHost from '@/components/SidePanelHost.vue'
import ConfirmDialog from '@/ui/ConfirmDialog.vue'

const route = useRoute()

const pageMeta = computed(() => {
  const map: Record<string, { title: string; hint: string }> = {
    providers: { title: '渠道', hint: '上游模型提供方与凭证' },
    models: { title: '模型', hint: '对外暴露的模型标识' },
    endpoints: { title: '端点', hint: 'HTTP 入口与请求形状' },
    requests: { title: '请求', hint: '推理请求历史与状态' },
    scripts: { title: '脚本', hint: '自定义钩子脚本与执行逻辑' },
  }
  return map[route.name as string] ?? { title: '', hint: '' }
})
</script>

<template>
  <div class="flex min-h-[100dvh] bg-surface-50">
    <AppSidebar />
    <main class="flex-1 flex flex-col min-w-0">
      <header class="px-8 pt-[1.125rem] pb-3.5">
        <div class="flex flex-col gap-0.5">
          <h1 class="text-xl font-semibold tracking-[-0.015em] text-ink m-0 leading-[1.2]">
            {{ pageMeta.title }}
          </h1>
          <p class="m-0 text-sm text-ink-faint leading-[1.2]">{{ pageMeta.hint }}</p>
        </div>
      </header>
      <div class="flex-1 flex min-h-0 min-w-0">
        <div class="flex-1 min-w-0 px-8 pt-3 pb-8 overflow-y-auto">
          <RouterView />
        </div>
        <SidePanelHost />
      </div>
    </main>
  </div>
  <ConfirmDialog />
</template>
