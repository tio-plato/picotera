<script setup lang="ts">
import { ref, computed } from 'vue'
import { useRoute, RouterView } from 'vue-router'
import AppSidebar from '@/components/AppSidebar.vue'
import OverlayPanel from '@/components/OverlayPanel.vue'

const route = useRoute()
const pageName = computed(() => {
  const map: Record<string, string> = {
    providers: '渠道',
    models: '模型',
    endpoints: '端点',
    mappings: '映射',
  }
  return map[route.name as string] ?? ''
})
</script>

<template>
  <div class="app-shell">
    <AppSidebar />
    <main class="app-main">
      <header class="app-header">
        <h1 class="page-title">{{ pageName }}</h1>
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
}
.app-main {
  flex: 1;
  display: flex;
  flex-direction: column;
  min-width: 0;
}
.app-header {
  padding: 1rem 2rem;
  background: var(--color-surface-0);
  border-bottom: 1px solid var(--color-card-border);
}
.page-title {
  font-size: 1.25rem;
  font-weight: 600;
  letter-spacing: -0.02em;
  color: var(--color-ink);
  margin: 0;
}
.app-content {
  flex: 1;
  padding: 1.5rem 2rem;
  overflow-y: auto;
}
</style>
