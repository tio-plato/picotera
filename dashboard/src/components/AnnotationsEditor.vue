<script setup lang="ts">
import { computed, ref, watch } from 'vue'

type Mode = 'rows' | 'bulk' | 'json'
type Entry = { id: number; key: string; value: string }
type Pair = { key: string; value: string }

const props = defineProps<{
  modelValue: Record<string, string>
  placeholder?: string
}>()
const emit = defineEmits<{ 'update:modelValue': [Record<string, string>] }>()

const mode = ref<Mode>('rows')

let nextId = 0
const entries = ref<Entry[]>(toEntries(props.modelValue))
const bulkText = ref('')
const bulkError = ref('')
const jsonText = ref('')
const jsonError = ref('')

function toEntries(obj: Record<string, string>): Entry[] {
  return Object.entries(obj ?? {}).map(([k, v]) => ({ id: nextId++, key: k, value: String(v ?? '') }))
}

function entriesToObj(list: Pair[]): Record<string, string> {
  const out: Record<string, string> = {}
  for (const { key, value } of list) {
    const k = key.trim()
    if (!k) continue
    out[k] = value
  }
  return out
}

function entriesToBulk(list: Pair[]): string {
  return list
    .filter((e) => e.key.trim())
    .map((e) => `${e.key.trim()}=${e.value}`)
    .join('\n')
}

function entriesToJson(list: Pair[]): string {
  const obj = entriesToObj(list)
  return Object.keys(obj).length ? JSON.stringify(obj, null, 2) : '{}'
}

function parseBulk(text: string): { ok: true; entries: Pair[] } | { ok: false; error: string } {
  const lines = text.split(/\r?\n/)
  const out: Pair[] = []
  for (let i = 0; i < lines.length; i++) {
    const raw = lines[i]
    const line = raw.trim()
    if (!line) continue
    if (line.startsWith('#')) continue
    const eq = line.indexOf('=')
    if (eq === -1) return { ok: false, error: `第 ${i + 1} 行缺少 "="` }
    const key = line.slice(0, eq).trim()
    const value = line.slice(eq + 1)
    if (!key) return { ok: false, error: `第 ${i + 1} 行 key 为空` }
    out.push({ key, value })
  }
  return { ok: true, entries: out }
}

function parseJson(text: string): { ok: true; entries: Pair[] } | { ok: false; error: string } {
  const trimmed = text.trim()
  if (!trimmed) return { ok: true, entries: [] }
  let parsed: unknown
  try {
    parsed = JSON.parse(trimmed)
  } catch (e) {
    return { ok: false, error: (e as Error).message }
  }
  if (parsed === null || typeof parsed !== 'object' || Array.isArray(parsed)) {
    return { ok: false, error: '必须是对象 { ... }' }
  }
  const out: Pair[] = []
  for (const [k, v] of Object.entries(parsed as Record<string, unknown>)) {
    if (typeof v !== 'string') {
      return { ok: false, error: `键 "${k}" 的值必须是字符串` }
    }
    out.push({ key: k, value: v })
  }
  return { ok: true, entries: out }
}

let lastEmitted = JSON.stringify(entriesToObj(entries.value))

watch(
  () => props.modelValue,
  (val) => {
    const next = JSON.stringify(val ?? {})
    if (next === lastEmitted) return
    entries.value = toEntries(val)
    lastEmitted = next
  },
  { deep: true }
)

function emitUpdate() {
  const obj = entriesToObj(entries.value)
  lastEmitted = JSON.stringify(obj)
  emit('update:modelValue', obj)
}

function addRow() {
  entries.value.push({ id: nextId++, key: '', value: '' })
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

  if (mode.value === 'rows') {
    entries.value = toEntries(entriesToObj(entries.value))
    emitUpdate()
  } else if (mode.value === 'bulk') {
    const r = parseBulk(bulkText.value)
    if (!r.ok) {
      bulkError.value = r.error
      return
    }
    entries.value = r.entries.map((e) => ({ id: nextId++, ...e }))
    emitUpdate()
  } else if (mode.value === 'json') {
    const r = parseJson(jsonText.value)
    if (!r.ok) {
      jsonError.value = r.error
      return
    }
    entries.value = r.entries.map((e) => ({ id: nextId++, ...e }))
    emitUpdate()
  }

  if (next === 'bulk') {
    bulkText.value = entriesToBulk(entries.value)
    bulkError.value = ''
  } else if (next === 'json') {
    jsonText.value = entriesToJson(entries.value)
    jsonError.value = ''
  }

  mode.value = next
}

