# API: Model Provider Endpoint CRUD + Pagination

Base path: `/api/picotera`

## Pagination Parameters (applies to all list endpoints)

| Parameter | Type | In | Default | Max | Description |
|-----------|------|----|---------|-----|-------------|
| `limit` | int32 | query | 20 | 100 | Items per page |
| `cursor` | string | query | "" | — | Opaque cursor from previous page |

### Paginated Response Envelope

```json
{
  "items": [...],
  "pagination": {
    "nextCursor": "eyJtb2RlbE5hbWUiOiJncHQtNG8ifQ==",
    "hasMore": true
  }
}
```

When `hasMore` is false, `nextCursor` is omitted.

---

## Model Provider Endpoint APIs

### List Model Provider Endpoints

```
GET /api/picotera/model-provider-endpoints?limit=20&cursor=...&modelName=...&providerId=...&endpointId=...
```

**Query Parameters:**

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `limit` | int32 | no | Page size (default 20, max 100) |
| `cursor` | string | no | Pagination cursor |
| `modelName` | string | no | Filter by model name |
| `providerId` | int32 | no | Filter by provider ID |
| `endpointId` | int32 | no | Filter by endpoint ID |

**Response 200:**

```json
{
  "items": [
    {
      "modelName": "gpt-4o",
      "providerId": 1,
      "endpointId": 2,
      "upstreamModelName": "gpt-4o-2024-08-06",
      "priority": 10,
      "annotations": {}
    }
  ],
  "pagination": {
    "nextCursor": "...",
    "hasMore": true
  }
}
```

### Get Model Provider Endpoint

```
GET /api/picotera/model-provider-endpoints/{modelName}/{providerId}/{endpointId}
```

**Path Parameters:**

| Parameter | Type | Description |
|-----------|------|-------------|
| `modelName` | string | Model name |
| `providerId` | int32 | Provider ID |
| `endpointId` | int32 | Endpoint ID |

**Response 200:**

```json
{
  "modelName": "gpt-4o",
  "providerId": 1,
  "endpointId": 2,
  "upstreamModelName": "gpt-4o-2024-08-06",
  "priority": 10,
  "annotations": {}
}
```

**Response 404:**

```json
{
  "message": "model provider endpoint not found",
  "code": "MODEL_PROVIDER_ENDPOINT_NOT_FOUND"
}
```

### Upsert Model Provider Endpoint

```
PUT /api/picotera/model-provider-endpoints
```

**Request Body:**

```json
{
  "modelName": "gpt-4o",
  "providerId": 1,
  "endpointId": 2,
  "upstreamModelName": "gpt-4o-2024-08-06",
  "priority": 10,
  "annotations": {}
}
```

**Response 200:**

```json
{
  "modelName": "gpt-4o",
  "providerId": 1,
  "endpointId": 2,
  "upstreamModelName": "gpt-4o-2024-08-06",
  "priority": 10,
  "annotations": {}
}
```

### Delete Model Provider Endpoint

```
POST /api/picotera/model-provider-endpoints/delete
```

**Request Body:**

```json
{
  "modelName": "gpt-4o",
  "providerId": 1,
  "endpointId": 2
}
```

**Response 204:** (empty)
