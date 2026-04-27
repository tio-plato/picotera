# Fetch Models from Upstream

## Summary

Add a feature to fetch model lists from upstream providers' `/models` API endpoints. When a provider has an endpoint binding whose path ends with `/models`, the user can trigger a fetch that GETs the upstream URL, parses the response, and updates `provider.provider_models`.

## Backend

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

Read `provider.credentials` and send as `Authorization: Bearer <credentials>`. If credentials is empty, omit the header.

### Response Parsing Priority

1. `response.data[].id` (OpenAI style)
2. `response.data[].name`
3. Top-level `response[].id`
4. Top-level `response[].name`

At each step, filter out non-string values, deduplicate, and sort alphabetically.

### Timeout

10 seconds.

## Frontend

### Trigger Condition

The "fetch models" button only appears on binding rows where `endpointPath` ends with `/models`.

### Button Placement

Inline to the right of the endpoint path text, before the upstream URL input row. Uses Tabler icons (not Unicode).

Layout per binding row:
```
/models  [↓ cloud-download icon]
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
