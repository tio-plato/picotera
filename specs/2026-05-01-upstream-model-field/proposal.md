# Proposal: Add `upstream_model` Field to Request Log

The current `model` field in the `request` table stores the model name from the client's original request. This is ambiguous — it doesn't distinguish between what the client asked for and what was actually sent to the upstream provider.

**Current behavior**: `model` stores the client-requested model name (e.g., `"gpt-4o"`). The upstream model name (e.g., `"gpt-4o-2024-08-06"`) from `model_provider_endpoint.upstream_model_name` is used to rewrite the outgoing request body but is never persisted.

**Requested change**:

- Clarify `model` as the client's original requested model name (no behavior change, just semantic clarity).
- Add a new `upstream_model` column to the `request` table to store the actual model name sent to the upstream provider.
