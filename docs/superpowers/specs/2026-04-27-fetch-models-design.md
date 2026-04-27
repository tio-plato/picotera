# Fetch Models from Upstream

## Summary

Add a feature to fetch model lists from upstream providers' `/models` API endpoints. When a provider has an endpoint binding whose path ends with `/models`, the user can trigger a fetch that GETs the upstream URL, parses the response, and updates `provider.provider_models`.

## Credentials Resolver Refactor

### Background

The `credentials_resolver` field on the `endpoint` table currently only supports `generalApiKey` (value 1). The gateway detects the upstream auth style from the client request headers and applies `provider.credentials` accordingly. This is indirect — the auth style should be a property of the endpoint configuration, not inferred from the incoming request.

### New Credentials Resolver Values

Add two new resolver types to the existing enum:

| Value | Name | Upstream Headers |
|-------|------|-----------------|
| 1 | `generalApiKey` | Both `Authorization: Bearer <creds>` and `X-Api-Key: <creds>` |
| 2 | `bearerToken` | Only `Authorization: Bearer <creds>` |
| 3 | `xApiKey` | Only `X-Api-Key: <creds>` |

`generalApiKey` retains its existing value (1) for backward compatibility.

### Shared Auth Helper

Extract a shared function `setCredentialsHeaders(headers http.Header, credentials string, resolver int)` that applies the correct headers based on the resolver type. Used by both the gateway and the fetch-models handler.

### Gateway Refactor

Replace the upstream auth logic in `buildUpstreamRequest` (`gateway_helpers.go`):
- Keep `resolveAuthType()` for validating that the client request is authenticated (return 401 if not), but no longer use its result for upstream header style
- Read `credentials_resolver` from the endpoint (already available in the routing query results) instead
- Call `setCredentialsHeaders(req.Header, creds, resolver)` to set upstream auth headers

This makes upstream auth behavior explicit and configurable per endpoint, rather than inferred from the client request. Client auth validation (401 for missing credentials) remains unchanged.

### Frontend: Endpoint Form

Update the endpoint form's credentials resolver field to offer three options:
- 通用 API Key (generalApiKey) — default
- Bearer Token (bearerToken)
- X-Api-Key (xApiKey)

## Backend: Fetch Models API

### New API Operation

`POST /api/picotera/provider-endpoints/fetch-models`

**Request body:**
```json
{ "providerId": 1, "endpointPath": "/models" }
```

**Success response (200):**
```json
{
  "providerId": 1,
  "models": ["gpt-4o", "gpt-4o-mini", "o1-preview"]
}
```

**Side effect:** Parsed model list is written to `provider.provider_models`.

**Error responses (standard Huma error format):**
- 404: provider or binding not found
- 502: upstream request failed (network error, 10s timeout, non-2xx)
- 422: upstream response parse failure (format not recognized)

### Auth

Read `provider.credentials` and `endpoint.credentials_resolver`. Apply headers via the shared `setCredentialsHeaders` function. If credentials is empty, omit all auth headers.

### Response Parsing Priority

1. `response.data[].id` (OpenAI style)
2. `response.data[].name`
3. Top-level `response[].id`
4. Top-level `response[].name`

At each step, filter out non-string values, deduplicate, and sort alphabetically.

### Timeout

10 seconds.

## Frontend: Fetch Models UI

### Trigger Condition

The "fetch models" button only appears on binding rows where `endpointPath` ends with `/models`.

### Button Placement

Inline to the right of the endpoint path text, before the upstream URL input row. Uses Tabler icons (not Unicode).

Layout per binding row:
```
/models  [cloud-download icon "拉取"]
[upstream URL input]              [trash icon]
```

### Interaction States

1. **Idle**: Button shows `cloud-download` Tabler icon, label "拉取"
2. **Loading**: Button shows `loader` Tabler icon (spinning), disabled, label "拉取中…"
3. **Success**: Button briefly shows `check` Tabler icon + "N 个模型" in green (~2s), then reverts to idle. `provider.provider_models` is updated.
4. **Failure**: Button reverts to idle. Error message shown in panel's `#error` slot (e.g. "拉取失败：上游返回 401 Unauthorized").

### Implementation Notes

- Add `fetchModels(providerId, endpointPath)` function in `ProviderEndpointsPanel.vue`
- Call `POST /api/picotera/provider-endpoints/fetch-models`
- Track per-binding loading/success state with a reactive map keyed by `endpointPath`
- After successful fetch, emit a `models-fetched` event with `{ providerId }` so the provider detail view can refresh `provider.provider_models` display
