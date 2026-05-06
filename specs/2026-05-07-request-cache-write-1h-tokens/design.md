## Design — 请求 1h 缓存写入 tokens

### 目标

请求记录需要区分 Anthropic 缓存写入的 5 分钟缓存和 1 小时缓存。现有 `request.cache_write_tokens` 继续表示普通缓存写入 tokens；当 Anthropic 返回 `usage.cache_creation` 明细时，该字段表示 `ephemeral_5m_input_tokens`。新增 `request.cache_write_1h_tokens` 表示 `ephemeral_1h_input_tokens`。

### 数据模型

新增迁移给 `request` 表增加 nullable integer 列：

```sql
ALTER TABLE request ADD COLUMN cache_write_1h_tokens INTEGER;
```

该列只在上游返回可识别的 1h 缓存写入 tokens 时写入。旧数据保持 `NULL`。

所有 `request` 查询、插入、完成更新和 trace 聚合都纳入该列：

- `InsertRequest` 写入 nullable `cache_write_1h_tokens`。
- `UpdateRequestOnComplete` 更新 `cache_write_1h_tokens`。
- `UpdateRequestMetrics` 更新 `cache_write_1h_tokens`。
- `ListRequests`、`GetRequest`、`ListRequestsBySpan` 返回 `cache_write_1h_tokens`。
- `ListRequestTraces.total_tokens` 将 `cache_write_1h_tokens` 计入总 token。
- `ListRequestTraces` 增加聚合字段 `cache_write_1h_tokens`。

### Usage 提取

`ResponseMetrics` 增加 `CacheWrite1HTokens *int64`。

Anthropic JSON 响应和 Anthropic SSE `message_start` 事件按同一规则解析：

1. 如果 `usage.cache_creation.ephemeral_5m_input_tokens` 和 `usage.cache_creation.ephemeral_1h_input_tokens` 都存在：
   - `CacheWriteTokens = ephemeral_5m_input_tokens`
   - `CacheWrite1HTokens = ephemeral_1h_input_tokens`
2. 如果两个明细字段没有同时存在：
   - `CacheWriteTokens = usage.cache_creation_input_tokens`
   - `CacheWrite1HTokens = nil`

解析逻辑要求字段存在，不把缺失字段视为 0。字段值使用 gjson 的整数读取方式，与现有 token 提取保持一致。

OpenAI Chat Completions、OpenAI Responses 和其他非 Anthropic 格式不写入 `CacheWrite1HTokens`。

### 费用计算

现有定价结构已经有 `PricingTier.cacheWrite1h`。`computeCost` 增加 `cacheWrite1hTokens` 参数，并按当前 tier 的 `CacheWrite1H` 单价计费。`costsFor`、普通 gateway 和 unified gateway 都传入新指标。

当新列为 `NULL` 时，1h 缓存写入费用为 0。旧的 `cache_write_tokens` 回退行为保持不变，因此没有明细的 Anthropic 响应仍使用 `cache_creation_input_tokens` 按普通缓存写入单价计费。

### API 与前端

管理 API 的请求视图增加 `cacheWrite1hTokens` 字段，trace 聚合视图增加同名字段。OpenAPI 重新生成后 dashboard 通过生成类型消费该字段。

dashboard 在请求列表、trace 列表和请求详情中把 1h 缓存写入纳入 token 总数展示。请求详情增加单独的 “1h 缓存写入” 指标，以便用户区分 5m 和 1h 缓存写入。

### 不在范围

- 不回填历史请求。
- 不改变 `cache_creation_input_tokens` 回退语义。
- 不为非 Anthropic 格式猜测 1h 缓存写入。
- 不增加旧 API 字段名别名。
