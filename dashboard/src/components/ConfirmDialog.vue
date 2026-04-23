<script setup lang="ts">
import { confirmState } from '@/composables/useConfirm'

async function accept() {
  if (confirmState.accepting) return
  confirmState.accepting = true
  try {
    await confirmState.onAccept?.()
  } finally {
    confirmState.visible = false
    confirmState.accepting = false
  }
}

function reject() {
  confirmState.visible = false
}
</script>

<template>
  <Teleport to="body">
    <div v-if="confirmState.visible" class="confirm-overlay" @click.self="reject">
      <div class="confirm-dialog">
        <p class="confirm-message">{{ confirmState.message }}</p>
        <div class="confirm-actions">
          <button class="btn-ghost" @click="reject">取消</button>
          <button class="btn-danger" :disabled="confirmState.accepting" @click="accept">删除</button>
        </div>
      </div>
    </div>
  </Teleport>
</template>

<style scoped>
.confirm-overlay {
  position: fixed;
  inset: 0;
  z-index: 9999;
  display: flex;
  align-items: center;
  justify-content: center;
  background: var(--color-overlay-bg);
}
.confirm-dialog {
  min-width: 320px;
  max-width: 420px;
  padding: 1.25rem 1.5rem;
  background: var(--color-card-bg);
  border: 1px solid var(--color-line);
  border-radius: 0.625rem;
  box-shadow: var(--shadow-lg);
}
.confirm-message {
  margin: 0 0 1rem;
  font-size: 0.875rem;
  line-height: 1.5;
  color: var(--color-ink);
}
.confirm-actions {
  display: flex;
  justify-content: flex-end;
  gap: 0.5rem;
}
</style>
