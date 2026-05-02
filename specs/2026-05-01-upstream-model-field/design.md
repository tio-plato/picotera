# Design: Add `upstream_model` to Request Log

## Problem

The `request.model` column stores `modelName` — but after the `rewriteModel` hook was added (commit 3e6fc98), this is the **rewritten** model name, not necessarily the client's original. The actual model sent to the upstream provider is computed by a 3-tier fallback (`dec.UpstreamModel` → `mpe.upstreamModelName` → `modelName`) and is never persisted. This makes it impossible to audit which upstream model was actually called, or to see what the client originally requested.

## Design

Add a nullable `upstream_model TEXT` column to the `request` table. The semantics of the two model fields:

| Column | Meaning | Example |
|---|---|---|
| `model` | Client's original requested model name (before `rewriteModel` hook) | `gpt-4o` |
| `upstream_model` | Model name actually sent to the upstream provider (after all hooks + MPE resolution) | `gpt-4o-2024-08-06` |

### When each field is populated

- **Meta request row** (type=0): `model` = original client model (captured before `rewriteModel`), `upstream_model` = the upstream model from the winning attempt (set on `UpdateRequestOnHeader`).
- **Upstream request row** (type=1): `model` = original client model, `upstream_model` = the model actually sent upstream for this attempt (set at `InsertRequest` time since it's already known).

### Where `upstream_model` comes from

The value is the result of the existing 3-tier fallback chain in the gateway handler: `dec.UpstreamModel` → `candidateUpstreamModel(cand)` → `modelName`. This is the same value passed to `buildUpstreamRequest`. When the fallback chain resolves to `modelName` (i.e., no MPE override and no hook override), `upstream_model` equals `model` — in that case we store it anyway for consistency and queryability.

### Capturing the original client model

The `rewriteModel` hook mutates `modelName` in-place. We must capture `originalModelName` **before** the hook runs and use that value for the `model` column in all request rows. Currently the code does `modelName = newModel` after the hook; we preserve the original in a separate variable.

### API changes

- `RequestView` gains `UpstreamModel string` field (JSON: `upstreamModel`).
- `ListRequestsRequest` gains an optional `UpstreamModel` filter query param.
- `UpdateRequestOnHeader` SQL adds `upstream_model = $7`.

### No new dependencies

Pure schema + generated code + handler changes. No new libraries.
