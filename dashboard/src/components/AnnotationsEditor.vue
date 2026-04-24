<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { Tabs, IconButton, Icon } from '@/ui'

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

const tabs = [
  { value: 'rows' as const, label: '交互', icon: 'list' as const },
  { value: 'bulk' as const, label: '批量', icon: 'lines' as const },
  { value: 'json' as const, label: 'JSON', icon: 'braces' as const },
]

function onModeChange(v: string | number) {
  switchMode(v as Mode)
}
</script>

<template>
  <div class="flex flex-col gap-2 border border-line rounded-lg bg-surface-0 overflow-hidden">
    <div class="flex items-center justify-between py-1.5 pl-2 pr-1.5 bg-surface-50 border-b border-line">
      <Tabs :model-value="mode" :tabs="tabs" @update:model-value="onModeChange" />
      <span class="font-mono text-2xs text-ink-faint pr-2 tabular-nums">
        {{ entryCount }} {{ entryCount === 1 ? 'entry' : 'entries' }}
      </span>
    </div>

    <div v-if="mode === 'rows'" class="p-2 flex flex-col gap-1.5">
      <div v-if="entries.length === 0" class="py-3 px-2 text-xs text-ink-faint text-center">
        暂无标注
      </div>
      <div v-else class="flex flex-col gap-1">
        <div
          v-for="row in entries"
          :key="row.id"
          class="grid grid-cols-[minmax(0,1fr)_auto_minmax(0,1.5fr)_auto] items-center gap-1"
        >
          <input
            v-model="row.key"
            class="min-w-0 px-2 py-1.5 border border-line rounded-[5px] bg-surface-0 font-mono text-xs text-ink transition-colors hover:border-surface-300 focus:outline-none focus:border-accent focus:ring-[3px] focus:ring-accent/20 placeholder:text-ink-faint"
            placeholder="key"
            spellcheck="false"
            autocomplete="off"
            @input="onRowInput"
          />
          <span class="font-mono text-xs text-ink-faint px-px select-none">=</span>
          <input
            v-model="row.value"
            class="min-w-0 px-2 py-1.5 border border-line rounded-[5px] bg-surface-0 font-mono text-xs text-ink transition-colors hover:border-surface-300 focus:outline-none focus:border-accent focus:ring-[3px] focus:ring-accent/20 placeholder:text-ink-faint"
            placeholder="value"
            spellcheck="false"
            autocomplete="off"
            @input="onRowInput"
          />
          <IconButton variant="danger" size="sm" aria-label="删除此行" @click="removeRow(row.id)">
            <Icon name="close" :size="11" :stroke-width="1.6" />
          </IconButton>
        </div>
      </div>
      <button
        type="button"
        class="inline-flex items-center gap-1.5 self-start pl-2 pr-2 py-1 bg-transparent border border-dashed border-line rounded-[5px] text-xs text-ink-muted cursor-pointer transition-colors hover:bg-accent-faint hover:text-accent-ink hover:border-accent/40 hover:border-solid [&_svg]:opacity-70 hover:[&_svg]:opacity-100"
        @click="addRow"
      >
        <Icon name="plus" :size="11" :stroke-width="1.6" />
        添加一行
      </button>
    </div>

    <div v-else-if="mode === 'bulk'" class="p-2 flex flex-col gap-1.5">
      <textarea
        v-model="bulkText"
        class="w-full px-2.5 py-2 border border-line rounded-[5px] bg-surface-0 font-mono text-xs leading-[1.55] text-ink resize-y min-h-24 transition-colors focus:outline-none focus:border-accent focus:ring-[3px] focus:ring-accent/20 placeholder:text-ink-faint"
        spellcheck="false"
        autocomplete="off"
        rows="6"
        :placeholder="'KEY=value\nregion=us-east-1\ntier=premium'"
        @input="onBulkInput"
      />
      <div v-if="bulkError" class="text-2xs text-err-ink bg-err-faint px-2 py-1.5 rounded-sm font-mono">
        {{ bulkError }}
      </div>
      <div v-else class="text-2xs text-ink-faint px-0.5">
        每行一条
        <span class="font-mono px-1 py-px bg-surface-100 rounded-xs text-ink-muted">KEY=VALUE</span>
        ，# 开头的行视为注释
      </div>
    </div>

    <div v-else class="p-2 flex flex-col gap-1.5">
      <textarea
        v-model="jsonText"
        class="w-full px-2.5 py-2 border border-line rounded-[5px] bg-surface-0 font-mono text-xs leading-[1.55] text-ink resize-y min-h-24 transition-colors focus:outline-none focus:border-accent focus:ring-[3px] focus:ring-accent/20 placeholder:text-ink-faint"
        spellcheck="false"
        autocomplete="off"
        rows="6"
        placeholder='{ "region": "us-east-1" }'
        @input="onJsonInput"
      />
      <div v-if="jsonError" class="text-2xs text-err-ink bg-err-faint px-2 py-1.5 rounded-sm font-mono">
        {{ jsonError }}
      </div>
      <div v-else class="text-2xs text-ink-faint px-0.5">对象字面量，值必须为字符串</div>
    </div>
  </div>
</template>
