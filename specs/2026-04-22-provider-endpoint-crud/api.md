# API: ProviderEndpoint CRUD

Base path: `/api/picotera`

## List Provider Endpoints

```
GET /provider-endpoints?providerId={id}
```

- Query param `providerId` (required, int32) — filter by provider
- Returns array of `ProviderEndpointView`

Response `200`:
```json
[
  {
    "providerId": 1,
    "endpointId": 2,
    "upstreamUrl": "https://api.example.com/v1"
  }
]
```

## Upsert Provider Endpoint

```
PUT /provider-endpoints
```

Request body:
```json
{
  "providerId": 1,
  "endpointId": 2,
  "upstreamUrl": "https://api.example.com/v1"
}
```

Response `200`: single `ProviderEndpointView`

## Delete Provider Endpoint

```
POST /provider-endpoints/delete
```

Request body:
```json
{
  "providerId": 1,
  "endpointId": 2
}
```

Response `204`: empty
