# API — Request Artifacts

## RequestView 字段扩展

```ts
type RequestView = {
  // ... existing fields
  requestArtifactUrl?: string   // presigned GET URL，1h TTL，指向 zstd 压缩的 JSON
  responseArtifactUrl?: string  // 同上
}
```

两个字段在以下情况下省略：

- 未配置 S3 / artifact 功能未启用。
- presigned 生成失败（错误已 log，列表不报错）。

字段对所有返回 `RequestView` 的 endpoint 生效：

- `GET /api/picotera/requests`
- `GET /api/picotera/requests/{id}`
- `GET /api/picotera/requests/{id}/spans`

## Artifact JSON Schema

```json
{
  "method": "POST",
  "url": "https://api.openai.com/v1/chat/completions",
  "statusCode": 200,
  "headers": {
    "Content-Type": ["application/json"],
    "Authorization": ["Bearer sk-..."]
  },
  "body": "...",
  "bodyEncoding": "utf8"
}
```

字段说明：

| 字段           | 出现在            | 说明                                                |
| -------------- | ----------------- | --------------------------------------------------- |
| `method`       | request 文档      | HTTP method                                         |
| `url`          | request 文档      | 完整 URL（meta = client 请求的原始 path-only URL；upstream = upstream URL） |
| `statusCode`   | response 文档     | HTTP status code                                    |
| `headers`      | 都有              | `map<string, string[]>`，保留多值                  |
| `body`         | 都有              | `bodyEncoding === "utf8"` 时是字符串，否则 base64   |
| `bodyEncoding` | 都有              | `"utf8"` 或 `"base64"`                              |

## Object key

```
artifacts/{YYYY-MM-DD}/{requestId}.request.json.zst
artifacts/{YYYY-MM-DD}/{requestId}.response.json.zst
```

- `{YYYY-MM-DD}` 取 `request.created_at` 的 UTC 日期，零填充。
- `{requestId}` 是 xid（20 字节、URL 安全）。
- 上传时 metadata：`Content-Type: application/json`，`Content-Encoding: zstd`。

## 配置

新增 env（无前缀已有 `PICOTERA_`）：

| Env                       | 默认值      | 必填 | 说明                                    |
| ------------------------- | ----------- | ---- | --------------------------------------- |
| `PICOTERA_S3_ENDPOINT`    | （空）      | 否   | 留空则关闭 artifact 功能                |
| `PICOTERA_S3_REGION`      | `us-east-1` | 否   |                                         |
| `PICOTERA_S3_ACCESS_KEY`  | （空）      | 是   | 启用 artifact 时必填                    |
| `PICOTERA_S3_SECRET_KEY`  | （空）      | 是   | 启用 artifact 时必填                    |
| `PICOTERA_S3_BUCKET`      | （空）      | 是   | 启用 artifact 时必填                    |
| `PICOTERA_S3_USE_SSL`     | `false`     | 否   |                                         |
| `PICOTERA_S3_PUBLIC_URL`  | （空）      | 否   | presigned URL 用此 host 重写，方便前端访问 |
