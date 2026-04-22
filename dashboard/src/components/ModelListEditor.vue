<script setup lang="ts">
import { computed, ref, watch } from 'vue'

type Mode = 'rows' | 'bulk'
type Entry = { id: number; name: string }

const props = defineProps<{
  modelValue: string[]
  placeholder?: string
}>()
const emit = defineEmits<{ 'update:modelValue': [string[]] }>()

const mode = ref<Mode>('rows')

let nextId = 0
const entries = ref<Entry[]>(toEntries(props.modelValue))
const bulkText = ref('')

function toEntries(list: string[] | null | undefined): Entry[] {
  return (list ?? []).map((name) => ({ id: nextId++, name: String(name ?? '') }))
}

function entriesToList(list: Entry[]): string[] {
  const seen = new Set<string>()
  const out: string[] = []
  for (const { name } of list) {
    const n = name.trim()
    if (!n) continue
    if (seen.has(n)) continue
    seen.add(n)
    out.push(n)
  }
  return out
}

function parseBulk(text: string): string[] {
  const out: string[] = []
  const seen = new Set<string>()
  for (const raw of text.split(/\r?\n/)) {
    const line = raw.trim()
    if (!line) continue
    if (line.startsWith('#')) continue
    if (seen.has(line)) continue
    seen.add(line)
    out.push(line)
  }
  return out
}

function listToBulk(list: Entry[]): string {
  return entriesToList(list).join('\n')
}

let lastEmitted = JSON.stringify(entriesToList(entries.value))

watch(
  () => props.modelValue,
  (val) => {
    const next = JSON.stringify(val ?? [])
    if (next === lastEmitted) return
    entries.value = toEntries(val)
    lastEmitted = next
  },
  { deep: true }
)

function emitUpdate() {
  const list = entriesToList(entries.value)
  lastEmitted = JSON.stringify(list)
  emit('update:modelValue', list)
}

function addRow() {
  entries.value.push({ id: nextId++, name: '' })
}

function removeRow(id: number) {
  const i = entries.value.findIndex((e) => e.id === id)
  if (i === -1) return
  entries.value.splice(i, 1)
  emitUpdate()
}

function onRowInput() {
  emitUpdate()
}

function switchMode(next: Mode) {
  if (next === mode.value) return

  if (mode.value === 'bulk') {
    const parsed = parseBulk(bulkText.value)
    entries.value = parsed.map((name) => ({ id: nextId++, name }))
    emitUpdate()
  } else {
    entries.value = entriesToList(entries.value).map((name) => ({ id: nextId++, name }))
    emitUpdate()
  }

  if (next === 'bulk') {
    bulkText.value = listToBulk(entries.value)
  }

  mode.value = next
}

function onBulkInput() {
  const parsed = parseBulk(bulkText.value)
  entries.value = parsed.map((name) => ({ id: nextId++, name }))
  emitUpdate()
}

const entryCount = computed(() => entriesToList(entries.value).length)
</script>

<template>
  <div class="ml-editor">
    <div class="ml-header">
      <div class="ml-tabs" role="tablist">
        <button
          type="button"
          role="tab"
          :aria-selected="mode === 'rows'"
          :class="['ml-tab', { 'ml-tab--active': mode === 'rows' }]"
          @click="switchMode('rows')"
        >
          <svg viewBox="0 0 16 16" width="11" height="11" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round"><path d="M2.5 4h11M2.5 8h11M2.5 12h11" /></svg>
          交互
        </button>
        <button
          type="button"
          role="tab"
          :aria-selected="mode === 'bulk'"
          :class="['ml-tab', { 'ml-tab--active': mode === 'bulk' }]"
          @click="switchMode('bulk')"
        >
          <svg viewBox="0 0 16 16" width="11" height="11" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round"><path d="M3 3.5h10M3 7h7M3 10.5h10M3 14h5" /></svg>
          批量
        </button>
      </div>
      <span class="ml-count">{{ entryCount }} {{ entryCount === 1 ? 'model' : 'models' }}</span>
    </div>

    <div v-if="mode === 'rows'" class="ml-rows">
      <div v-if="entries.length === 0" class="ml-empty">
        暂无模型
      </div>
      <div v-else class="ml-rows-list">
        <div v-for="(row, idx) in entries" :key="row.id" class="ml-row">
          <span class="ml-index">{{ idx + 1 }}</span>
          <input
            v-model="row.name"
            class="ml-input"
            :placeholder="placeholder ?? '例如 gpt-4o'"
            spellcheck="false"
            autocomplete="off"
            @input="onRowInput"
          />
          <button
            type="button"
            class="ml-row-remove"
            aria-label="删除此行"
            @click="removeRow(row.id)"
          >
            <svg viewBox="0 0 16 16" width="11" height="11" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linecap="round"><path d="M4 4l8 8M12 4l-8 8" /></svg>
          </button>
        </div>
      </div>
      <button type="button" class="ml-add" @click="addRow">
        <svg viewBox="0 0 16 16" width="11" height="11" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linecap="round"><path d="M8 3v10M3 8h10" /></svg>
        添加一行
      </button>
    </div>

    <div v-else class="ml-pane">
      <textarea
        v-model="bulkText"
        class="ml-textarea"
        spellcheck="false"
        autocomplete="off"
        rows="6"
        :placeholder="'gpt-4o\ngpt-4o-mini\no1-preview'"
        @input="onBulkInput"
      />
      <div class="ml-hint">每行一个模型名，# 开头的行视为注释，重复项自动去重</div>
    </div>
  </div>
