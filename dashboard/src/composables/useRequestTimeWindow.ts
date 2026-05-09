import { computed, reactive } from 'vue'

export const requestWindowPresets = [
  { value: '1h', label: '1 小时', hours: 1 },
  { value: '6h', label: '6 小时', hours: 6 },
  { value: '24h', label: '24 小时', hours: 24 },
  { value: '7d', label: '7 天', hours: 24 * 7 },
] as const

export type RequestWindowPreset = typeof requestWindowPresets[number]['value']

export function useRequestTimeWindow(initialPreset: RequestWindowPreset = '24h') {
  const initial = requestWindowPresets.find(p => p.value === initialPreset) ?? requestWindowPresets[2]
  const state = reactive({
    preset: initial.value as RequestWindowPreset,
    createdAtFrom: new Date(Date.now() - initial.hours * 60 * 60 * 1000).toISOString(),
    createdAtTo: new Date().toISOString(),
  })

  const label = computed(() => {
    const from = new Date(state.createdAtFrom)
    const to = new Date(state.createdAtTo)
    if (Number.isNaN(from.getTime()) || Number.isNaN(to.getTime())) return '时间范围无效'
    return `${formatWindowPoint(from)} - ${formatWindowPoint(to)}`
  })

  function applyPreset(preset: RequestWindowPreset = state.preset) {
    const next = requestWindowPresets.find(p => p.value === preset)
    if (!next) return
    state.preset = next.value
    const to = new Date()
    const from = new Date(to.getTime() - next.hours * 60 * 60 * 1000)
    state.createdAtFrom = from.toISOString()
    state.createdAtTo = to.toISOString()
  }

  return {
    requestWindow: state,
    requestWindowLabel: label,
    applyRequestWindowPreset: applyPreset,
  }
}

function formatWindowPoint(value: Date): string {
  const pad = (n: number) => String(n).padStart(2, '0')
  return `${value.getFullYear()}-${pad(value.getMonth() + 1)}-${pad(value.getDate())} ${pad(value.getHours())}:${pad(value.getMinutes())}`
}
