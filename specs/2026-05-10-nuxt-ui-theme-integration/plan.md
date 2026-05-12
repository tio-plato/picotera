# Plan

## 1. Install and Wire Nuxt UI

- Add `@nuxt/ui` to `dashboard/package.json`.
- Run the package manager install from the workspace root so `pnpm-lock.yaml` is updated.
- Add `@nuxt/ui/vite` to `dashboard/vite.config.ts`.
- Configure Nuxt UI semantic colors so `primary`, `success`, `warning`, `error`, and `neutral` point to PicoTera-owned palette names.
- Keep the existing Vue, Vue DevTools, and API/query plugins unchanged.

## 2. Add the Theme Bridge

- Import Nuxt UI CSS from `dashboard/src/index.css` after `@import "tailwindcss";`.
- Add full Nuxt-compatible shade palettes for `primary`, `success`, `warning`, `error`, and `neutral` inside the existing `@theme` block.
- Alias every Nuxt shade to an existing PicoTera semantic token.
- Register `secondary` and `info` as runtime semantic colors that point to existing palettes: `secondary` uses `neutral`, and `info` uses `primary`.
- Preserve the current `data-theme` override structure. Nuxt palette variables alias existing PicoTera tokens, so runtime theme switching stays centralized in the existing theme blocks.

## 3. Define Nuxt UI Defaults

- Create `dashboard/src/ui/nuxt-ui.config.ts` for Nuxt UI component defaults.
- Encode PicoTera defaults for Nuxt UI components that are introduced later.
- Match the existing design system: compact typography, 3px spacing scale, small radii, tinted neutral surfaces, blue accent, state colors, color-only transitions, and existing focus treatments.
- Keep Nuxt UI component overrides declarative and token-based. Do not hard-code raw OKLCH values in component classes.

## 4. Keep `src/ui` as the Application Boundary

- Keep existing feature views importing from `dashboard/src/ui`.
- Keep the existing `Button`, `Input`, `Textarea`, `Select`, `Badge`, `Tabs`, table, panel, overlay, and icon primitives handwritten.
- Do not import Nuxt UI components directly from route views or feature components.
- Re-export no Nuxt UI components from `src/ui/index.ts` until a local wrapper exists with PicoTera-compatible props and slots.
- Update `dashboard/DESIGN_SYSTEM.md` to document Nuxt UI as an optional supplemental component source and token consumer.

## 5. Add Guardrails for Future Use

- Add a short `dashboard/src/ui/README.md` section or `dashboard/DESIGN_SYSTEM.md` section that defines when Nuxt UI is allowed.
- Require a PicoTera wrapper before any Nuxt UI component is used in feature code.
- Allow Nuxt UI only for future components with complex behavior that is expensive to maintain locally.
- Keep routine controls, data display primitives, panels, overlays, icons, and domain-specific components in the existing handwritten style.
- Require every Nuxt-backed wrapper to use PicoTera tokens, PicoTera copy tone, existing focus states, and strict input behavior.

## 6. Verify Behavior and Visual Consistency

- Run `pnpm --dir dashboard type-check`.
- Run `pnpm --dir dashboard lint`.
- Run `pnpm --dir dashboard build`.
- Start `mise run web` or `pnpm --dir dashboard dev` and inspect the dashboard in light, dark, solarized-light, and solarized-dark themes.
- Check Providers, Requests, Request Detail, Scripts, Projects, and Preferences for unchanged layout density, focus rings, selected states, badges, tabs, and form controls.
- Fix any global CSS, contrast, spacing, or theme drift caused by importing Nuxt UI before adding any Nuxt-backed wrapper.

## 7. Document the Final Integration

- Update `dashboard/DESIGN_SYSTEM.md` with the Nuxt UI bridge section.
- Document that new UI work still starts with local `src/ui` primitives.
- Document the Nuxt palette-to-PicoTera-token mapping.
- Document that existing primitives are not migrated to Nuxt UI.
- Document that Nuxt UI is reserved for small, future, wrapper-based use cases.
- Document the rule that new raw color tokens must be added to every dashboard theme block.

## Non-Goals

- Do not migrate every dashboard component in the first pass.
- Do not migrate existing local primitives to Nuxt UI in this integration.
- Do not replace vue-query, Pinia, Vue Router, OpenAPI fetchers, or dashboard route structure.
- Do not introduce a second design language or Nuxt UI default visual style.
- Do not add compatibility fallbacks for old and new component systems. The local primitive contract remains stable and existing primitives stay handwritten.
- Do not relax existing input validation or add forgiving normalization while touching form components.
