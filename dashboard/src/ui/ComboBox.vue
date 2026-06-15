<script setup lang="ts">
import { ref, computed, useTemplateRef, watch, nextTick, onBeforeUnmount } from 'vue'
import { useFloating, offset, flip, shift, autoUpdate, size as sizeMiddleware } from '@floating-ui/vue'
import Icon from './icons/Icon.vue'

export interface ComboBoxOption {
  value: string
  label?: string
  hint?: string
}

const props = withDefaults(
  defineProps<{
    modelValue: string
    options: ComboBoxOption[]
    allowCustom?: boolean
    placeholder?: string
    disabled?: boolean
    size?: 'sm' | 'md'
  }>(),
  {
    allowCustom: false,
    placeholder: '',
    disabled: false,
    size: 'md',
  },
)

const emit = defineEmits<{ 'update:modelValue': [string] }>()

const open = ref(false)
const query = ref('')
const activeIndex = ref(-1)
const triggerRef = useTemplateRef<HTMLElement>('triggerRef')
const floatingRef = useTemplateRef<HTMLElement>('floatingRef')
const inputRef = useTemplateRef<HTMLInputElement>('inputRef')

const sizeClass = computed(() =>
  props.size === 'sm' ? 'px-2 py-1.5 text-sm' : 'px-3 py-2 text-sm',
)

const { floatingStyles } = useFloating(triggerRef, floatingRef, {
  placement: 'bottom-start',
  strategy: 'fixed',
  whileElementsMounted: autoUpdate,
  middleware: [
    offset(4),
    flip({ padding: 8 }),
    shift({ padding: 8 }),
    sizeMiddleware({
      apply({ availableHeight, rects, elements }) {
        elements.floating.style.maxHeight = `${Math.max(180, availableHeight - 8)}px`
        elements.floating.style.minWidth = `${rects.reference.width}px`
      },
      padding: 8,
    }),
  ],
})

const filtered = computed(() => {
  const q = query.value.trim().toLowerCase()
  if (!q) return props.options
  return props.options.filter((o) => {
    const hay = `${o.label ?? ''} ${o.hint ?? ''} ${o.value}`.toLowerCase()
    return hay.includes(q)
  })
})

// When custom input is allowed and the typed text is a non-empty value that does
// not exactly match an existing option, surface it as a selectable custom entry.
const customValue = computed(() => {
  if (!props.allowCustom) return null
  const q = query.value
  if (q.trim() === '') return null
  if (props.options.some((o) => o.value === q)) return null
  return q
})

interface DisplayItem extends ComboBoxOption {
  custom?: boolean
}

const displayItems = computed<DisplayItem[]>(() => {
  const items: DisplayItem[] = []
  if (customValue.value !== null) items.push({ value: customValue.value, custom: true })
  items.push(...filtered.value)
  return items
})

const displayLabel = computed(() => props.modelValue || props.placeholder)

function show() {
  if (props.disabled) return
  open.value = true
  query.value = props.modelValue
  activeIndex.value = -1
  nextTick(() => {
    inputRef.value?.focus()
    inputRef.value?.select()
  })
}

// Close without committing the in-progress query (Esc / post-pick).
function close() {
  open.value = false
}

// Close on blur / outside click: when custom values are allowed, commit the
// trimmed query (if non-empty); otherwise discard it and keep modelValue.
function commitAndClose() {
  if (props.allowCustom && query.value.trim() !== '') {
    if (query.value !== props.modelValue) emit('update:modelValue', query.value)
  }
  close()
}

function clearQuery() {
  query.value = ''
  inputRef.value?.focus()
}

function pick(value: string) {
  emit('update:modelValue', value)
  close()
  triggerRef.value?.focus()
}

function onDocMouseDown(e: MouseEvent) {
  const t = e.target as Node
  if (floatingRef.value?.contains(t)) return
  if (triggerRef.value?.contains(t)) return
  commitAndClose()
}

function onKeydown(e: KeyboardEvent) {
  if (!open.value) return
  if (e.key === 'Escape') {
    e.preventDefault()
    close()
    triggerRef.value?.focus()
  } else if (e.key === 'ArrowDown') {
    e.preventDefault()
    activeIndex.value = Math.min(displayItems.value.length - 1, activeIndex.value + 1)
  } else if (e.key === 'ArrowUp') {
    e.preventDefault()
    activeIndex.value = Math.max(0, activeIndex.value - 1)
  } else if (e.key === 'Enter') {
    const item = displayItems.value[activeIndex.value]
    if (item) {
      e.preventDefault()
      pick(item.value)
    } else if (props.allowCustom && query.value.trim() !== '') {
      e.preventDefault()
      pick(query.value)
    }
  }
}

