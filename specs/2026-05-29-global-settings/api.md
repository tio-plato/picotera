# API: Global Settings

## Endpoints

All endpoints are under the `/api/picotera` group.

### List Settings

```
GET /api/picotera/settings
```

**Response (200)**:
```json
[
  { "key": "app.title", "value": "My Gateway" }
]
```

### Get Setting

```
GET /api/picotera/settings/{key}
```

**Path Parameters**:
- `key` (string) — the setting key.

**Response (200)**:
```json
{ "key": "app.title", "value": "My Gateway" }
```

**Error (404)**: Setting not found.

### Upsert Setting

```
PUT /api/picotera/settings
```

**Request Body**:
```json
{
  "key": "app.title",
  "value": "My Gateway"
}
```

- `key`: required, non-empty string.
- `value`: required, any JSON value (object, array, string, number, boolean, null).

**Response (200)**:
```json
{ "key": "app.title", "value": "My Gateway" }
```

### Delete Setting

```
DELETE /api/picotera/settings/{key}
```

**Path Parameters**:
- `key` (string) — the setting key.

**Response (200)**: Empty body.

**Error (404)**: Setting not found.

## Types

### GlobalSettingView

```typescript
interface GlobalSettingView {
  key: string
  value: unknown  // any JSON value
}
```
