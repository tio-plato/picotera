import { defineStore } from 'pinia'
import { computed, ref } from 'vue'
import type { ExchangeRateView } from '@/api'
import { createApi } from '@/api/plugin'

const api = createApi()

export const useExchangeRatesStore = defineStore('exchangeRates', () => {
  const rates = ref<ExchangeRateView[]>([])
  const loaded = ref(false)
  const loading = ref(false)

  const byCode = computed(() => {
    const map = new Map<string, ExchangeRateView>()
    for (const rate of rates.value) map.set(rate.code, rate)
    return map
  })

  async function fetch() {
    loading.value = true
    try {
      const { data, error } = await api.GET('/api/picotera/exchange-rates')
      if (error) throw new Error('failed to load exchange rates')
      rates.value = data ?? []
      loaded.value = true
    } finally {
      loading.value = false
    }
  }

  async function upsert(rate: ExchangeRateView) {
    const { error } = await api.PUT('/api/picotera/exchange-rates', { body: rate })
    if (error) throw error
    await fetch()
  }

  async function remove(code: string) {
    const { error } = await api.POST('/api/picotera/exchange-rates/delete', { body: { code } })
    if (error) throw error
    await fetch()
  }

  return { rates, loaded, loading, byCode, fetch, upsert, remove }
})
