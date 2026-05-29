<script setup lang="ts">
import { ref } from 'vue'
import { SidePanel, Button, Input, Field } from '@/ui'
import type { ExchangeRateView } from '@/api'
import { useExchangeRates } from '@/composables/useExchangeRates'

const emit = defineEmits<{ close: [] }>()
const props = defineProps<{
  rate?: ExchangeRateView
}>()

const isEdit = !!props.rate
const exchange = useExchangeRates()

const form = ref({
  code: props.rate?.code ?? '',
  name: props.rate?.name ?? '',
  symbol: props.rate?.symbol ?? '',
  unitsPerUsd: props.rate?.unitsPerUsd ?? 1,
})
const saving = ref(false)
const error = ref('')

async function submit() {
  saving.value = true
  error.value = ''
  try {
    await exchange.upsert({
      code: form.value.code,
      name: form.value.name,
      symbol: form.value.symbol,
      unitsPerUsd: Number(form.value.unitsPerUsd),
    })
    emit('close')
  } catch (e: unknown) {
    const msg = e instanceof Error ? e.message : '操作失败'
    error.value = msg
  } finally {
    saving.value = false
  }
}
</script>

<template>
  <SidePanel
    :title="isEdit ? form.code : '新增货币'"
    :kicker="isEdit ? '编辑汇率' : '汇率'"
    @close="emit('close')"
  >
    <form id="rate-form" class="flex flex-col gap-4" @submit.prevent="submit">
      <Field label="货币代码">
        <Input
          v-model="form.code"
          required
          placeholder="USD / CNY / EUR"
          :disabled="isEdit"
          maxlength="6"
        />
      </Field>
      <Field label="名称">
        <Input v-model="form.name" required placeholder="例如 美元 / 人民币" />
      </Field>
      <Field label="符号">
        <Input v-model="form.symbol" required placeholder="例如 $ / ¥" maxlength="4" />
      </Field>
      <Field label="对 USD 汇率">
        <Input
          v-model.number="form.unitsPerUsd"
          required
          type="number"
          step="0.0001"
          min="0"
          placeholder="例如 7.2"
        />
        <span class="text-xs text-ink-faint">1 USD 等于多少单位的该币种；USD 自身固定为 1。</span>
      </Field>
    </form>

    <template v-if="error" #error>{{ error }}</template>

    <template #footer>
      <Button variant="ghost" @click="emit('close')">取消</Button>
      <Button type="submit" form="rate-form" :disabled="saving">
        {{ saving ? '保存中…' : isEdit ? '更新' : '创建' }}
      </Button>
    </template>
  </SidePanel>
</template>
