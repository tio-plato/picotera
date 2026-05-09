import { computed } from 'vue'
import { storeToRefs } from 'pinia'
import { usePreferencesStore } from '@/stores/preferences'
import { useExchangeRates } from '@/composables/useExchangeRates'

export interface ConvertResult {
  amount: number
  currency: string
  converted: boolean
  original: { amount: number; currency: string }
}

export function useCurrency() {
  const exchange = useExchangeRates()
  const prefs = usePreferencesStore()
  const { rates, byCode } = exchange
  const { displayCurrency } = storeToRefs(prefs)

  const targetCurrency = computed(() => displayCurrency.value ?? null)

  function unitsPerUsdFor(code: string): number | null {
    const rate = byCode.value.get(code)
    if (!rate) return null
    const u = rate.unitsPerUsd
    if (typeof u !== 'number' || !isFinite(u) || u <= 0) return null
    return u
  }

  function convert(amount: number, fromCode: string): ConvertResult {
    const original = { amount, currency: fromCode }
    const target = targetCurrency.value
    if (!target || target === fromCode) {
      return { amount, currency: fromCode, converted: false, original }
    }
    const fromUnits = unitsPerUsdFor(fromCode)
    const toUnits = unitsPerUsdFor(target)
    if (fromUnits == null || toUnits == null) {
      return { amount, currency: fromCode, converted: false, original }
    }
    const converted = (amount / fromUnits) * toUnits
    return { amount: converted, currency: target, converted: true, original }
  }

  function format(amount: number, code: string, opts?: { minDigits?: number; maxDigits?: number }): string {
    const symbol = byCode.value.get(code)?.symbol ?? ''
    const minDigits = opts?.minDigits ?? 2
    const maxDigits = opts?.maxDigits ?? 4
    const abs = Math.abs(amount)
    let effMax = maxDigits
    if (abs > 0 && abs < 0.0001) effMax = 6
    const formatted = new Intl.NumberFormat(undefined, {
      minimumFractionDigits: minDigits,
      maximumFractionDigits: Math.max(minDigits, effMax),
    }).format(amount)
    return symbol ? `${symbol}${formatted}` : `${formatted} ${code}`
  }

  return { rates, convert, format, targetCurrency }
}
