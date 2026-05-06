# 请求表增加 1h 缓存写入 tokens

请求表增加 1 小时缓存写入 tokens。

Anthropic usage 样例如下：

```json
{
  "usage": {
    "input_tokens": 1,
    "cache_creation_input_tokens": 668,
    "cache_read_input_tokens": 61127,
    "cache_creation": {
      "ephemeral_5m_input_tokens": 0,
      "ephemeral_1h_input_tokens": 668
    },
    "output_tokens": 1,
    "service_tier": "standard",
    "inference_geo": "not_available"
  }
}
```

如果能识别到 `cache_creation.ephemeral_5m_input_tokens` 和 `cache_creation.ephemeral_1h_input_tokens` 这两个值，那么缓存写入按 5m 的值计算，这里是 `0`；1h 缓存写入 tokens 则按 1h 的值计算，这里是 `668`。

如果读不到这两个明细值，则还按以前的逻辑处理，即缓存创建使用 `cache_creation_input_tokens`，没有 1h 缓存写入 tokens。
