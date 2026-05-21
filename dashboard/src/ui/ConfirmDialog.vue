<script setup lang="ts">
import { confirmState } from '@/composables/useConfirm'
import Button from './Button.vue'

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
    <div
      v-if="confirmState.visible"
      class="fixed inset-0 z-[9999] flex items-center justify-center bg-overlay-bg"
      @click.self="reject"
    >
      <div
        class="min-w-80 max-w-[420px] px-6 py-5 bg-surface-0 border border-line rounded-xl shadow-lg"
      >
        <p class="m-0 mb-4 text-sm leading-[1.5] text-ink">{{ confirmState.message }}</p>
        <div class="flex justify-end gap-2">
          <Button variant="ghost" @click="reject">取消</Button>
          <Button variant="danger" :disabled="confirmState.accepting" @click="accept">删除</Button>
        </div>
      </div>
    </div>
  </Teleport>
</template>
