export type ThemeKind = 'light' | 'dark'

export interface ThemeDef {
  /** data-theme slug applied to <html> */
  value: string
  /** display name in the preferences menu */
  label: string
  /** drives the data-dark flag and the menu grouping */
  kind: ThemeKind
  /** left half of the menu swatch (oklch literal) */
  surface: string
  /** right half of the menu swatch (oklch literal) */
  accent: string
}

// Single source of truth. Lights first, then darks; menu groups filter by kind
// and preserve this order. Adding a theme = one entry here + one
// :root[data-theme='…'] block in index.css.
export const THEMES = [
  {
    value: 'light',
    label: 'Pico Light',
    kind: 'light',
    surface: 'oklch(0.986 0.003 250)',
    accent: 'oklch(0.54 0.19 262)',
  },
  {
    value: 'solarized-light',
    label: 'Solarized Light',
    kind: 'light',
    surface: 'oklch(0.965 0.036 92)',
    accent: 'oklch(0.72 0.15 85)',
  },
  {
    value: 'vs-light',
    label: 'Visual Studio Light',
    kind: 'light',
    surface: 'oklch(0.985 0.001 250)',
    accent: 'oklch(0.5 0.16 250)',
  },
  {
    value: 'dark',
    label: 'Tera Dark',
    kind: 'dark',
    surface: 'oklch(0.22 0.02 255)',
    accent: 'oklch(0.70 0.18 262)',
  },
  {
    value: 'solarized-dark',
    label: 'Solarized Dark',
    kind: 'dark',
    surface: 'oklch(0.30 0.035 210)',
    accent: 'oklch(0.68 0.14 235)',
  },
  {
    value: 'vs-dark',
    label: 'Visual Studio Dark',
    kind: 'dark',
    surface: 'oklch(0.26 0.004 250)',
    accent: 'oklch(0.62 0.15 245)',
  },
] as const satisfies readonly ThemeDef[]

export type Theme = (typeof THEMES)[number]['value']

export const THEME_VALUES = THEMES.map((t) => t.value) as Theme[]

export function isDarkTheme(value: Theme): boolean {
  return THEMES.find((t) => t.value === value)?.kind === 'dark'
}