</template>

<style scoped>
.ml-editor {
  display: flex;
  flex-direction: column;
  gap: 0.5rem;
  border: 1px solid var(--color-line);
  border-radius: 0.5rem;
  background: var(--color-surface-0);
  overflow: hidden;
}

.ml-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 0.3125rem 0.375rem 0.3125rem 0.5rem;
  background: var(--color-surface-50);
  border-bottom: 1px solid var(--color-line);
}

.ml-tabs {
  display: inline-flex;
  gap: 0.125rem;
  background: var(--color-surface-100);
  padding: 0.1875rem;
  border-radius: 0.375rem;
}

.ml-tab {
  display: inline-flex;
  align-items: center;
  gap: 0.3125rem;
  padding: 0.25rem 0.5625rem;
  background: transparent;
  border: none;
  border-radius: 0.25rem;
  font-size: 0.75rem;
  font-weight: 500;
  color: var(--color-ink-muted);
  cursor: pointer;
  transition: background 0.12s ease, color 0.12s ease;
}
.ml-tab:hover { color: var(--color-ink); }
.ml-tab--active {
  background: var(--color-surface-0);
  color: var(--color-ink);
  box-shadow: var(--shadow-xs);
}
.ml-tab svg { opacity: 0.7; }
.ml-tab--active svg { opacity: 1; color: var(--color-accent); }

.ml-count {
  font-family: var(--font-mono);
  font-size: 0.6875rem;
  color: var(--color-ink-faint);
  padding-right: 0.5rem;
  font-variant-numeric: tabular-nums;
}

/* Rows */
.ml-rows {
  padding: 0.5rem;
  display: flex;
  flex-direction: column;
  gap: 0.3125rem;
}
.ml-rows-list {
  display: flex;
  flex-direction: column;
  gap: 0.25rem;
}
.ml-empty {
  padding: 0.75rem 0.5rem;
  font-size: 0.75rem;
  color: var(--color-ink-faint);
  text-align: center;
}
.ml-row {
  display: grid;
  grid-template-columns: auto minmax(0, 1fr) auto;
  align-items: center;
  gap: 0.375rem;
}
.ml-index {
  font-family: var(--font-mono);
  font-size: 0.6875rem;
  color: var(--color-ink-faint);
  width: 1.25rem;
  text-align: right;
  font-variant-numeric: tabular-nums;
  user-select: none;
}
.ml-input {
  min-width: 0;
  padding: 0.3125rem 0.5rem;
  border: 1px solid var(--color-line);
  border-radius: 0.3125rem;
  background: var(--color-surface-0);
  font-family: var(--font-mono);
  font-size: 0.75rem;
  color: var(--color-ink);
  transition: border-color 0.12s ease, box-shadow 0.12s ease;
}
.ml-input::placeholder { color: var(--color-ink-faint); }
.ml-input:hover { border-color: var(--color-surface-300); }
.ml-input:focus {
  outline: none;
  border-color: var(--color-accent);
  box-shadow: 0 0 0 3px color-mix(in oklch, var(--color-accent) 18%, transparent);
}
.ml-row-remove {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 1.375rem;
  height: 1.375rem;
  background: transparent;
  border: 1px solid transparent;
  border-radius: 0.25rem;
  color: var(--color-ink-faint);
  cursor: pointer;
  transition: background 0.12s ease, color 0.12s ease, border-color 0.12s ease;
}
.ml-row-remove:hover {
  background: var(--color-indicator-err-faint);
  border-color: oklch(0.88 0.06 25);
  color: var(--color-indicator-err-ink);
}
.ml-add {
  display: inline-flex;
  align-items: center;
  gap: 0.375rem;
  align-self: flex-start;
  padding: 0.25rem 0.5rem 0.25rem 0.4375rem;
  background: transparent;
  border: 1px dashed var(--color-line);
  border-radius: 0.3125rem;
  font-size: 0.75rem;
  color: var(--color-ink-muted);
  cursor: pointer;
  transition: background 0.12s ease, color 0.12s ease, border-color 0.12s ease;
}
.ml-add:hover {
  background: var(--color-accent-faint);
  color: var(--color-accent-ink);
  border-color: color-mix(in oklch, var(--color-accent) 35%, transparent);
  border-style: solid;
}
.ml-add svg { opacity: 0.7; }
.ml-add:hover svg { opacity: 1; }

/* Bulk pane */
.ml-pane {
  padding: 0.5rem;
  display: flex;
  flex-direction: column;
  gap: 0.375rem;
}
.ml-textarea {
  width: 100%;
  padding: 0.5rem 0.625rem;
  border: 1px solid var(--color-line);
  border-radius: 0.3125rem;
  background: var(--color-surface-0);
  font-family: var(--font-mono);
  font-size: 0.75rem;
  line-height: 1.55;
  color: var(--color-ink);
  resize: vertical;
  min-height: 6rem;
  transition: border-color 0.12s ease, box-shadow 0.12s ease;
  tab-size: 2;
}
.ml-textarea::placeholder { color: var(--color-ink-faint); }
.ml-textarea:focus {
  outline: none;
  border-color: var(--color-accent);
  box-shadow: 0 0 0 3px color-mix(in oklch, var(--color-accent) 18%, transparent);
}

.ml-hint {
  font-size: 0.6875rem;
  color: var(--color-ink-faint);
  padding: 0 0.125rem;
}
</style>