function onBulkInput() {
  const r = parseBulk(bulkText.value)
  if (!r.ok) {
    bulkError.value = r.error
    return
  }
  bulkError.value = ''
  entries.value = r.entries.map((e) => ({ id: nextId++, ...e }))
  emitUpdate()
}

function onJsonInput() {
  const r = parseJson(jsonText.value)
  if (!r.ok) {
    jsonError.value = r.error
    return
  }
  jsonError.value = ''
  entries.value = r.entries.map((e) => ({ id: nextId++, ...e }))
  emitUpdate()
}

const entryCount = computed(() => entries.value.filter((e) => e.key.trim()).length)
</script>

<template>
  <div class="anno-editor">
    <div class="anno-header">
      <div class="anno-tabs" role="tablist">
        <button
          type="button"
          role="tab"
          :aria-selected="mode === 'rows'"
          :class="['anno-tab', { 'anno-tab--active': mode === 'rows' }]"
          @click="switchMode('rows')"
        >
          <svg viewBox="0 0 16 16" width="11" height="11" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round"><path d="M2.5 4h11M2.5 8h11M2.5 12h11" /></svg>
          交互
        </button>
        <button
          type="button"
          role="tab"
          :aria-selected="mode === 'bulk'"
          :class="['anno-tab', { 'anno-tab--active': mode === 'bulk' }]"
          @click="switchMode('bulk')"
        >
          <svg viewBox="0 0 16 16" width="11" height="11" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round"><path d="M3 3.5h10M3 7h7M3 10.5h10M3 14h5" /></svg>
          批量
        </button>
        <button
          type="button"
          role="tab"
          :aria-selected="mode === 'json'"
          :class="['anno-tab', { 'anno-tab--active': mode === 'json' }]"
          @click="switchMode('json')"
        >
          <svg viewBox="0 0 16 16" width="11" height="11" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round"><path d="M5.5 3C4 3 3 4 3 5.5V7c0 .8-.5 1.3-1 1.3C2.5 8.3 3 8.8 3 9.6v1.4C3 12.5 4 13 5.5 13" /><path d="M10.5 3C12 3 13 4 13 5.5V7c0 .8.5 1.3 1 1.3-.5 0-1 .5-1 1.3v1.4c0 1.5-1 2-2.5 2" /></svg>
          JSON
        </button>
      </div>
      <span class="anno-count">{{ entryCount }} {{ entryCount === 1 ? 'entry' : 'entries' }}</span>
    </div>

    <div v-if="mode === 'rows'" class="anno-rows">
      <div v-if="entries.length === 0" class="anno-empty">
        暂无标注
      </div>
      <div v-else class="anno-rows-list">
        <div v-for="row in entries" :key="row.id" class="anno-row">
          <input
            v-model="row.key"
            class="anno-input anno-input--key"
            placeholder="key"
            spellcheck="false"
            autocomplete="off"
            @input="onRowInput"
          />
          <span class="anno-eq">=</span>
          <input
            v-model="row.value"
            class="anno-input anno-input--value"
            placeholder="value"
            spellcheck="false"
            autocomplete="off"
            @input="onRowInput"
          />
          <button
            type="button"
            class="anno-row-remove"
            aria-label="删除此行"
            @click="removeRow(row.id)"
          >
            <svg viewBox="0 0 16 16" width="11" height="11" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linecap="round"><path d="M4 4l8 8M12 4l-8 8" /></svg>
          </button>
        </div>
      </div>
      <button type="button" class="anno-add" @click="addRow">
        <svg viewBox="0 0 16 16" width="11" height="11" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linecap="round"><path d="M8 3v10M3 8h10" /></svg>
        添加一行
      </button>
    </div>

    <div v-else-if="mode === 'bulk'" class="anno-pane">
      <textarea
        v-model="bulkText"
        class="anno-textarea"
        spellcheck="false"
        autocomplete="off"
        rows="6"
        :placeholder="'KEY=value\nregion=us-east-1\ntier=premium'"
        @input="onBulkInput"
      />
      <div v-if="bulkError" class="anno-err">{{ bulkError }}</div>
      <div v-else class="anno-hint">每行一条 <span class="anno-hint-code">KEY=VALUE</span>，# 开头的行视为注释</div>
    </div>

    <div v-else class="anno-pane">
      <textarea
        v-model="jsonText"
        class="anno-textarea"
        spellcheck="false"
        autocomplete="off"
        rows="6"
        placeholder='{ "region": "us-east-1" }'
        @input="onJsonInput"
      />
      <div v-if="jsonError" class="anno-err">{{ jsonError }}</div>
      <div v-else class="anno-hint">对象字面量，值必须为字符串</div>
    </div>
  </div>
