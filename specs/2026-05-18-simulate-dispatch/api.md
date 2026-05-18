# API — Simulate dispatch

Single new Huma operation registered under the `/api/picotera` group.

## Operation

```
POST /api/picotera/simulate/dispatch
```

Defined as `OperationSimulateDispatch` in `pkg/contract/simulate.go`. Wired in `pkg/server/server.go`:

```go
huma.Register(mgmt, contract.OperationSimulateDispatch, s.handleSimulateDispatch)
```

## Request body

```jsonc
{
  "endpoint": {
    "kind": "path" | "unified",
    "path": "/v1/chat/completions",          // when kind == "path"
    "format": "anthropicMessages"            // when kind == "unified"
                                             // one of: anthropicMessages,
                                             // openaiChatCompletions, openaiResponses,
                                             // geminiGenerateContent, geminiStreamGenerateContent
  },
  "apiKeyId": 7,
  "model": "claude-sonnet-4-6",
  "body": "{\"messages\":[…],\"stream\":true}",   // raw JSON string, opaque to the server
  "pathVars": { "model": "claude-sonnet-4-6" }    // optional, only used by path endpoints
                                                  // whose modelPath / upstream URL has {…} tokens
}
```

Validation:
- `endpoint.kind` must be `"path"` or `"unified"` — anything else returns 400.
- `endpoint.path` is required when `kind == "path"`; `endpoint.format` is required when `kind == "unified"`. The other field must be absent.
- `apiKeyId` must be a positive int; the row must exist and not be disabled.
- `model` is required and non-empty.
- `body` is required. If non-empty it must parse as JSON — otherwise 400. Empty body is allowed (hooks see `body` as omitted).
- `pathVars` keys are validated against the endpoint's `{…}` tokens; unknown keys are accepted silently (matches production behavior where extra path vars are inert).

## Response body

```jsonc
{
  "originalModel": "claude-sonnet-4-6",
  "resolvedModel": "claude-sonnet-4-6",
  "sourceFormat": "anthropicMessages",
  "stream": true,
  "candidates": [
    {
      "provider": {
        "id": 3,
        "name": "Anthropic",
        "priority": 0,
        "annotations": { "region": "us-east-1" },
        "disabled": false
      },
      "mpe": {
        "modelName": "claude-sonnet-4-6",
        "providerId": 3,
        "endpointPath": "/v1/messages",
        "upstreamModelName": "claude-sonnet-4-6",
        "priority": 10,
        "annotations": { "tier": "premium" }
      },
      "mergedAnnotations": {
        "region": "us-east-1",
        "tier": "premium",
        "channel": "default"
      },
      "upstreamFormat": "anthropicMessages",
      "bridged": false
    }
  ],
  "logs": [
    { "level": "info", "message": "sortProviders fired", "ts": "2026-05-18T07:14:02Z" }
  ]
}
```

Notes:
- `candidates` is ordered exactly as `sortProviders` returned. If JS introduces a candidate whose `(providerId, endpointPath)` is not in the resolved set, that candidate is **dropped** from the response (matches production behavior — those are skipped in the dispatch loop, never sent).
- `mergedAnnotations` is the post-merge map (`model < provider < entry < apiKey`, last wins) the JS hook saw.
- `upstreamFormat` always equals `sourceFormat` for path endpoints; for unified routes it can differ per candidate.
- `bridged` is sugar (`sourceFormat != upstreamFormat`) so the dashboard doesn't have to compute it.
- `logs.ts` is RFC 3339 UTC, same encoding `artifacts.LogEntry` already uses.

## Error responses

All errors return the standard `PicoTeraError` body (`{message, code, details}`).

| Condition | Status | Code |
|---|---|---|
| Endpoint path / format unknown | 404 | `RouteNotFound` |
| `apiKeyId` not found | 404 | `NotFound` |
| `apiKey.disabled == true` | 403 | `Forbidden` |
| `model` empty, or `{name}` path var missing | 400 | `ModelNotFound` |
| `body` not valid JSON | 400 | `InvalidRequest` |
| No MPE matches model (after rewriteModel) | 404 | `NoProviderAvailable` |
| `rewriteModel` or `sortProviders` hook errors out | 502 | `UpstreamError` |
| Hook timed out (`jsx.ErrHookTimeout`) | 503 | `UpstreamError` |
| Any other failure | 500 | `InternalError` |
