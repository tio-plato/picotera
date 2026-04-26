<script setup lang="ts" generic="V extends string | number">
import { ref, computed, useTemplateRef, watch, nextTick, onBeforeUnmount } from 'vue'
import { useFloating, offset, flip, shift, autoUpdate, size } from '@floating-ui/vue'
import Icon from './icons/Icon.vue'

export interface ColumnFilterOption<T extends string | number> {
  value: T
  label: string
  hint?: string
}

const props = withDefaults(
  defineProps<{
    label: string
    modelValue: V | ''
    options: ColumnFilterOption<V>[]
    emptyValue?: V | ''
    allLabel?: string
    placeholder?: string
    align?: 'left' | 'right'
    searchable?: boolean
    formatActive?: (value: V) => string
  }>(),
  {
    emptyValue: '',
    allLabel: '全部',
    placeholder: '过滤…',
    align: 'left',
    searchable: true,
  },
)

const emit = defineEmits<{ 'update:modelValue': [V | ''] }>()

const open = ref(false)
const query = ref('')
const triggerRef = useTemplateRef<HTMLElement>('triggerRef')
const floatingRef = useTemplateRef<HTMLElement>('floatingRef')
const inputRef = useTemplateRef<HTMLInputElement>('inputRef')
const activeIndex = ref(-1)

const { floatingStyles } = useFloating(triggerRef, floatingRef, {
  placement: computed(() => (props.align === 'right' ? 'bottom-end' : 'bottom-start')),
  strategy: 'fixed',
  whileElementsMounted: autoUpdate,
  middleware: [
    offset(4),
    flip({ padding: 8 }),
    shift({ padding: 8 }),
    size({
      apply({ availableHeight, elements }) {
        elements.floating.style.maxHeight = `${Math.max(180, availableHeight - 8)}px`
      },
      padding: 8,
    }),
  ],
})

const isActive = computed(
  () =>
    props.modelValue !== props.emptyValue &&
    props.modelValue !== '' &&
    props.modelValue !== undefined,
)

const activeLabel = computed(() => {
  if (!isActive.value) return ''
  if (props.formatActive) return props.formatActive(props.modelValue as V)
  const opt = props.options.find((o) => o.value === props.modelValue)
  return opt?.label ?? String(props.modelValue)
})

const filtered = computed(() => {
  const q = query.value.trim().toLowerCase()
  if (!q) return props.options
  return props.options.filter((o) => {
    const hay = `${o.label} ${o.hint ?? ''} ${o.value}`.toLowerCase()
    return hay.includes(q)
  })
})

function toggle() {
  if (open.value) close()
  else show()
}

function show() {
  open.value = true
  query.value = ''
  activeIndex.value = -1
  nextTick(() => inputRef.value?.focus())
}

function close() {
  open.value = false
}

function pick(value: V | '') {
  emit('update:modelValue', value)
  close()
}

function clear(e: Event) {
  e.stopPropagation()
  emit('update:modelValue', props.emptyValue)
}

function onDocMouseDown(e: MouseEvent) {
  const t = e.target as Node
  if (floatingRef.value?.contains(t)) return
  if (triggerRef.value?.contains(t)) return
  close()
}

