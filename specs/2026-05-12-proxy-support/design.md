# Design: Proxy Support for Gateway Outbound Requests

## Background

PicoTera's gateway forwards LLM inference requests to upstream providers. Currently all outbound HTTP calls use a single `http.Client` with a default `http.Transport`. Go's `http.Transport.Proxy` defaults to `http.ProxyFromEnvironment`, which already reads `HTTP_PROXY`, `HTTPS_PROXY`, `ALL_PROXY`, and `NO_PROXY` environment variables. However, there is no way to override or disable the proxy per upstream provider endpoint.

## Goals

1. **Explicit environment proxy support** — make the proxy-from-environment behavior visible and intentional (set `Proxy: http.ProxyFromEnvironment` explicitly on the transport).
2. **Per-provider-endpoint proxy override** — add a `proxy_url` column to the `provider_endpoint` table so operators can specify a custom proxy for a specific upstream, or bypass the proxy entirely.
3. **Transparent to JS hooks and existing logic** — proxy configuration is a transport-layer concern; it does not leak into the JS hook interface or the request/response artifacts.

## Design

### Proxy URL semantics

The `proxy_url` field on `provider_endpoint` has three modes:

| Value      | Meaning                                                       |
|------------|---------------------------------------------------------------|
| `NULL`     | Use environment proxy (`http.ProxyFromEnvironment`). Default. |
| `"direct"` | Bypass all proxies; connect directly to the upstream.         |
| `<URL>`    | Use this URL as the proxy (e.g. `http://proxy:8080`).        |

Any other non-URL string is rejected at the API validation layer.

### Transport caching

A `proxyTransportCache` is introduced in the `server` package. It lazily creates and caches `*http.Transport` instances keyed by the proxy configuration string (`""` for environment, `"direct"` for no proxy, or the proxy URL). Each cached transport is cloned from a shared base transport (which carries `ResponseHeaderTimeout`, TLS settings, connection pool limits, etc.) with only the `Proxy` function overridden.

This avoids per-request transport allocation while keeping connection pools correctly separated by proxy target.

### Data flow

1. Routing SQL queries (`GetProvidersByEndpointAndModel`, `GetProvidersByEndpointTypesAndModel`) are updated to return `pe.proxy_url`.
2. The gateway handler reads `proxy_url` from the provider row's sidecar and passes it to `forwardRequest`.
3. `forwardRequest` looks up (or creates) the appropriate transport via the cache and executes the request.
4. The `handleFetchModels` management endpoint also uses the proxy URL from the `provider_endpoint` row.

### API contract

The `ProviderEndpointView` gains an optional `proxyUrl` string field. The OpenAPI spec is regenerated. The dashboard's `ProviderEndpointsPanel.vue` adds an input field for proxy URL in both the add and edit forms.

## Components changed

| Layer       | File / Area                                     | Change                                                    |
|-------------|------------------------------------------------|------------------------------------------------------------|
| Database    | New migration `022`                            | `ALTER TABLE provider_endpoint ADD COLUMN proxy_url TEXT`  |
| sqlc        | `db/queries/provider_endpoint.sql`             | Add `proxy_url` to UPSERT and SELECT queries              |
| sqlc        | `db/queries/routing.sql`                       | Add `pe.proxy_url` to both routing queries                |
| Generated   | `pkg/db/`                                      | Regenerate (sqlc generate)                                |
| Contract    | `pkg/contract/provider_endpoint.go`            | Add `ProxyUrl` to `ProviderEndpointView`                  |
| Server      | `pkg/server/server.go`                         | Add `proxyTransportCache` field; initialize it            |
| Server      | `pkg/server/proxy_transport.go` (new)          | Transport cache implementation                            |
| Server      | `pkg/server/gateway_helpers.go`                | `forwardRequest` accepts proxy URL string                 |
| Server      | `pkg/server/handle_gateway.go`                 | Pass proxy URL from sidecar to `forwardRequest`           |
| Server      | `pkg/server/handle_unified_gateway.go`         | Pass proxy URL from sidecar to `forwardRequest`           |
| Server      | `pkg/server/handle_provider_endpoint.go`       | Use proxy URL for `handleFetchModels`                     |
| Dashboard   | `ProviderEndpointsPanel.vue`                   | Add proxy URL input to add/edit forms                     |
| OpenAPI     | `openapi.yaml`                                 | Regenerate                                                |
