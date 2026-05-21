import { inject, provide, type ComputedRef, type InjectionKey } from 'vue'
import type { ExchangeRateView } from '@/api'
import { useExchangeRates } from '@/composables/useExchangeRates'

export interface CurrencyFormatOptions {
  minDigits?: number
  maxDigits?: number
}

export interface ConvertResult {
  amount: number
  currency: string
  converted: boolean
  original: { amount: number; currency: string }
}

export interface CurrencyContext {
  rates: ComputedRef<ExchangeRateView[]>
  byCode: ComputedRef<Map<string, ExchangeRateView>>
  targetCurrency: ComputedRef<string | null>
  convert(amount: number, fromCode: string): ConvertResult
  convertTo(amount: number, fromCode: string, targetCode: string | null): ConvertResult
  format(amount: number, code: string, opts?: CurrencyFormatOptions): string
}

const currencyContextKey: InjectionKey<CurrencyContext> = Symbol('currencyContext')

function createCurrencyContext(targetCurrency: ComputedRef<string | null>): CurrencyContext {
  const exchange = useExchangeRates()
  const { rates, byCode } = exchange

  function unitsPerUsdFor(code: string): number | null {
    const rate = byCode.value.get(code)
    if (!rate) return null
    const u = rate.unitsPerUsd
    if (typeof u !== 'number' || !isFinite(u) || u <= 0) return null
    return u
  }

  function convertTo(amount: number, fromCode: string, targetCode: string | null): ConvertResult {
    const original = { amount, currency: fromCode }
    if (!targetCode || targetCode === fromCode) {
      return { amount, currency: fromCode, converted: false, original }
    }
    const fromUnits = unitsPerUsdFor(fromCode)
    const toUnits = unitsPerUsdFor(targetCode)
    if (fromUnits == null || toUnits == null) {
      return { amount, currency: fromCode, converted: false, original }
    }
    const converted = (amount / fromUnits) * toUnits
    return { amount: converted, currency: targetCode, converted: true, original }
  }

  function convert(amount: number, fromCode: string): ConvertResult {
    return convertTo(amount, fromCode, targetCurrency.value)
  }

  function format(amount: number, code: string, opts?: CurrencyFormatOptions): string {
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

  return { rates, byCode, targetCurrency, convert, convertTo, format }
}

export function provideCurrencyContext(
  targetCurrency: ComputedRef<string | null>,
): CurrencyContext {
  const ctx = createCurrencyContext(targetCurrency)
  provide(currencyContextKey, ctx)
  return ctx
}

export function useCurrencyContext(): CurrencyContext {
  const ctx = inject(currencyContextKey)
  if (!ctx) throw new Error('Currency context is not provided')
  return ctx
}
