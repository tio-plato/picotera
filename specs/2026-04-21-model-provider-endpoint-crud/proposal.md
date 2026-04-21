# Proposal: Model Provider Endpoint CRUD API + Generic Pagination

## Requirements

1. Add full CRUD (Create, Read, Update, Delete) API for the `model_provider_endpoint` table.
2. Implement a generic pagination mechanism that can be reused across all list endpoints.

## Background

The `model_provider_endpoint` table links models to provider endpoints with routing metadata:

```sql
CREATE TABLE model_provider_endpoint (
  model_name TEXT NOT NULL,
  provider_id INTEGER NOT NULL,
  endpoint_id INTEGER NOT NULL,
  upstream_model_name TEXT,
  priority INTEGER NOT NULL,
  annotations JSONB NOT NULL,
  PRIMARY KEY (model_name, provider_id, endpoint_id)
);
```

Currently this table only has a read query (`GetProvidersByEndpointAndModel`) used for gateway routing. No management API exists for it.

Existing list endpoints (`GetModels`, `GetEndpoints`, `GetProviders`) return all rows without pagination, which won't scale.