function onKeydown(e: KeyboardEvent) {
  if (e.key === 'Escape') {
    e.preventDefault()
    close()
    triggerRef.value?.focus()
    return
  }
  if (!open.value) return
  if (e.key === 'ArrowDown') {
    e.preventDefault()
    activeIndex.value = Math.min(filtered.value.length - 1, activeIndex.value + 1)
  } else if (e.key === 'ArrowUp') {
    e.preventDefault()
    activeIndex.value = Math.max(0, activeIndex.value - 1)
  } else if (e.key === 'Enter') {
    const opt = filtered.value[activeIndex.value]
    if (opt) {
      e.preventDefault()
      pick(opt.value)
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
  activeIndex.value = filtered.value.length > 0 ? 0 : -1
})

onBeforeUnmount(() => {
  document.removeEventListener('mousedown', onDocMouseDown, true)
  document.removeEventListener('keydown', onKeydown)
})
</script>

<template>
  <button
    ref="triggerRef"
    type="button"
    :aria-expanded="open"
    aria-haspopup="listbox"
    class="group inline-flex items-center gap-1 -mx-1 px-1 py-0.5 max-w-full bg-transparent border-0 rounded-xs text-ink-muted text-xs font-medium uppercase tracking-[0.03em] cursor-pointer transition-colors hover:text-ink hover:bg-surface-100 focus:outline-none focus-visible:outline-2 focus-visible:outline-accent focus-visible:outline-offset-2"
    :class="[
      isActive ? 'text-accent-ink' : '',
      align === 'right' ? 'flex-row-reverse' : '',
    ]"
    @click="toggle"
  >
    <span class="inline-flex items-center gap-1 min-w-0">
      <span class="truncate">{{ label }}</span>
      <span
        v-if="isActive"
        class="inline-flex items-center max-w-[10rem] px-1 rounded-xs bg-accent-faint text-accent-ink font-mono normal-case tracking-normal text-2xs"
        :title="activeLabel"
      >
        <span class="truncate">{{ activeLabel }}</span>
      </span>
    </span>
    <span class="inline-flex items-center text-ink-faint group-hover:text-ink-muted">
      <button
        v-if="isActive"
        type="button"
        class="inline-flex items-center p-0 bg-transparent border-0 cursor-pointer text-ink-faint hover:text-ink"
        :aria-label="`清除${label}筛选`"
        :title="`清除${label}筛选`"
        @click="clear"
      >
        <Icon name="close" :size="11" />
      </button>
      <Icon
        v-else
        name="chevron-down"
        :size="11"
        class="transition-transform"
        :class="open ? 'rotate-180' : ''"
      />
    </span>
  </button>

  <Teleport to="body">
    <div
      v-if="open"
      ref="floatingRef"
      class="flex flex-col w-64 bg-surface-0 border border-line rounded-xl shadow-lg z-[1000] overflow-hidden"
      role="listbox"
      :style="floatingStyles"
    >
      <div v-if="searchable" class="flex items-center gap-1.5 px-2.5 py-2 border-b border-line-soft">
        <Icon name="search" :size="12" class="text-ink-faint flex-none" />
        <input
          ref="inputRef"
          v-model="query"
          type="text"
          :placeholder="placeholder"
          class="flex-1 min-w-0 bg-transparent border-0 outline-none text-sm text-ink placeholder:text-ink-faint"
        />
      </div>
      <div class="flex-1 overflow-y-auto py-1">
        <button
          type="button"
          role="option"
          :aria-selected="!isActive"
          class="flex items-center justify-between gap-2 w-full px-2.5 py-1.5 bg-transparent border-0 text-left text-sm text-ink-muted cursor-pointer transition-colors hover:bg-surface-50"
          :class="!isActive ? 'text-accent-ink font-medium' : ''"
          @click="pick(emptyValue)"
        >
          <span>{{ allLabel }}</span>
          <span
            v-if="!isActive"
            class="inline-block w-1.5 h-1.5 rounded-full bg-accent flex-none"
            aria-hidden="true"
          />
        </button>
        <div class="my-1 mx-2.5 h-px bg-line-soft" />
        <button
          v-for="(opt, i) in filtered"
          :key="String(opt.value)"
          type="button"
          role="option"
          :aria-selected="opt.value === modelValue"
          class="flex items-center justify-between gap-2 w-full px-2.5 py-1.5 bg-transparent border-0 text-left text-sm cursor-pointer transition-colors"
          :class="[
            opt.value === modelValue
              ? 'bg-accent-faint text-accent-ink font-medium'
              : 'text-ink hover:bg-surface-50',
            activeIndex === i && opt.value !== modelValue ? 'bg-surface-50' : '',
          ]"
          @mouseenter="activeIndex = i"
          @click="pick(opt.value)"
        >
          <span class="flex flex-col min-w-0 leading-tight">
            <span class="truncate">{{ opt.label }}</span>
            <span v-if="opt.hint" class="font-mono text-2xs text-ink-faint truncate">{{ opt.hint }}</span>
          </span>
          <span
            v-if="opt.value === modelValue"
            class="inline-block w-1.5 h-1.5 rounded-full bg-accent flex-none"
            aria-hidden="true"
          />
        </button>
        <div
          v-if="filtered.length === 0"
          class="px-2.5 py-3 text-center text-xs text-ink-faint"
        >
          无匹配项
        </div>
      </div>
    </div>
  </Teleport>
</template>
