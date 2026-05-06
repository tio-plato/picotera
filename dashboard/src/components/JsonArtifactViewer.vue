<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { createJSONEditor, Mode, type JSONEditorPropsOptional } from 'vanilla-jsoneditor'
import type { JsonEditor } from 'vanilla-jsoneditor'

const props = defineProps<{ value: unknown }>()

const target = ref<HTMLDivElement | null>(null)
let editor: JsonEditor | null = null

function editorProps(): JSONEditorPropsOptional {
  return {
    content: { json: props.value },
    mode: Mode.tree,
    readOnly: true,
    mainMenuBar: false,
    navigationBar: false,
    statusBar: false,
    tabSize: 2,
    indentation: 2,
  }
}

onMounted(() => {
  if (!target.value) return
  editor = createJSONEditor({
    target: target.value,
    props: editorProps(),
  })
})

watch(
  () => props.value,
  () => {
    editor?.updateProps(editorProps())
  },
)

onBeforeUnmount(() => {
  editor?.destroy()
  editor = null
})
</script>

<template>
  <div class="json-artifact-viewer overflow-hidden rounded-md border border-line-soft bg-surface-50">
    <div ref="target" />
  </div>
</template>

<style scoped>
.json-artifact-viewer {
  --jse-theme: light;
  --jse-theme-color: var(--color-accent);
  --jse-theme-color-highlight: var(--color-accent-strong);
  --jse-background-color: var(--color-surface-50);
  --jse-text-color: var(--color-ink);
  --jse-text-color-inverse: var(--color-surface-0);
  --jse-error-color: var(--color-err-ink);
  --jse-warning-color: var(--color-warn-ink);
  --jse-font-family: var(--font-sans);
  --jse-font-family-mono: var(--font-mono);
  --jse-font-size-mono: var(--text-xs);
  --jse-main-border: 0;

  --jse-menu-color: var(--color-surface-0);
  --jse-modal-background: var(--color-surface-0);
  --jse-modal-overlay-background: var(--color-overlay-bg);
  --jse-modal-code-background: var(--color-surface-50);

  --jse-tooltip-color: var(--color-ink);
  --jse-tooltip-background: var(--color-surface-200);
  --jse-tooltip-border: 1px solid var(--color-line);
  --jse-tooltip-action-button-color: var(--color-ink);
  --jse-tooltip-action-button-background: var(--color-surface-300);

  --jse-panel-background: var(--color-surface-50);
  --jse-panel-background-border: 1px solid var(--color-line-soft);
  --jse-panel-color: var(--color-ink);
  --jse-panel-color-readonly: var(--color-ink-faint);
  --jse-panel-border: 1px solid var(--color-line-soft);
  --jse-panel-button-color-highlight: var(--color-ink);
  --jse-panel-button-background-highlight: var(--color-surface-200);

  --jse-navigation-bar-background: var(--color-surface-100);
  --jse-navigation-bar-background-highlight: var(--color-surface-200);
  --jse-navigation-bar-dropdown-color: var(--color-ink);

  --jse-context-menu-background: var(--color-surface-0);
  --jse-context-menu-background-highlight: var(--color-surface-100);
  --jse-context-menu-separator-color: var(--color-line-soft);
  --jse-context-menu-color: var(--color-ink);
  --jse-context-menu-pointer-background: var(--color-surface-200);
  --jse-context-menu-pointer-background-highlight: var(--color-surface-300);
  --jse-context-menu-pointer-color: var(--color-ink);

  --jse-key-color: var(--color-accent-ink);
  --jse-value-color: var(--color-ink);
  --jse-value-color-number: var(--color-ok-ink);
  --jse-value-color-boolean: var(--color-accent-ink);
  --jse-value-color-null: var(--color-ink-faint);
  --jse-value-color-string: var(--color-warn-ink);
  --jse-value-color-url: var(--color-accent-ink);
  --jse-delimiter-color: var(--color-ink-muted);
  --jse-edit-outline: 2px solid var(--color-accent);

  --jse-selection-background-color: var(--color-accent-faint);
  --jse-selection-background-inactive-color: var(--color-surface-200);
  --jse-hover-background-color: var(--color-surface-100);
  --jse-active-line-background-color: var(--color-surface-100);
  --jse-search-match-background-color: var(--color-warn-faint);

  --jse-collapsed-items-background-color: var(--color-surface-100);
  --jse-collapsed-items-selected-background-color: var(--color-accent-faint);
  --jse-collapsed-items-link-color: var(--color-ink-muted);
  --jse-collapsed-items-link-color-highlight: var(--color-accent-ink);

  --jse-search-match-color: var(--color-warn-faint);
  --jse-search-match-outline: 1px solid var(--color-warn);
  --jse-search-match-active-color: var(--color-accent-faint);
  --jse-search-match-active-outline: 1px solid var(--color-accent);

  --jse-tag-background: var(--color-surface-200);
  --jse-tag-color: var(--color-ink-muted);

  --jse-table-header-background: var(--color-surface-100);
  --jse-table-header-background-highlight: var(--color-surface-200);
  --jse-table-row-odd-background: var(--color-surface-50);

  --jse-input-background: var(--color-surface-0);
  --jse-input-border: 1px solid var(--color-line);
  --jse-button-background: var(--color-accent);
  --jse-button-background-highlight: var(--color-accent-strong);
  --jse-button-color: var(--color-surface-0);
  --jse-button-secondary-background: var(--color-surface-100);
  --jse-button-secondary-background-highlight: var(--color-surface-200);
  --jse-button-secondary-background-disabled: var(--color-surface-300);
  --jse-button-secondary-color: var(--color-ink);
  --jse-a-color: var(--color-accent-ink);
  --jse-a-color-highlight: var(--color-accent-strong);

  --jse-svelte-select-background: var(--color-surface-0);
  --jse-svelte-select-border: 1px solid var(--color-line);
  --list-background: var(--color-surface-0);
  --item-hover-bg: var(--color-surface-100);
  --multi-item-bg: var(--color-surface-200);
  --input-color: var(--color-ink);
  --multi-clear-bg: var(--color-surface-300);
  --multi-item-clear-icon-color: var(--color-ink);
  --multi-item-outline: 1px solid var(--color-line);
  --list-shadow: var(--shadow-lg);

  --jse-color-picker-background: var(--color-surface-100);
  --jse-color-picker-border-box-shadow: var(--color-line) 0 0 0 1px;

  --jse-indent-size: 0.8rem;
  --jse-line-height: 1.2em;
}
:deep(.fa-icon) {
  width: 8px;
  height: 8px;
  margin-left: 1px;
  margin-bottom: 1px;
}

.json-artifact-viewer :deep(.jse-main) {
  height: auto !important;
  min-height: 0 !important;
}
</style>
