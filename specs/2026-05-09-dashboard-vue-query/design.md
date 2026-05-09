# Design

## Goal

Dashboard data fetching will move from component-local `ref` + `onMounted` loaders and Pinia-managed request state to `@tanstack/vue-query`. The dashboard will keep `openapi-fetch` as the typed HTTP client, and Vue Query will own request lifecycle state, cache identity, refetching, invalidation, and mutation side effects.

## Dependency

Add `@tanstack/vue-query` to `dashboard/package.json`.

The app will install `VueQueryPlugin` in `dashboard/src/main.ts` with a shared `QueryClient`. Default options will favor stable management-console behavior:

- `refetchOnWindowFocus: false`
- `retry: 1`
- a nonzero `staleTime` for management resources such as providers, endpoints, models, mappings, scripts, API keys, and exchange rates
- a shorter `staleTime` for operational history resources such as requests, traces, and request details

## API Client Integration

`openapi-fetch` remains the only HTTP transport for management API calls. Vue Query query functions and mutation functions will call `api.GET`, `api.POST`, and `api.PUT` through typed helper functions instead of issuing raw management fetches.

Introduce a small dashboard query layer under `dashboard/src/api/`:

- `queryClient.ts` creates and exports the shared `QueryClient`.
- `queryKeys.ts` defines stable query-key factories for every API resource.
- resource-specific query modules define typed fetcher functions and mutation helpers around `openapi-fetch`.

This keeps endpoint strings centralized and gives components stable invalidation targets.

## Query Keys

Query keys will be resource-first arrays with parameter objects as the final segment where needed:

- `['providers']`
- `['providers', id]`
- `['endpoints']`
- `['models']`
- `['providerEndpoints', filters]`
- `['apiKeys']`
- `['scripts']`
- `['exchangeRates']`
- `['requests', filters]`
- `['requestTraces', filters]`
- `['requestSpans', requestId]`
- `['artifacts', url]`

Filter objects must be constructed from validated UI state and kept deterministic. Components will not concatenate ad hoc string keys.

## Read Requests

Every dashboard read request will use `useQuery`.

Single-list CRUD pages such as Providers, Endpoints, Models, Scripts, API Keys, and Exchange Rates will bind their table data and loading states directly to query results. Manual `onMounted(fetch*)` functions will be removed.

Composite pages will use multiple queries:

- Models view reads models, providers, provider endpoints, and endpoints through independent cached queries.
- Requests view reads reference data through cached queries and request history through a filter-keyed query.
- Traces and request detail views use parameterized queries keyed by the selected filters or request ID.
- Side panels that load provider endpoints, provider models, request spans, pricing matches, and artifact payloads use local `useQuery` calls keyed by their props.

Artifact viewers currently using raw `fetch(url)` will be migrated to `useQuery` with `enabled` tied to the URL prop. Raw `fetch` remains acceptable only as the implementation inside the query function for non-OpenAPI artifact URLs.

## Mutations and Invalidation

Every create, update, delete, toggle, fetch-models, pricing-match, and exchange-rate write will use `useMutation`.

On successful mutation, the mutation will invalidate the exact affected query families:

- Provider writes invalidate `providers`; model/provider-model edits also invalidate `models`, `providerEndpoints`, and views derived from provider model data.
- Endpoint writes invalidate `endpoints` and `providerEndpoints`.
- Provider endpoint writes invalidate `providerEndpoints`, `providers`, and `models` where upstream derivations are displayed.
- Model writes invalidate `models`, `providers`, and request filter reference data that uses model names.
- Script writes invalidate `scripts`.
- API key writes invalidate `apiKeys`.
- Exchange-rate writes invalidate `exchangeRates` and any views displaying converted money values.
- Request history filters invalidate or refetch only `requests` keys matching the current filters.
- Request detail span updates invalidate `requestSpans` for the request ID.

Mutation functions will throw on API errors so Vue Query receives failures through its normal error channel. Components will preserve existing quiet error behavior except on pages that already display an error state.

## Shared Composables

Existing request-oriented composables will become Vue Query wrappers:

- `useProvidersMap` will read from the `providers` query cache and expose the same derived map and label helpers without its module-level `loaded` flag.
- The exchange rates Pinia store will stop owning fetch/load/mutation state. Exchange-rate request state will move to a Vue Query composable, and callers will use that composable directly.

Pinia remains in use for UI preferences and other local UI state.

## UX Behavior

The visible dashboard behavior stays the same: tables, side panels, confirmations, filters, and actions remain in place. Loading indicators will be driven by `isLoading`, `isFetching`, and mutation `isPending`. Existing Chinese UI copy and local UI primitives remain unchanged.

Cache-backed navigation will make repeated visits to dashboard pages show cached data immediately while Vue Query refreshes stale resources in the background according to the resource defaults.

## API Design

No backend API changes are required.