watch(open, (v) => {
  if (v) {
    document.addEventListener('mousedown', onDocMouseDown, true)
    document.addEventListener('keydown', onKeydown)
  } else {
    document.removeEventListener('mousedown', onDocMouseDown, true)
    document.removeEventListener('keydown', onKeydown)
  }
})

watch(query, () => {
  activeIndex.value = -1
})

onBeforeUnmount(() => {
  document.removeEventListener('mousedown', onDocMouseDown, true)
  document.removeEventListener('keydown', onKeydown)
})
</script>

<template>
  <div ref="triggerRef" class="relative w-full">
    <button
      v-if="!open"
      type="button"
      :disabled="disabled"
      :aria-expanded="false"
      aria-haspopup="listbox"
      class="flex items-center justify-between gap-2 w-full border border-line rounded-md bg-surface-0 text-ink font-sans text-left transition-colors hover:border-surface-300 focus:outline-none focus:border-accent disabled:opacity-55 disabled:cursor-not-allowed"
      :class="sizeClass"
      @click="show"
      @focus="show"
    >
      <span class="truncate" :class="modelValue ? '' : 'text-ink-faint'">{{ displayLabel }}</span>
      <Icon name="chevron-down" :size="14" class="flex-none text-ink-faint" />
    </button>

    <div
      v-else
      class="flex items-center gap-2 w-full border border-accent rounded-md bg-surface-0 text-ink transition-colors focus-within:ring-[3px] focus-within:ring-accent/20"
      :class="sizeClass"
    >
      <input
        ref="inputRef"
        v-model="query"
        type="text"
        :placeholder="placeholder"
        class="flex-1 min-w-0 bg-transparent border-0 outline-none text-sm text-ink placeholder:text-ink-faint"
      />
      <button
        v-if="query !== ''"
        type="button"
        class="flex-none inline-flex items-center p-0 bg-transparent border-0 cursor-pointer text-ink-faint hover:text-ink"
        aria-label="清空"
        title="清空"
        @click="clearQuery"
      >
        <Icon name="close" :size="14" />
      </button>
      <Icon v-else name="chevron-down" :size="14" class="flex-none text-ink-faint rotate-180" />
    </div>

    <Teleport to="body">
      <div
        v-if="open"
        ref="floatingRef"
        class="flex flex-col bg-surface-0 border border-line rounded-xl shadow-lg z-[1000] overflow-hidden"
        role="listbox"
        :style="floatingStyles"
      >
        <div class="flex-1 overflow-y-auto py-1">
          <button
            v-for="(item, i) in displayItems"
            :key="(item.custom ? 'custom:' : 'opt:') + item.value"
            type="button"
            role="option"
            :aria-selected="item.value === modelValue"
            class="flex items-center justify-between gap-2 w-full px-2.5 py-1.5 bg-transparent border-0 text-left text-sm cursor-pointer transition-colors"
            :class="[
              item.value === modelValue
                ? 'bg-accent-faint text-accent-ink font-medium'
                : 'text-ink hover:bg-surface-50',
              activeIndex === i && item.value !== modelValue ? 'bg-surface-50' : '',
            ]"
            @mouseenter="activeIndex = i"
            @click="pick(item.value)"
          >
            <span v-if="item.custom" class="flex items-center gap-1.5 min-w-0">
              <span class="text-ink-muted flex-none">使用</span>
              <span class="font-mono truncate">"{{ item.value }}"</span>
            </span>
            <span v-else class="flex flex-col min-w-0 leading-tight">
              <span class="truncate">{{ item.label ?? item.value }}</span>
              <span v-if="item.hint" class="font-mono text-2xs text-ink-faint truncate">{{
                item.hint
              }}</span>
            </span>
            <span
              v-if="item.value === modelValue"
              class="inline-block w-1.5 h-1.5 rounded-full bg-accent flex-none"
              aria-hidden="true"
            />
          </button>
          <div
            v-if="displayItems.length === 0"
            class="px-2.5 py-3 text-center text-xs text-ink-faint"
          >
            无匹配项
          </div>
        </div>
      </div>
    </Teleport>
  </div>
</template>
