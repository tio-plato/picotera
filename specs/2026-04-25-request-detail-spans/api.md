# API — Request Detail Spans

## 调整：`GET /api/picotera/requests`

新增 `type` 查询参数语义：

- `type` 已存在（`int32`，默认 `-1`）。前端默认传 `0`（meta）；切换可传 `1`（upstream）或省略以查询全部。
- 后端逻辑无变化（`Type < 0` 视为不过滤）。

## 新增：`GET /api/picotera/requests/{id}/spans`

列出与给定 meta 请求关联的全部 span（meta 自身 + 所有 upstream）。

**Operation ID**: `listRequestSpans`

**Path 参数**

| 名称 | 类型 | 说明 |
| --- | --- | --- |
| `id` | string | meta 请求 id（同 `span_id`） |

**响应**

```json
[
  { "id": "...", "type": 0, "status": 2, "spanId": "...", ... },
  { "id": "...", "type": 1, "status": 2, "spanId": "<meta-id>", ... }
]
```

- 类型：`RequestView[]`
- 排序：`created_at` 升序；meta 行（`id == span_id`）保证位列首位。
- 找不到任何匹配行时返回 `404 RequestNotFound`。

## sqlc 查询

```sql
-- name: ListRequestsBySpan :many
SELECT id, span_id, parent_span_id, type, status, provider_id, endpoint_path, api_key_id, model,
       input_tokens, cache_read_tokens, output_tokens, cache_write_tokens,
       status_code, error_message, ttft_ms, time_spent_ms, created_at
FROM request
WHERE span_id = $1
ORDER BY created_at ASC, id ASC;
```
