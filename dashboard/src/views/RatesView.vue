<script setup lang="ts">
import { computed } from 'vue'
import { useConfirm } from '@/composables/useConfirm'
import { useSidePanel } from '@/composables/useSidePanel'
import { useExchangeRates } from '@/composables/useExchangeRates'
import type { ExchangeRateView } from '@/api'
import RateForm from '@/components/RateForm.vue'
import { Button, IconButton, DataCard, DataTable, Th, Td, Tr, StateText, Tag, Icon } from '@/ui'

const panel = useSidePanel()
const confirm = useConfirm()
const exchange = useExchangeRates()
const { rates, loaded, loading, removeMutation } = exchange

const count = computed(() => rates.value.length)

function openCreate() {
  panel.open(RateForm, {}, { key: 'rate:new' })
}

function openEdit(r: ExchangeRateView) {
  panel.open(RateForm, { rate: r }, { key: `rate:${r.code}` })
}

function confirmDelete(_event: Event, r: ExchangeRateView) {
  if (r.code === 'USD') return
  confirm.require({
    message: `确定要删除汇率「${r.code}」吗？`,
    accept: async () => {
      try {
        await exchange.remove(r.code)
      } catch {
        // surfaced in future via toast; ignore for now
      }
    },
  })
}
</script>

<template>
  <div class="flex flex-col gap-3.5">
    <div class="flex items-center justify-between gap-3">
      <span class="text-xs text-ink-faint tabular-nums">{{ count }} 种货币</span>
      <Button @click="openCreate">
        <Icon name="plus" :size="14" :stroke-width="2.2" />
        <span>新增汇率</span>
      </Button>
    </div>
    <StateText v-if="loading && !loaded">加载中…</StateText>
    <DataCard v-else-if="rates.length">
      <DataTable>
        <thead>
          <tr>
            <Th>代码</Th>
            <Th>名称</Th>
            <Th>符号</Th>
            <Th>对 USD 汇率</Th>
            <Th actions />
          </tr>
        </thead>
        <tbody>
          <Tr v-for="r in rates" :key="r.code" :selected="panel.isActive(`rate:${r.code}`)">
            <Td>
              <span class="font-mono font-medium">{{ r.code }}</span>
              <Tag v-if="r.code === 'USD'" variant="muted" class="ml-1.5">基准</Tag>
            </Td>
            <Td>{{ r.name }}</Td>
            <Td
              ><span class="font-mono">{{ r.symbol }}</span></Td
            >
            <Td>
              <span class="tabular-nums">{{ r.unitsPerUsd }}</span>
            </Td>
            <Td actions>
              <div class="inline-flex gap-1 opacity-55 group-hover:opacity-100 transition-opacity">
                <IconButton
                  :active="panel.isActive(`rate:${r.code}`)"
                  title="编辑"
                  aria-label="编辑"
                  @click="openEdit(r)"
                >
                  <Icon name="edit" :size="13" />
                </IconButton>
                <IconButton
                  variant="danger"
                  :disabled="r.code === 'USD' || removeMutation.isPending.value"
                  :title="r.code === 'USD' ? '基准货币不可删除' : '删除'"
                  aria-label="删除"
                  @click="(e: Event) => confirmDelete(e, r)"
                >
                  <Icon name="trash" :size="13" />
                </IconButton>
              </div>
            </Td>
          </Tr>
        </tbody>
      </DataTable>
    </DataCard>
    <StateText v-else>暂无汇率，点击右上角按钮新增</StateText>
  </div>
</template>
