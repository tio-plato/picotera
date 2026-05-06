## API — 请求 1h 缓存写入 tokens

所有路径在 `/api/picotera` 下。现有请求查询接口保持路径和方法不变，仅扩展响应字段。

### `RequestView`

新增字段：

```jsonc
{
  "cacheWrite1hTokens": 668
}
```

字段语义：

- 类型：integer。
- 可选：是。
- 来源：`request.cache_write_1h_tokens`。
- 仅当响应 usage 中识别到 1h 缓存写入 tokens 时返回。

相关接口：

- `GET /requests`
- `GET /requests/{id}`
- `GET /requests/{id}/spans`

### `RequestTraceView`

新增字段：

```jsonc
{
  "cacheWrite1hTokens": 668
}
```

字段语义：

- 类型：integer。
- 必填：是。
- 来源：同一 trace 下 upstream 请求的 `cache_write_1h_tokens` 聚合和。
- 没有 1h 缓存写入 tokens 时返回 `0`。

`totalTokens` 计算公式更新为：

```text
inputTokens
+ cacheReadTokens
+ outputTokens
+ cacheWriteTokens
+ cacheWrite1hTokens
```

相关接口：

- `GET /request-traces`

### OpenAPI

实现后运行：

```bash
mise run openapi
pnpm --dir dashboard generate-openapi
```

生成结果需要包含 `cacheWrite1hTokens`。
