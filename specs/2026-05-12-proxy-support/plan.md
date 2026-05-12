# Execution Plan: Proxy Support

## Step 1 — Database migration

Create `db/migrations/022_provider_endpoint_proxy_url.sql`:

```sql
-- +goose Up
ALTER TABLE provider_endpoint ADD COLUMN proxy_url TEXT;

-- +goose Down
ALTER TABLE provider_endpoint DROP COLUMN proxy_url;
```

The column is nullable with no default, so existing rows get `NULL` (meaning "use environment proxy").

## Step 2 — Update sqlc queries

### `db/queries/provider_endpoint.sql`

- `UpsertProviderEndpoint`: add `proxy_url` as the 5th parameter (`$5`). Update the INSERT column list, VALUES, and the ON CONFLICT DO UPDATE SET clause.
- `GetProviderEndpoint`, `ListProviderEndpoints`: unchanged (they already use `SELECT *`, which will pick up the new column automatically).

### `db/queries/routing.sql`

- `GetProvidersByEndpointAndModel`: add `pe.proxy_url` to the SELECT list.
- `GetProvidersByEndpointTypesAndModel`: add `pe.proxy_url` to the SELECT list.

## Step 3 — Regenerate sqlc output

Run `sqlc generate`. This updates:
- `pkg/db/models.go` — `ProviderEndpoint` struct gains `ProxyUrl pgtype.Text`
- `pkg/db/provider_endpoint.sql.go` — `UpsertProviderEndpointParams` gains `ProxyUrl pgtype.Text`
- `pkg/db/routing.sql.go` — both routing row types gain `ProxyUrl pgtype.Text`

## Step 4 — Create proxy transport cache

Create `pkg/server/proxy_transport.go`:

```go
package server

import (
    "net/http"
    "net/url"
    "sync"
)

type proxyTransportCache struct {
    base *http.Transport
    mu   sync.RWMutex
    cache map[string]*http.Transport
}

func newProxyTransportCache(base *http.Transport) *proxyTransportCache {
    return &proxyTransportCache{
        base:  base,
        cache: make(map[string]*http.Transport),
    }
}

// get returns an http.Transport configured for the given proxy URL.
//   - "" (empty) → ProxyFromEnvironment (default behavior)
//   - "direct"   → no proxy
//   - URL string → use that proxy
func (c *proxyTransportCache) get(proxyURL string) *http.Transport {
    // "" → use base transport as-is (ProxyFromEnvironment)
    if proxyURL == "" {
        return c.base
    }

    c.mu.RLock()
    t, ok := c.cache[proxyURL]
    c.mu.RUnlock()
    if ok {
        return t
    }

    c.mu.Lock()
    defer c.mu.Unlock()
    // Double-check after acquiring write lock.
    if t, ok = c.cache[proxyURL]; ok {
        return t
    }

    cloned := c.base.Clone()
    if proxyURL == "direct" {
        cloned.Proxy = nil // no proxy
    } else {
        parsed, err := url.Parse(proxyURL)
        if err != nil {
            // Invalid URL — fall back to environment proxy.
            // This shouldn't happen because API validation catches it,
            // but be safe.
            return c.base
        }
        cloned.Proxy = http.ProxyURL(parsed)
    }
    c.cache[proxyURL] = cloned
    return cloned
}
```

## Step 5 — Wire proxy transport cache into Server

In `pkg/server/server.go`:

1. Add `proxyCache *proxyTransportCache` field to the `Server` struct.
2. After creating the base `http.Transport` and `http.Client`, create the cache:
   ```go
   proxyCache := newProxyTransportCache(baseTransport)
   ```
