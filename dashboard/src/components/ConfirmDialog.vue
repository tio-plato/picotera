<script setup lang="ts">
const emit = defineEmits<{ close: [] }>()
const props = defineProps<{
  title?: string
  message: string
  confirmLabel?: string
  onConfirm?: () => void | Promise<void>
}>()

const confirming = ref(false)

async function confirm() {
  confirming.value = true
  await props.onConfirm?.()
  confirming.value = false
  emit('close')
}
</script>

<template>
  <div class="confirm-panel">
    <div class="confirm-header">
      <h2 class="confirm-title">{{ title ?? '确认操作' }}</h2>
      <button class="close-btn" @click="emit('close')">&times;</button>
    </div>
    <div class="confirm-body">
      <p class="confirm-message">{{ message }}</p>
      <div class="confirm-actions">
        <button class="btn-ghost" @click="emit('close')">取消</button>
        <button class="btn-danger" :disabled="confirming" @click="confirm">
          {{ confirming ? '删除中…' : confirmLabel ?? '删除' }}
        </button>
      </div>
    </div>
  </div>
</template>

<script lang="ts">
import { ref } from 'vue'
</script>

<style scoped>
.confirm-panel { padding: 0; }
.confirm-header {
  display: flex; align-items: center; justify-content: space-between;
  padding: 1rem 1.5rem; border-bottom: 1px solid var(--color-card-border);
}
.confirm-title { font-size: 1rem; font-weight: 600; margin: 0; }
.close-btn {
  background: none; border: none; font-size: 1.25rem; color: var(--color-ink-faint);
  cursor: pointer; line-height: 1; padding: 0.25rem;
}
.close-btn:hover { color: var(--color-ink); }
.confirm-body { padding: 1.25rem 1.5rem; display: flex; flex-direction: column; gap: 1.25rem; }
.confirm-message { font-size: 0.875rem; color: var(--color-ink); margin: 0; line-height: 1.5; }
.confirm-actions { display: flex; justify-content: flex-end; gap: 0.5rem; }
.btn-ghost {
  padding: 0.375rem 0.875rem; background: none; border: 1px solid var(--color-card-border);
  border-radius: 0.375rem; font-size: 0.8125rem; cursor: pointer; color: var(--color-ink-muted);
}
.btn-ghost:hover { background: var(--color-surface-50); }
.btn-danger {
  padding: 0.375rem 0.875rem; background: var(--color-indicator-err); color: #fff; border: none;
  border-radius: 0.375rem; font-size: 0.8125rem; font-weight: 500; cursor: pointer;
}
.btn-danger:hover { opacity: 0.9; }
.btn-danger:disabled { opacity: 0.5; cursor: not-allowed; }
</style>
