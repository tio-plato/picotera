<script setup lang="ts">
import { useOverlay } from '@/composables/useOverlay'

const { visible, component, props, close } = useOverlay()
</script>

<template>
  <Teleport to="body">
    <Transition name="overlay">
      <div v-if="visible" class="overlay-backdrop" @click.self="close">
        <div class="overlay-container">
          <component :is="component" v-bind="props" @close="close" />
        </div>
      </div>
    </Transition>
  </Teleport>
</template>

<style scoped>
.overlay-backdrop {
  position: fixed;
  inset: 0;
  z-index: 1000;
  background: var(--color-overlay-bg);
  display: flex;
  align-items: center;
  justify-content: center;
  backdrop-filter: blur(4px);
}
.overlay-container {
  background: var(--color-card-bg);
  border-radius: 0.75rem;
  box-shadow: 0 25px 50px -12px oklch(0.1 0.02 250 / 0.25);
  width: 90vw;
  max-width: 520px;
  max-height: 85vh;
  overflow-y: auto;
}
.overlay-enter-active {
  transition: opacity 0.15s ease-out;
}
.overlay-leave-active {
  transition: opacity 0.1s ease-in;
}
.overlay-enter-from,
.overlay-leave-to {
  opacity: 0;
}
.overlay-enter-from .overlay-container {
  transform: scale(0.97);
}
</style>