3. Store `proxyCache` on the server instance.
4. Keep `httpClient` as-is for backward compatibility (it's used in `handleFetchModels` and `forwardRequest`).

Update the `http.Client` creation to use a named base transport:
```go
baseTransport := &http.Transport{
    ResponseHeaderTimeout: config.GatewayReadTimeout,
}
httpClient := &http.Client{Transport: baseTransport}
proxyCache := newProxyTransportCache(baseTransport)
```

## Step 6 — Update forwardRequest

In `pkg/server/gateway_helpers.go`, change `forwardRequest` to accept a proxy URL:

```go
func (s *Server) forwardRequest(req *http.Request, proxyURL string) (*http.Response, error) {
    transport := s.proxyCache.get(proxyURL)
    return transport.RoundTrip(req)
}
```

Using `transport.RoundTrip(req)` directly instead of `httpClient.Do(req)` because we're swapping the transport per request. The `http.Client` is only a thin wrapper around `Do` → `transport.RoundTrip`, so this is equivalent.

## Step 7 — Thread proxy URL through gateway handler

### `pkg/server/handle_gateway.go`

In the `providerSidecar` struct, add a `proxyURL string` field.

In the candidate-building loop, populate it from the routing row:
```go
sidecar[row.ProviderID] = providerSidecar{
    upstreamURL:  row.UpstreamUrl,
    credentials:  row.ProviderCredentials,
    sendResolver: ...,
    annotations:  merged,
    proxyURL:     row.ProxyUrl.String, // pgtype.Text
}
```

In the retry loop, pass `side.proxyURL` to `forwardRequest`:
```go
resp, err := h.forwardRequest(req, side.proxyURL)
```

### `pkg/server/handle_unified_gateway.go`

Same changes:
1. Add `proxyURL string` to the unified `providerSidecar` struct.
2. Populate from the routing row's `ProxyUrl`.
3. Pass to `forwardRequest` in the retry loop.

## Step 8 — Thread proxy URL through fetch models

In `pkg/server/handle_provider_endpoint.go`, `handleFetchModels`:

After fetching the `provider_endpoint` row, use its proxy URL when making the upstream request:

```go
transport := s.proxyCache.get(pe.ProxyUrl.String)
resp, err := transport.RoundTrip(req)
```

Replace `s.httpClient.Do(req)` with the transport-based call.

## Step 9 — Update contract

In `pkg/contract/provider_endpoint.go`:

Add `ProxyUrl` to `ProviderEndpointView`:
```go
type ProviderEndpointView struct {
    ProviderID          int32  `json:"providerId"`
    EndpointPath        string `json:"endpointPath"`
    UpstreamUrl         string `json:"upstreamUrl"`
    CredentialsResolver string `json:"credentialsResolver,omitempty" enum:"unknown,generalApiKey,bearerToken,xApiKey,searchKey,googApiKey"`
    ProxyUrl            string `json:"proxyUrl,omitempty"`
}
```

Update `ToProviderEndpointView`:
```go
func ToProviderEndpointView(pe *db.ProviderEndpoint) *ProviderEndpointView {
    v := &ProviderEndpointView{
        ProviderID:          pe.ProviderID,
        EndpointPath:        pe.EndpointPath,
        UpstreamUrl:         pe.UpstreamUrl,
        CredentialsResolver: FromCredentialsResolver(pe.CredentialsResolver),
    }
    if pe.ProxyUrl.Valid {
        v.ProxyUrl = pe.ProxyUrl.String
    }
    return v
}
```

Update `FromProviderEndpointView`:
```go
func FromProviderEndpointView(view *ProviderEndpointView) *db.UpsertProviderEndpointParams {
    p := &db.UpsertProviderEndpointParams{
        ProviderID:          view.ProviderID,
        EndpointPath:        view.EndpointPath,
        UpstreamUrl:         view.UpstreamUrl,
        CredentialsResolver: ToCredentialsResolver(view.CredentialsResolver),
    }
    if view.ProxyUrl != "" {
        p.ProxyUrl = pgtype.Text{String: view.ProxyUrl, Valid: true}
    }
    return p
}
```

## Step 10 — Validate proxy URL in contract

Add validation in `FromProviderEndpointView` or the handler: if `proxyUrl` is non-empty and not `"direct"`, parse it as a URL. Reject invalid values with a 400 error.

## Step 11 — Regenerate OpenAPI spec and TypeScript types

```bash
mise run openapi
pnpm --dir dashboard generate-openapi
```

## Step 12 — Update dashboard ProviderEndpointsPanel.vue

1. Add `proxyUrl` to the form state and edit draft state.
2. Add a new `<Field label="代理 URL">` input in both the add form and the edit template.
3. Include placeholder text like `留空使用环境代理，填 direct 禁用代理`.
4. Wire `proxyUrl` through the upsert mutation calls.
5. Display the proxy URL in the read-only binding view (when not editing).
