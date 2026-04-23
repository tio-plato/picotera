<script setup lang="ts">
import { ref } from 'vue'
import { useRoute, RouterLink } from 'vue-router'
import PreferencesMenu from '@/components/PreferencesMenu.vue'

const route = useRoute()

const nav = [
  { name: 'providers', label: '渠道' },
  { name: 'models', label: '模型' },
  { name: 'endpoints', label: '端点' },
  { name: 'mappings', label: '映射' },
]

const prefsRef = ref<InstanceType<typeof PreferencesMenu> | null>(null)
function openPrefs(event: Event) {
  prefsRef.value?.toggle(event)
}
</script>

<template>
  <aside class="sidebar">
    <div class="sidebar-brand">
      <span class="brand-mark" aria-hidden="true">
        <svg viewBox="0 0 24 24" width="18" height="18" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
          <path d="M4 7h10a4 4 0 0 1 0 8H8" />
          <path d="M8 4v16" />
        </svg>
      </span>
      <div class="brand-text">
        <span class="brand-name">PicoTera</span>
        <span class="brand-sub">LLM gateway</span>
      </div>
    </div>

    <nav class="sidebar-nav" aria-label="主导航">
      <div class="nav-section-label">配置</div>
      <RouterLink
        v-for="item in nav"
        :key="item.name"
        :to="{ name: item.name }"
        class="nav-item"
        :class="{ active: route.name === item.name }"
      >
        <span class="nav-icon" aria-hidden="true">
          <!-- providers: database stack -->
          <svg v-if="item.name === 'providers'" viewBox="0 0 24 24" width="15" height="15" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linecap="round" stroke-linejoin="round">
            <ellipse cx="12" cy="5" rx="8" ry="3" />
            <path d="M4 5v6c0 1.66 3.58 3 8 3s8-1.34 8-3V5" />
            <path d="M4 11v6c0 1.66 3.58 3 8 3s8-1.34 8-3v-6" />
          </svg>
          <!-- models: cpu -->
          <svg v-else-if="item.name === 'models'" viewBox="0 0 24 24" width="15" height="15" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linecap="round" stroke-linejoin="round">
            <rect x="5" y="5" width="14" height="14" rx="2" />
            <rect x="9" y="9" width="6" height="6" rx="0.5" />
            <path d="M9 2v3M15 2v3M9 19v3M15 19v3M2 9h3M2 15h3M19 9h3M19 15h3" />
          </svg>
          <!-- endpoints: plug -->
          <svg v-else-if="item.name === 'endpoints'" viewBox="0 0 24 24" width="15" height="15" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linecap="round" stroke-linejoin="round">
            <path d="M9 2v6M15 2v6" />
            <path d="M6 8h12v3a6 6 0 0 1-12 0z" />
            <path d="M12 17v5" />
          </svg>
          <!-- mappings: git-branch -->
          <svg v-else viewBox="0 0 24 24" width="15" height="15" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linecap="round" stroke-linejoin="round">
            <circle cx="6" cy="5" r="2" />
            <circle cx="6" cy="19" r="2" />
            <circle cx="18" cy="12" r="2" />
            <path d="M6 7v10" />
            <path d="M6 12h4a4 4 0 0 0 4-4V7" stroke-dasharray="0" />
            <path d="M16 12h0" />
          </svg>
        </span>
        <span class="nav-label">{{ item.label }}</span>
      </RouterLink>
    </nav>

    <div class="sidebar-footer">
      <button
        class="btn-icon"
        type="button"
        aria-label="设置"
        title="设置"
        @click="openPrefs"
      >
        <svg viewBox="0 0 24 24" width="14" height="14" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round">
          <circle cx="12" cy="12" r="3" />
          <path d="M19.4 15a1.65 1.65 0 0 0 .33 1.82l.06.06a2 2 0 1 1-2.83 2.83l-.06-.06a1.65 1.65 0 0 0-1.82-.33 1.65 1.65 0 0 0-1 1.51V21a2 2 0 1 1-4 0v-.09a1.65 1.65 0 0 0-1-1.51 1.65 1.65 0 0 0-1.82.33l-.06.06a2 2 0 1 1-2.83-2.83l.06-.06a1.65 1.65 0 0 0 .33-1.82 1.65 1.65 0 0 0-1.51-1H3a2 2 0 1 1 0-4h.09a1.65 1.65 0 0 0 1.51-1 1.65 1.65 0 0 0-.33-1.82l-.06-.06a2 2 0 1 1 2.83-2.83l.06.06a1.65 1.65 0 0 0 1.82.33H9a1.65 1.65 0 0 0 1-1.51V3a2 2 0 1 1 4 0v.09a1.65 1.65 0 0 0 1 1.51 1.65 1.65 0 0 0 1.82-.33l.06-.06a2 2 0 1 1 2.83 2.83l-.06.06a1.65 1.65 0 0 0-.33 1.82V9a1.65 1.65 0 0 0 1.51 1H21a2 2 0 1 1 0 4h-.09a1.65 1.65 0 0 0-1.51 1z" />
        </svg>
      </button>
      <span class="version">v1.0.0</span>
      <PreferencesMenu ref="prefsRef" />
    </div>
  </aside>