</template>

<style scoped>
.anno-editor {
  display: flex;
  flex-direction: column;
  gap: 0.5rem;
  border: 1px solid var(--color-line);
  border-radius: 0.5rem;
  background: var(--color-surface-0);
  overflow: hidden;
}

.anno-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 0.3125rem 0.375rem 0.3125rem 0.5rem;
  background: var(--color-surface-50);
  border-bottom: 1px solid var(--color-line);
}

.anno-tabs {
  display: inline-flex;
  gap: 0.125rem;
  background: var(--color-surface-100);
  padding: 0.1875rem;
  border-radius: 0.375rem;
}

.anno-tab {
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
.anno-tab:hover { color: var(--color-ink); }
.anno-tab--active {
  background: var(--color-surface-0);
  color: var(--color-ink);
  box-shadow: var(--shadow-xs);
}
.anno-tab svg { opacity: 0.7; }
.anno-tab--active svg { opacity: 1; color: var(--color-accent); }

.anno-count {
  font-family: var(--font-mono);
  font-size: 0.6875rem;
  color: var(--color-ink-faint);
  padding-right: 0.5rem;
  font-variant-numeric: tabular-nums;
}

/* Rows */
.anno-rows {
  padding: 0.5rem;
  display: flex;
  flex-direction: column;
  gap: 0.3125rem;
}
.anno-rows-list {
  display: flex;
  flex-direction: column;
  gap: 0.25rem;
}
.anno-empty {
  padding: 0.75rem 0.5rem;
  font-size: 0.75rem;
  color: var(--color-ink-faint);
  text-align: center;
}
.anno-row {
  display: grid;
  grid-template-columns: minmax(0, 1fr) auto minmax(0, 1.5fr) auto;
  align-items: center;
  gap: 0.25rem;
}
.anno-input {
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
.anno-input::placeholder { color: var(--color-ink-faint); }
.anno-input:hover { border-color: var(--color-surface-300); }
.anno-input:focus {
  outline: none;
  border-color: var(--color-accent);
  box-shadow: 0 0 0 3px color-mix(in oklch, var(--color-accent) 18%, transparent);
}
.anno-eq {
  font-family: var(--font-mono);
  font-size: 0.75rem;
  color: var(--color-ink-faint);
  padding: 0 0.0625rem;
  user-select: none;
}
.anno-row-remove {
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
.anno-row-remove:hover {
  background: var(--color-indicator-err-faint);
  border-color: oklch(0.88 0.06 25);
  color: var(--color-indicator-err-ink);
}
.anno-add {
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
.anno-add:hover {
  background: var(--color-accent-faint);
  color: var(--color-accent-ink);
  border-color: color-mix(in oklch, var(--color-accent) 35%, transparent);
  border-style: solid;
}
.anno-add svg { opacity: 0.7; }
.anno-add:hover svg { opacity: 1; }

/* Bulk & JSON pane */
.anno-pane {
  padding: 0.5rem;
  display: flex;
  flex-direction: column;
  gap: 0.375rem;
}
.anno-textarea {
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
.anno-textarea::placeholder { color: var(--color-ink-faint); }
.anno-textarea:focus {
  outline: none;
  border-color: var(--color-accent);
  box-shadow: 0 0 0 3px color-mix(in oklch, var(--color-accent) 18%, transparent);
}

.anno-hint {
  font-size: 0.6875rem;
  color: var(--color-ink-faint);
  padding: 0 0.125rem;
}
.anno-hint-code {
  font-family: var(--font-mono);
  padding: 0.0625rem 0.25rem;
  background: var(--color-surface-100);
  border-radius: 0.1875rem;
  color: var(--color-ink-muted);
}

.anno-err {
  font-size: 0.6875rem;
  color: var(--color-indicator-err-ink);
  background: var(--color-indicator-err-faint);
  padding: 0.3125rem 0.5rem;
  border-radius: 0.25rem;
  font-family: var(--font-mono);
}
</style>
