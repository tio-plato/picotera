## Plan — 请求 1h 缓存写入 tokens

1. 数据库与 sqlc
   - 新增 goose 迁移，给 `request` 表增加 `cache_write_1h_tokens INTEGER`。
   - 更新 `db/queries/routing.sql` 的 `InsertRequest` 参数和列清单。
   - 更新 `db/queries/request.sql` 的列表、详情、span、完成更新、metrics 更新和 trace 聚合 SQL。
   - 在 trace 聚合中把 `cache_write_1h_tokens` 加入 `total_tokens`。
   - 运行 `sqlc generate`。

2. 合约层
   - 在 `pkg/contract/request.go` 的 `RequestView` 增加 `CacheWrite1HTokens *int32`，JSON 字段名为 `cacheWrite1hTokens`。
   - 在 `RequestTraceView` 增加 `CacheWrite1HTokens int64`。
   - 更新 `requestLike`、`toRequestView`、`ToRequestView`、`ToListRequestRowView`、`ToListRequestsBySpanRowView`、`ToRequestTraceView` 的字段映射。

3. Usage 提取
   - 在 `pkg/server/response_extractor.go` 的 `ResponseMetrics` 增加 `CacheWrite1HTokens *int64`。
   - 抽出 Anthropic cache creation 提取 helper，供 SSE 和 JSON 两条路径共用。
   - 当 `cache_creation.ephemeral_5m_input_tokens` 与 `cache_creation.ephemeral_1h_input_tokens` 同时存在时，分别写入 `CacheWriteTokens` 和 `CacheWrite1HTokens`。
   - 当两个明细字段没有同时存在时，沿用 `cache_creation_input_tokens` 写入 `CacheWriteTokens`，并保持 `CacheWrite1HTokens` 为空。

4. Gateway 写入
   - 更新 `metricsToPG`，返回 `cacheWrite1hTokens`。
   - 更新普通 gateway 成功完成路径，传递并写入 `CacheWrite1HTokens`。
   - 更新 unified gateway 成功完成路径，传递并写入 `CacheWrite1HTokens`。
   - 更新仍使用 `UpdateRequestMetrics` 的调用点，传递新参数。

5. 费用计算
   - 更新 `computeCost` 签名，接收 `cacheWrite1hTokens`。
   - 在计算中使用 `PricingTier.CacheWrite1H` 加入 1h 缓存写入费用。
   - 更新 `costsFor`、普通 gateway、unified gateway 和相关测试调用。
   - 增加费用测试覆盖 `cacheWrite1hTokens` 按 `cacheWrite1h` 单价计费。

6. Dashboard
   - 运行 OpenAPI 生成，更新 dashboard API 类型。
   - 更新 `RequestsView.vue` 的 token 总数和缓存命中比例计算，把 `cacheWrite1hTokens` 纳入输入侧 token。
   - 更新 `TracesView.vue` 的 token 总数、输入侧 token 和缓存命中比例计算。
   - 更新 `RequestDetailsContent.vue`，增加 “1h 缓存写入” 指标。

7. 测试
   - 更新 `pkg/server/response_extractor_test.go`，覆盖：
     - Anthropic JSON 同时包含 5m 和 1h 明细时，普通缓存写入为 5m，1h 缓存写入为 1h。
     - Anthropic SSE `message_start` 同时包含 5m 和 1h 明细时，普通缓存写入为 5m，1h 缓存写入为 1h。
     - 缺少任一明细字段时，回退到 `cache_creation_input_tokens`，1h 缓存写入为空。
   - 运行 `go test ./pkg/server`。
   - 运行 `pnpm --dir dashboard type-check`。
   - 运行 `pnpm --dir dashboard lint`。

8. 生成文件
   - 运行 `mise run openapi`。
   - 运行 `pnpm --dir dashboard generate-openapi`。
   - 确认 `openapi.yaml`、`dashboard/src/openapi-types.d.ts` 和 `dashboard/src/api/openapi.ts` 都包含 `cacheWrite1hTokens`。
