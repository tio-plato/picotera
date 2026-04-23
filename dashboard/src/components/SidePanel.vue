<script setup lang="ts">
defineProps<{
  title: string
  kicker?: string
  subtitle?: string
}>()
const emit = defineEmits<{ close: [] }>()
</script>

<template>
  <section class="panel">
    <header class="panel-header">
      <div class="panel-title-group">
        <span v-if="kicker" class="panel-kicker">{{ kicker }}</span>
        <h2 class="panel-title" :title="title">{{ title }}</h2>
        <p v-if="subtitle" class="panel-subtitle">{{ subtitle }}</p>
      </div>
      <button class="btn-icon" title="关闭" aria-label="关闭侧边栏" @click="emit('close')">
        <svg viewBox="0 0 24 24" width="14" height="14" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M6 6l12 12M18 6l-12 12" /></svg>
      </button>
    </header>
    <div class="panel-body">
      <slot />
    </div>
    <footer v-if="$slots.footer" class="panel-footer">
      <slot name="footer" />
    </footer>
    <div v-if="$slots.error" class="panel-error">
      <slot name="error" />
    </div>
  </section>
</template>

<style scoped>
.panel {
  background: var(--color-card-bg);
  border: 1px solid var(--color-line);
  border-radius: 0.625rem;
  box-shadow: var(--shadow-sm);
  display: flex;
  flex-direction: column;
  overflow: hidden;
  min-height: 0;
  height: 100%;
}
.panel-header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 0.5rem;
  padding: 0.875rem 1rem;
  border-bottom: 1px solid var(--color-line);
  background: var(--color-surface-50);
  flex: 0 0 auto;
}
.panel-title-group { display: flex; flex-direction: column; gap: 0.125rem; min-width: 0; }
.panel-kicker {
  font-size: 0.6875rem;
  font-weight: 550;
  color: var(--color-ink-muted);
  text-transform: uppercase;
  letter-spacing: 0.04em;
}
.panel-title {
  font-size: 0.9375rem;
  font-weight: 600;
  margin: 0;
  color: var(--color-ink);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.panel-subtitle {
  margin: 0;
  font-size: 0.75rem;
  color: var(--color-ink-faint);
  line-height: 1.35;
}
.panel-body {
  padding: 0.875rem 1rem 1rem;
  display: flex;
  flex-direction: column;
  gap: 1.125rem;
  overflow-y: auto;
  flex: 1 1 auto;
  min-height: 0;
}
.panel-footer {
  flex: 0 0 auto;
  display: flex;
  gap: 0.5rem;
  justify-content: flex-end;
  padding: 0.75rem 1rem;
  border-top: 1px solid var(--color-line);
  background: var(--color-surface-50);
}
.panel-error {
  flex: 0 0 auto;
  padding: 0.5rem 1rem;
  background: var(--color-indicator-err-faint);
  color: var(--color-indicator-err-ink);
  font-size: 0.8125rem;
  border-top: 1px solid var(--color-line);
}
</style>
