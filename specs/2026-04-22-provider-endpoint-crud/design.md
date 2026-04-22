# Design: ProviderEndpoint CRUD API

## Overview

Add CRUD API for the `provider_endpoint` join table, which associates providers with endpoints and stores the `upstream_url` for each association.

## Database

The `provider_endpoint` table already exists in the schema with composite PK `(provider_id, endpoint_id)` and one data column `upstream_url`. No schema changes needed.

## Pattern Selection

Follow the **endpoint pattern** (non-paginated list) rather than the model_provider_endpoint pattern (paginated). The user explicitly said no pagination is needed.

- **List**: simple array response filtered by `provider_id` query param, no cursor/pagination
- **Upsert**: `INSERT ... ON CONFLICT DO UPDATE` (same as model_provider_endpoint)
- **Delete**: by composite key `(provider_id, endpoint_id)` (same as model_provider_endpoint)

## Scope

Three operations only:
1. List by provider (no pagination, no cursor)
2. Upsert
3. Delete

No single-record GET — the list already returns full records and the composite key makes a dedicated GET endpoint unnecessary given the simple schema.
