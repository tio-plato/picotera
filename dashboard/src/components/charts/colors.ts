const CHART_COLOR_COUNT = 10

const FALLBACK_COLORS = [
  'oklch(0.54 0.19 262)',
  'oklch(0.62 0.15 155)',
  'oklch(0.65 0.15 80)',
  'oklch(0.58 0.19 25)',
  'oklch(0.60 0.14 195)',
  'oklch(0.55 0.18 300)',
  'oklch(0.62 0.16 350)',
  'oklch(0.60 0.14 120)',
  'oklch(0.52 0.17 235)',
  'oklch(0.60 0.15 55)',
]

function readCSSVar(name: string): string {
  return getComputedStyle(document.documentElement).getPropertyValue(name).trim()
}

export function groupColor(index: number): string {
  if (index < CHART_COLOR_COUNT) {
    return readCSSVar(`--color-chart-${index}`) || FALLBACK_COLORS[index]!
  }
  const base = readCSSVar(`--color-chart-${index % CHART_COLOR_COUNT}`) || FALLBACK_COLORS[index % CHART_COLOR_COUNT]!
  const hueMatch = base.match(/oklch\(([^ ]+) ([^ ]+) ([^ )]+)\)/)
  if (!hueMatch) return base
  const [, l, c, h] = hueMatch
  const rotation = 23 * Math.floor(index / CHART_COLOR_COUNT)
  const hue = (parseFloat(h!) + rotation) % 360
  return `oklch(${l} ${c} ${hue})`
}

export function getChartColors(): string[] {
  const colors: string[] = []
  for (let i = 0; i < CHART_COLOR_COUNT; i++) {
    colors.push(readCSSVar(`--color-chart-${i}`) || FALLBACK_COLORS[i]!)
  }
  return colors
}

export function getThemeAxisStyle() {
  return {
    axisLine: readCSSVar('--color-line') || '#e5e5e5',
    axisTick: readCSSVar('--color-line') || '#e5e5e5',
    axisLabel: readCSSVar('--color-ink-muted') || '#737373',
    splitLine: readCSSVar('--color-line-soft') || '#f0f0f0',
    tooltipBg: readCSSVar('--color-surface-0') || '#ffffff',
    tooltipBorder: readCSSVar('--color-line') || '#e5e5e5',
    tooltipText: readCSSVar('--color-ink') || '#1a1a1a',
  }
}