</template>

<style scoped>
.sidebar {
  width: 224px;
  min-width: 224px;
  background: var(--color-sidebar-bg);
  border-right: 1px solid var(--color-sidebar-border);
  display: flex;
  flex-direction: column;
  height: 100dvh;
  position: sticky;
  top: 0;
}

.sidebar-brand {
  padding: 1.125rem 1rem 1rem;
  display: flex;
  align-items: center;
  gap: 0.625rem;
}
.brand-mark {
  width: 1.875rem;
  height: 1.875rem;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  background: var(--color-accent);
  color: #fff;
  border-radius: 0.4375rem;
  box-shadow:
    inset 0 0 0 1px oklch(1 0 0 / 0.12),
    0 1px 2px oklch(0.3 0.1 262 / 0.25);
}
.brand-text {
  display: flex;
  flex-direction: column;
  line-height: 1.15;
}
.brand-name {
  font-weight: 600;
  font-size: 0.9375rem;
  letter-spacing: -0.01em;
  color: var(--color-ink);
}
.brand-sub {
  font-family: var(--font-mono);
  font-size: 0.6875rem;
  color: var(--color-ink-faint);
  letter-spacing: 0;
}

.sidebar-nav {
  flex: 1;
  padding: 0.375rem 0.5rem 1rem;
  display: flex;
  flex-direction: column;
  gap: 1px;
}
.nav-section-label {
  padding: 0.75rem 0.625rem 0.375rem;
  font-size: 0.6875rem;
  font-weight: 500;
  color: var(--color-ink-faint);
  text-transform: uppercase;
  letter-spacing: 0.06em;
}
.nav-item {
  position: relative;
  display: flex;
  align-items: center;
  gap: 0.625rem;
  padding: 0.4375rem 0.625rem;
  border-radius: 0.375rem;
  color: var(--color-sidebar-text);
  text-decoration: none;
  font-size: 0.8125rem;
  font-weight: 450;
  transition: background-color 0.1s ease, color 0.1s ease;
}
.nav-item:hover {
  background: var(--color-sidebar-hover);
  color: var(--color-sidebar-text-active);
}
.nav-item.active {
  background: var(--color-sidebar-active-bg);
  color: var(--color-sidebar-active-text);
  font-weight: 550;
}
.nav-icon {
  display: inline-flex;
  width: 1.125rem;
  height: 1.125rem;
  align-items: center;
  justify-content: center;
  color: var(--color-ink-faint);
  transition: color 0.1s ease;
}
.nav-item:hover .nav-icon { color: var(--color-ink-muted); }
.nav-item.active .nav-icon { color: var(--color-accent); }

.sidebar-footer {
  padding: 0.625rem 0.875rem 0.75rem;
  border-top: 1px solid var(--color-sidebar-border);
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.5rem;
}
.version {
  font-family: var(--font-mono);
  font-size: 0.6875rem;
  color: var(--color-ink-faint);
  font-variant-numeric: tabular-nums;
}
</style>
