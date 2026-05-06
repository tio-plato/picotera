## API — 模型价格匹配

所有路径在 `/api/picotera` 下。

### `PricingMatchCandidate`

```jsonc
{
  "providerId": "anthropic",
  "providerName": "Anthropic",
  "modelId": "claude-sonnet-4-6",
  "modelName": "Claude Sonnet 4.6",
  "score": 0,
  "pricing": {
    "currency": "USD",
    "tiers": [
      {
        "minInputTokens": 0,
        "input": 3,
        "output": 15,
        "cacheRead": 0.3,
        "cacheWrite": 3.75,
        "cacheWrite1h": 6,
        "implicitCacheRead": 0
      }
    ]
  }
}
```

`pricing` 字段必须是现有 `Pricing` schema，不暴露 `pricing.json` 内部字段。

### `POST /pricing/matches`

Operation ID: `matchPricing`

Request body:

```jsonc
{
  "targetModel": "claude-sonnet-4-6"
}
```

Validation:

- `targetModel` 必须非空。
- 后端不 trim、不大小写折叠、不把空字符串转换为默认值。

Response body:

```jsonc
{
  "candidates": [
    {
      "providerId": "anthropic",
      "providerName": "Anthropic",
      "modelId": "claude-sonnet-4-6",
      "modelName": "Claude Sonnet 4.6",
      "score": 0,
      "pricing": {
        "currency": "USD",
        "tiers": [
          {
            "minInputTokens": 0,
            "input": 3,
            "output": 15,
            "cacheRead": 0.3,
            "cacheWrite": 3.75,
            "cacheWrite1h": 6,
            "implicitCacheRead": 0
          }
        ]
      }
    }
  ]
}
```

Status codes:

- `200`：返回 0 到 8 个候选。
- `400`：`targetModel == ""`。
- `500`：嵌入价格表无法解析或转换流程出现内部错误。
