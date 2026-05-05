# API

## Database

Add nullable text column:

```sql
ALTER TABLE request ADD COLUMN user_message_preview TEXT;
```

The down migration drops the column.

## Request Insertion

`InsertRequest` accepts `user_message_preview` as an additional nullable argument and writes it with the new request row.

Meta request insertion in both gateway handlers passes the extracted preview. Upstream request insertion passes `NULL`.

## Management API

### `RequestView`

Add:

```json
{
  "userMessagePreview": "请帮我分析...下一步怎么做"
}
```

Field behavior:

- Omitted when `NULL`.
- Present on meta rows when a user message was extracted.
- Normally omitted on upstream rows.

### `RequestTraceView`

Add:

```json
{
  "userMessagePreview": "请帮我分析...下一步怎么做"
}
```

Field behavior:

- Omitted when no meta row in the trace has an extracted preview.
- Chosen from the newest meta request row in the trace with a non-null preview.

Affected endpoints:

- `GET /api/picotera/requests`
- `GET /api/picotera/request-traces`
- `GET /api/picotera/requests/{id}`
- `GET /api/picotera/requests/{id}/spans`

## OpenAPI and Dashboard Types

Regenerate `openapi.yaml`, `dashboard/src/openapi-types.d.ts`, and `dashboard/src/api/openapi.ts` through the repository workflow after implementation.
