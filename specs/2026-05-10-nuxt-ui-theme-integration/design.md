# Design

## Goal

Introduce Nuxt UI into the Vue/Vite dashboard as an optional supplemental component source while preserving PicoTera's existing visual language, density, theme switching, typography, and interaction conventions.

This is a frontend-only integration. It does not change backend APIs, generated OpenAPI types, routing, or the data layer.

## Current Baseline

The dashboard already uses Tailwind CSS v4 and a local primitive library in `dashboard/src/ui/`. The design tokens live in `dashboard/src/index.css` inside `@theme`, with runtime theme overrides under `:root[data-theme="..."]`. The current themes are:

- light
- dark
- solarized-light
- solarized-dark

The existing tokens are semantic and project-specific: `surface-*`, `ink-*`, `accent-*`, `ok-*`, `warn-*`, `err-*`, `line-*`, `sidebar-*`, and `overlay-bg`. Existing primitives use these tokens directly through Tailwind utilities such as `bg-surface-0`, `text-ink`, `border-line`, and `ring-accent/20`.

Nuxt UI's theme model is compatible with this foundation because it also builds on Tailwind CSS v4 `@theme` variables and semantic color aliases. In Vue/Vite projects, Nuxt UI is installed through `@nuxt/ui/vite` and configured in `vite.config.ts`.

## Integration Strategy

Nuxt UI will be introduced as a themed component toolbox, not as a visual redesign and not as a replacement for the current local primitives.

The integration keeps PicoTera's local `src/ui` barrel as the application-facing design system. Existing primitives continue to be maintained as handwritten PicoTera components. Feature views continue importing primitives from `src/ui`.

Nuxt UI is used only when a future screen needs a small number of components whose behavior is expensive to reproduce locally. Those components are wrapped in PicoTera-owned components before feature code uses them. The wrapper owns props, slots, copy, density, and theme classes so Nuxt UI defaults never become the dashboard's design language.

Nuxt UI components will be configured to use PicoTera's token names and component defaults:

- `primary` maps to the existing blue `accent` role.
- `secondary` maps to `surface`/neutral UI treatment rather than a new decorative hue.
- `success` maps to `ok`.
- `warning` maps to `warn`.
- `error` maps to `err`.
- `neutral` maps to the existing tinted neutral surface/ink/line system.

Nuxt UI expects color palettes with shade names from `50` to `950`. PicoTera currently has role tokens, not full palettes. The theme bridge will define Nuxt-compatible palette names in `@theme` by aliasing shades to the existing semantic variables. This avoids duplicating OKLCH values and preserves the active `data-theme` runtime behavior.

## Token Bridge

Add Nuxt UI after Tailwind in `dashboard/src/index.css`:

```css
@import "tailwindcss";
@import "@nuxt/ui";
@plugin "@tailwindcss/typography";
```

Then add Nuxt-compatible palettes inside the existing `@theme` block. The bridge uses aliases, not new color decisions. The implementation enumerates every required shade; this excerpt shows the mapping pattern:

```css
@theme {
  --color-primary-50: var(--color-accent-faint);
  --color-primary-100: var(--color-accent-faint);
  --color-primary-200: var(--color-accent-faint);
  --color-primary-300: var(--color-accent);
  --color-primary-400: var(--color-accent);
  --color-primary-500: var(--color-accent);
  --color-primary-600: var(--color-accent-strong);
  --color-primary-700: var(--color-accent-strong);
  --color-primary-800: var(--color-accent-ink);
  --color-primary-900: var(--color-accent-ink);
  --color-primary-950: var(--color-accent-ink);

  --color-success-50: var(--color-ok-faint);
  --color-success-500: var(--color-ok);
  --color-success-700: var(--color-ok-ink);

  --color-warning-50: var(--color-warn-faint);
  --color-warning-500: var(--color-warn);
  --color-warning-700: var(--color-warn-ink);

  --color-error-50: var(--color-err-faint);
  --color-error-500: var(--color-err);
  --color-error-700: var(--color-err-ink);

  --color-neutral-50: var(--color-surface-50);
  --color-neutral-100: var(--color-surface-100);
  --color-neutral-200: var(--color-line-soft);
  --color-neutral-300: var(--color-line);
  --color-neutral-400: var(--color-ink-faint);
  --color-neutral-500: var(--color-ink-muted);
  --color-neutral-600: var(--color-ink-muted);
  --color-neutral-700: var(--color-ink);
  --color-neutral-800: var(--color-ink);
  --color-neutral-900: var(--color-ink);
  --color-neutral-950: var(--color-ink);
}
```

