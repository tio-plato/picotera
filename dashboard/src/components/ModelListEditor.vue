<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { Tabs, IconButton, Icon } from '@/ui'

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

const tabs = [
  { value: 'rows' as const, label: '交互', icon: 'list' as const },
  { value: 'bulk' as const, label: '批量', icon: 'lines' as const },
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
        {{ entryCount }} {{ entryCount === 1 ? 'model' : 'models' }}
      </span>
    </div>

    <div v-if="mode === 'rows'" class="p-2 flex flex-col gap-4">
      <div v-if="entries.length === 0" class="py-3 px-2 text-xs text-ink-faint text-center">
        暂无模型
      </div>
      <div v-else class="flex flex-col gap-1">
        <div
          v-for="(row, idx) in entries"
          :key="row.id"
          class="grid grid-cols-[auto_minmax(0,1fr)_auto] items-center gap-2"
        >
          <span class="font-mono text-2xs text-ink-faint w-5 text-right tabular-nums select-none">{{ idx + 1 }}</span>
          <input
            v-model="row.name"
            class="min-w-0 px-2 py-1.5 border border-line rounded-[5px] bg-surface-0 font-mono text-xs text-ink transition-colors hover:border-surface-300 focus:outline-none focus:border-accent focus:ring-[3px] focus:ring-accent/20 placeholder:text-ink-faint"
            :placeholder="placeholder ?? '例如 gpt-4o'"
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
        class="inline-flex items-center gap-2 self-start pl-2 pr-2 py-1 bg-transparent border border-dashed border-line rounded-[5px] text-xs text-ink-muted cursor-pointer transition-colors hover:bg-accent-faint hover:text-accent-ink hover:border-accent/40 hover:border-solid [&_svg]:opacity-70 hover:[&_svg]:opacity-100"
        @click="addRow"
      >
        <Icon name="plus" :size="11" :stroke-width="1.6" />
        添加一行
      </button>
    </div>

    <div v-else class="p-2 flex flex-col gap-4">
      <textarea
        v-model="bulkText"
        class="w-full px-2.5 py-2 border border-line rounded-[5px] bg-surface-0 font-mono text-xs leading-[1.55] text-ink resize-y min-h-24 transition-colors focus:outline-none focus:border-accent focus:ring-[3px] focus:ring-accent/20 placeholder:text-ink-faint"
        spellcheck="false"
        autocomplete="off"
        rows="6"
        :placeholder="'gpt-4o\ngpt-4o-mini\no1-preview'"
        @input="onBulkInput"
      />
      <div class="text-2xs text-ink-faint px-0.5 pb-1">每行一个模型名，# 开头的行视为注释，重复项自动去重</div>
    </div>
  </div>
</template>
