<script setup lang="ts" generic="Row">
import DataTable from './DataTable.vue'
import Th from './Th.vue'
import Td from './Td.vue'
import Tr from './Tr.vue'

export interface AutoDataTableColumn<R> {
  key: string
  header?: string
  field?: keyof R | string
  actions?: boolean
  align?: 'left' | 'right'
  headerClass?: string
  cellClass?: string
}

const props = defineProps<{
  columns: AutoDataTableColumn<Row>[]
  items: Row[]
  rowKey: (row: Row, index: number) => string | number
  selected?: (row: Row) => boolean
  hoverable?: boolean
  onRowClick?: (row: Row, event: MouseEvent) => void
}>()

defineSlots<{
  [K: `header-${string}`]: () => unknown
  [K: `cell-${string}`]: (p: { row: Row; value: unknown; index: number }) => unknown
  empty: () => unknown
}>()

function get(row: Row, path: keyof Row | string | undefined): unknown {
  if (path === undefined || path === null) return undefined
  const key = String(path)
  if (!key.includes('.')) {
    return (row as Record<string, unknown>)[key]
  }
  let current: unknown = row
  for (const part of key.split('.')) {
    if (current === null || current === undefined) return undefined
    current = (current as Record<string, unknown>)[part]
  }
  return current
}

function defaultFormat(value: unknown): string {
  if (value === null || value === undefined) return ''
  return String(value)
}

function handleRowClick(row: Row, event: MouseEvent) {
  props.onRowClick?.(row, event)
}
</script>

<template>
  <DataTable>
    <thead>
      <tr>
        <Th v-for="col in columns" :key="col.key" :actions="col.actions" :class="col.headerClass">
          <slot :name="`header-${col.key}`">{{ col.header ?? '' }}</slot>
        </Th>
      </tr>
    </thead>
    <tbody>
      <Tr
        v-for="(row, i) in items"
        :key="rowKey(row, i)"
        :selected="selected?.(row)"
        :hoverable="hoverable"
        :class="onRowClick ? 'cursor-pointer' : ''"
        @click="(event: MouseEvent) => handleRowClick(row, event)"
      >
        <Td
          v-for="col in columns"
          :key="col.key"
          :actions="col.actions"
          :class="[col.align === 'right' ? 'text-right' : '', col.cellClass]"
        >
          <slot :name="`cell-${col.key}`" :row="row" :value="get(row, col.field)" :index="i">{{
            defaultFormat(get(row, col.field))
          }}</slot>
        </Td>
      </Tr>
      <tr v-if="items.length === 0">
        <td :colspan="columns.length" class="px-4 py-10 text-center text-sm text-ink-faint">
          <slot name="empty">暂无数据</slot>
        </td>
      </tr>
    </tbody>
  </DataTable>
</template>
