<script setup lang="ts">
import { useRoute, RouterLink } from 'vue-router'

const route = useRoute()

const nav = [
  { name: 'providers', label: '渠道', icon: '◈' },
  { name: 'models', label: '模型', icon: '◆' },
  { name: 'endpoints', label: '端点', icon: '▸' },
  { name: 'mappings', label: '映射', icon: '⬡' },
]
</script>

<template>
  <aside class="sidebar">
    <div class="sidebar-brand">
      <span class="brand-mark">PT</span>
      <span class="brand-name">PicoTera</span>
    </div>
    <nav class="sidebar-nav">
      <RouterLink
        v-for="item in nav"
        :key="item.name"
        :to="{ name: item.name }"
        class="nav-item"
        :class="{ active: route.name === item.name }"
      >
        <span class="nav-icon">{{ item.icon }}</span>
        <span class="nav-label">{{ item.label }}</span>
      </RouterLink>
    </nav>
    <div class="sidebar-footer">
      <span class="version">v1.0.0</span>
    </div>
  </aside>
</template>

<style scoped>
.sidebar {
  width: 200px;
  min-width: 200px;
  background: var(--color-sidebar-bg);
  display: flex;
  flex-direction: column;
  height: 100dvh;
  position: sticky;
  top: 0;
}
.sidebar-brand {
  padding: 1.25rem 1rem;
  display: flex;
  align-items: center;
  gap: 0.625rem;
  border-bottom: 1px solid oklch(0.25 0.02 250);
}
.brand-mark {
  width: 2rem;
  height: 2rem;
  display: flex;
  align-items: center;
  justify-content: center;
  background: var(--color-sidebar-active);
  color: #fff;
  font-weight: 700;
  font-size: 0.75rem;
  letter-spacing: 0.05em;
  border-radius: 0.375rem;
}
.brand-name {
  color: var(--color-sidebar-text-active);
  font-weight: 600;
  font-size: 0.9375rem;
  letter-spacing: -0.01em;
}
.sidebar-nav {
  flex: 1;
  padding: 0.5rem 0;
  display: flex;
  flex-direction: column;
  gap: 2px;
}
.nav-item {
  display: flex;
  align-items: center;
  gap: 0.625rem;
  padding: 0.5rem 1rem;
  margin: 0 0.375rem;
  border-radius: 0.375rem;
  color: var(--color-sidebar-text);
  text-decoration: none;
  font-size: 0.8125rem;
  font-weight: 450;
  transition: background 0.1s, color 0.1s;
}
.nav-item:hover {
  background: var(--color-sidebar-hover);
  color: var(--color-sidebar-text-active);
}
.nav-item.active {
  background: oklch(0.55 0.18 255 / 0.15);
  color: oklch(0.80 0.12 255);
  font-weight: 550;
}
.nav-icon {
  font-size: 0.75rem;
  width: 1.25rem;
  text-align: center;
  opacity: 0.7;
}
.nav-item.active .nav-icon {
  opacity: 1;
}
.sidebar-footer {
  padding: 1rem;
  border-top: 1px solid oklch(0.25 0.02 250);
}
.version {
  font-size: 0.6875rem;
  color: oklch(0.45 0.01 250);
  font-family: var(--font-mono);
}
</style>