Every Nuxt palette defines all required shade keys from `50` to `950`. The implementation fills intermediate shades explicitly using the closest existing PicoTera token. Because the shade variables reference existing semantic variables, all four dashboard themes continue to work through the current `data-theme` override blocks.

Do not introduce independent raw OKLCH values for Nuxt UI. When a missing contrast state requires a new value, add a new PicoTera token to every existing theme block and document it in `dashboard/DESIGN_SYSTEM.md`.

## Vite Plugin Configuration

Add Nuxt UI to `dashboard/vite.config.ts` using the Vue/Vite integration:

```ts
import ui from '@nuxt/ui/vite'

export default defineConfig({
  plugins: [
    vue(),
    vueDevTools(),
    tailwindcss(),
    ui({
      theme: {
        colors: ['primary', 'secondary', 'success', 'info', 'warning', 'error', 'neutral'],
      },
      ui: {
        colors: {
          primary: 'primary',
          secondary: 'neutral',
          success: 'success',
          info: 'primary',
          warning: 'warning',
          error: 'error',
          neutral: 'neutral',
        },
      },
    }),
  ],
})
```

The final Vite configuration has exactly one working Tailwind v4 pipeline and no duplicate utility generation warnings. During implementation, verify whether `@nuxt/ui/vite` owns Tailwind processing for the installed version; remove `@tailwindcss/vite` from the dashboard config when the Nuxt UI plugin provides the Tailwind pipeline.

## Nuxt UI Defaults

Create a Nuxt UI configuration module under `dashboard/src/ui/nuxt-ui.config.ts` and import it from `vite.config.ts`. Keep only the color registration inline when the plugin type contract requires inline values.

Defaults encode PicoTera's existing visual behavior for any Nuxt UI component that is used later:

- Buttons use `rounded-md`, `text-sm`, `font-medium`, `leading-none`, `transition-colors`, `shadow-xs`, and the existing focus outline pattern.
- Inputs use the existing border/ring treatment: `border-line`, `focus:border-accent`, `focus:ring-[3px]`, `focus:ring-accent/20`.
- Badges and chips keep `rounded-[5px]`, monospace type, and compact density.
- Cards use `bg-surface-0`, `border-line`, `rounded-xl`, `shadow-sm`, and `overflow-hidden`.
- Tables keep the existing row density and selected-row inset accent stripe.
- Overlays use `overlay-bg`.

Nuxt UI component names do not leak into feature views. The existing `src/ui` components remain the app-facing API. New Nuxt-backed wrappers are added only for specific future needs and expose PicoTera-owned props and slots.

## Usage Boundary

The first implementation wires Nuxt UI and validates theme coverage without replacing existing primitives.

Existing primitives remain handwritten:

- `Button`
- `Input`
- `Textarea`
- `Select`
- `Badge`
- `Tabs`
- `AutoDataTable`
- `DataTable`, `Tr`, `Th`, `Td`
- `SidePanel`
- `ConfirmDialog`
- `Overlay`
- `Icon` and icon registry
- domain-specific primitives such as `MoneyDisplay`, `CodeEditor`, and `ColumnFilter`

Nuxt UI can be used for future components that meet all of these conditions:

- The component has complex interaction behavior that is costly to maintain locally.
- The component can be fully themed through PicoTera tokens and Nuxt UI slot/class overrides.
- The component is wrapped under `dashboard/src/ui/` or a feature-owned wrapper before use.
- The wrapper keeps PicoTera's density, copy style, focus states, and strict input behavior.

Nuxt UI is not used for routine buttons, inputs, badges, tables, panels, and other primitives that are already implemented cleanly in PicoTera.

## Validation

The integration is accepted when:

- `pnpm --dir dashboard build` passes.
- `pnpm --dir dashboard type-check` passes.
- `pnpm --dir dashboard lint` passes after source changes.
- The dashboard renders correctly in all four existing themes.
- The optional Nuxt UI components available through wrappers match the current PicoTera density, radius, focus, and color treatment.
- No feature view imports Nuxt UI components directly.
- `dashboard/DESIGN_SYSTEM.md` documents the Nuxt UI bridge and the rule that `src/ui` remains the app-facing primitive layer.

Manual visual checks cover Providers, Requests, Request Detail, Scripts, Projects, and Preferences because they exercise tables, forms, side panels, badges, tabs, and theme switching.
