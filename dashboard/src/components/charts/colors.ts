const SEMANTIC_TOKENS = [
  'var(--color-accent)',
  'var(--color-ok)',
  'var(--color-warn)',
  'var(--color-err)',
] as const

const SEMANTIC_HUES = [262, 155, 80, 25]

export function groupColor(index: number): string {
  if (index < SEMANTIC_TOKENS.length) {
    return SEMANTIC_TOKENS[index] as string
  }
  const step = index - SEMANTIC_TOKENS.length
  const baseHue = SEMANTIC_HUES[step % SEMANTIC_HUES.length] ?? 262
  const rotation = 23 * Math.floor(step / SEMANTIC_HUES.length + 1)
  const hue = (baseHue + rotation) % 360
  return `oklch(0.62 0.16 ${hue})`
}
