# Plan

## 1. Install and bootstrap Vue Query

1. Add `@tanstack/vue-query` to `dashboard/package.json` with pnpm.
2. Create `dashboard/src/api/queryClient.ts` with a shared `QueryClient` and default query options.
3. Install `VueQueryPlugin` in `dashboard/src/main.ts` before mounting the app.
4. Keep the existing `apiPlugin` so `openapi-fetch` remains injectable during the migration.

## 2. Add typed query infrastructure

1. Create `dashboard/src/api/queryKeys.ts` with stable key factories for providers, endpoints, models, provider endpoints, scripts, API keys, exchange rates, requests, traces, request spans, pricing matches, fetch-models operations, and artifacts.
2. Create typed fetcher and mutation helpers that wrap `openapi-fetch` and throw on `error`.
3. Add a small helper for invalidating related query families from mutations.
4. Type-check these helpers against `dashboard/src/openapi-types.d.ts`.

## 3. Migrate shared data composables

1. Convert `useProvidersMap` to read providers through `useQuery`.
2. Replace the exchange-rate Pinia request logic with Vue Query queries and mutations.
3. Update `App.vue`, `RatesView.vue`, and `MoneyDisplay` callers to use the new exchange-rate composable directly.

## 4. Migrate CRUD list pages

1. Migrate `ProvidersView.vue` to `useQuery` for the providers list and `useMutation` for update/delete/toggle actions.
2. Migrate `EndpointsView.vue`.
3. Migrate `ModelsView.vue`, including its provider, provider endpoint, and endpoint reference queries.
4. Migrate `ScriptsView.vue`.
5. Migrate `ApiKeysView.vue`.
6. Preserve existing side-panel keys, selected-row behavior, confirmation dialogs, and Chinese UI copy.

## 5. Migrate forms and side panels

1. Migrate `ProviderForm.vue`, `EndpointForm.vue`, `ModelForm.vue`, `ScriptForm.vue`, `ApiKeyForm.vue`, and mapping/provider-endpoint forms to mutations.
2. Migrate `ProviderEndpointsPanel.vue` and `ProviderModelsPanel.vue` to query their reference data with prop-keyed queries.
3. Migrate `ModelPricingMatchPanel.vue` to mutations for match and save actions.
4. Ensure each successful form save invalidates the affected query families and still calls any `onSave` callback needed to close or refresh surrounding UI.

## 6. Migrate operational history and details

1. Migrate `RequestsView.vue` reference data to cached queries.
2. Migrate request history loading to `useInfiniteQuery` with a filter-keyed query and cursor-based page parameters.
3. Migrate `TracesView.vue` to a filter-keyed query.
4. Migrate `RequestDetailsContent.vue` request span loading to a request-ID-keyed query.
5. Migrate artifact viewers to `useQuery` keyed by artifact URL.

## 7. Remove old manual request state

1. Remove `onMounted(fetch*)` and watcher-triggered fetch calls that are replaced by reactive query keys.
2. Remove module-level loaded flags and duplicated `loading` refs that Vue Query now owns.
3. Keep non-request watchers that synchronize URL state, side-panel state, editor state, and user input.
4. Verify there are no remaining direct management API calls outside Vue Query mutation/query functions.

## 8. Verify

1. Run `pnpm --dir dashboard type-check`.
2. Run `pnpm --dir dashboard lint`.
3. Run `pnpm --dir dashboard build`.
4. Manually smoke-test the dashboard flows for Providers, Endpoints, Models, Scripts, API Keys, Exchange Rates, Requests, Traces, request details, provider side panels, and artifact views.
