# API — Endpoint Path Matching with Variables

## Management REST API

No new operations. No fields added or removed. The shape of `EndpointView` is unchanged.

The only change is **semantic**: `EndpointView.path` may now contain `{name}` placeholders, e.g.

```
/v1beta/models/{model}:generateContent
```

Each `{name}` matches any non-empty string including `/`. Variable names must match `[A-Za-z_][A-Za-z0-9_]*` and must be unique within a single path.

A `provider_endpoint.upstream_url` bound to such an endpoint may reference the same variables:

```
https://generativelanguage.googleapis.com/v1beta/models/{model}:generateContent
```

Unresolved `{name}` tokens in the upstream URL after substitution cause the attempt to fail (retried like any other upstream error).

## `endpoint.model_path` semantics

- If `model_path == "{name}"` (exactly one `{}` token, nothing else), the model is taken from the path variable `name` in the matched endpoint. If the variable is missing or empty, the request fails with `MODEL_NOT_FOUND`.
- Otherwise `model_path` is a gjson expression evaluated against the request body (unchanged).

## JS hook context

`RequestShape` (the `request` / `clientRequest` object passed to `sortProviders`, `rewriteModel`, `beforeRequest`, `rewriteRequest`) gains:

```ts
{
  path: string,
  method: string,
  headers: Record<string, string[]>,
  model: string,
  pathVars?: Record<string, string>, // NEW — omitted when the endpoint has no variables
  body?: any
}
```

Scripts can read `ctx.request.pathVars.model` etc. to branch on URL-derived data.

## SDK regeneration

`openapi.yaml` and the dashboard's typed client do not need regeneration — no Huma operation or contract type changed.
