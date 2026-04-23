import { defineStore } from 'pinia'
import { ref, watch } from 'vue'
import { updatePrimaryPalette } from '@primeuix/themes'

export type Theme = 'light' | 'solarized-light' | 'solarized-dark' | 'dark'
export type PanelMode = 'auto' | 'right' | 'modal'

const STORAGE_KEY = 'picotera.preferences'

const DEFAULTS = {
  theme: 'light' as Theme,
  panelMode: 'auto' as PanelMode,
}

const THEME_VALUES: Theme[] = ['light', 'solarized-light', 'solarized-dark', 'dark']
const PANEL_MODE_VALUES: PanelMode[] = ['auto', 'right', 'modal']

const PRIMARY_PALETTES: Record<Theme, Record<string, string>> = {
  light: {
    50: '{blue.50}',
    100: '{blue.100}',
    200: '{blue.200}',
    300: '{blue.300}',
    400: '{blue.400}',
    500: '{blue.500}',
    600: '{blue.600}',
    700: '{blue.700}',
    800: '{blue.800}',
    900: '{blue.900}',
    950: '{blue.950}',
  },
  'solarized-light': {
    50: '{amber.50}',
    100: '{amber.100}',
    200: '{amber.200}',
    300: '{amber.300}',
    400: '{amber.400}',
    500: '{amber.500}',
    600: '{amber.600}',
    700: '{amber.700}',
    800: '{amber.800}',
    900: '{amber.900}',
    950: '{amber.950}',
  },
  'solarized-dark': {
    50: '{amber.50}',
    100: '{amber.100}',
    200: '{amber.200}',
    300: '{amber.300}',
    400: '{amber.400}',
    500: '{amber.500}',
    600: '{amber.600}',
    700: '{amber.700}',
    800: '{amber.800}',
    900: '{amber.900}',
    950: '{amber.950}',
  },
  dark: {
    50: '{blue.50}',
    100: '{blue.100}',
    200: '{blue.200}',
    300: '{blue.300}',
    400: '{blue.400}',
    500: '{blue.500}',
    600: '{blue.600}',
    700: '{blue.700}',
    800: '{blue.800}',
    900: '{blue.900}',
    950: '{blue.950}',
  },
}

function load() {
  try {
    const raw = localStorage.getItem(STORAGE_KEY)
    if (!raw) return { ...DEFAULTS }
    const parsed = JSON.parse(raw) as Partial<typeof DEFAULTS>
    return {
      theme: THEME_VALUES.includes(parsed.theme as Theme) ? (parsed.theme as Theme) : DEFAULTS.theme,
      panelMode: PANEL_MODE_VALUES.includes(parsed.panelMode as PanelMode) ? (parsed.panelMode as PanelMode) : DEFAULTS.panelMode,
    }
  } catch {
    return { ...DEFAULTS }
  }
}

export const usePreferencesStore = defineStore('preferences', () => {
  const initial = load()
  const theme = ref<Theme>(initial.theme)
  const panelMode = ref<PanelMode>(initial.panelMode)

  function apply() {
    const root = document.documentElement
    root.dataset.theme = theme.value
    root.dataset.panelMode = panelMode.value
    const dark = theme.value === 'dark' || theme.value === 'solarized-dark'
    root.classList.toggle('p-dark', dark)
    updatePrimaryPalette(PRIMARY_PALETTES[theme.value])
  }

  function persist() {
    try {
      localStorage.setItem(
        STORAGE_KEY,
        JSON.stringify({ theme: theme.value, panelMode: panelMode.value }),
      )
    } catch {
      // ignore quota / privacy-mode errors
    }
  }

  watch([theme, panelMode], () => {
    apply()
    persist()
  })

  function init() {
    apply()
  }

  return { theme, panelMode, init }
})
