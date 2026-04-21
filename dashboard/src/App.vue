<script setup lang="ts">
import { computed } from 'vue'
import { useRoute, RouterView } from 'vue-router'
import AppSidebar from '@/components/AppSidebar.vue'
import OverlayPanel from '@/components/OverlayPanel.vue'

const route = useRoute()

const pageMeta = computed(() => {
  const map: Record<string, { title: string; hint: string }> = {
    providers: { title: '渠道', hint: '上游模型提供方与凭证' },
    models: { title: '模型', hint: '对外暴露的模型标识' },
    endpoints: { title: '端点', hint: 'HTTP 入口与请求形状' },
    mappings: { title: '映射', hint: '模型 × 渠道 × 端点的路由' },
  }
  return map[route.name as string] ?? { title: '', hint: '' }
})
</script>

<template>
  <div class="app-shell">
    <AppSidebar />
    <main class="app-main">
      <header class="app-header">
        <div class="header-titles">
          <h1 class="page-title">{{ pageMeta.title }}</h1>
          <p class="page-hint">{{ pageMeta.hint }}</p>
        </div>
      </header>
      <div class="app-content">
        <RouterView />
      </div>
    </main>
  </div>
  <OverlayPanel />
</template>

<style scoped>
.app-shell {
  display: flex;
  min-height: 100dvh;
  background: var(--color-surface-50);
}
.app-main {
  flex: 1;
  display: flex;
  flex-direction: column;
  min-width: 0;
}
.app-header {
  padding: 1.125rem 2rem 0.875rem;
  background: transparent;
}
.header-titles {
  display: flex;
  flex-direction: column;
  gap: 0.125rem;
}
.page-title {
  font-size: 1.25rem;
  font-weight: 600;
  letter-spacing: -0.015em;
  color: var(--color-ink);
  margin: 0;
  line-height: 1.2;
}
.page-hint {
  margin: 0;
  font-size: 0.8125rem;
  color: var(--color-ink-faint);
  line-height: 1.2;
}
.app-content {
  flex: 1;
  padding: 0.75rem 2rem 2rem;
  overflow-y: auto;
}
</style>
