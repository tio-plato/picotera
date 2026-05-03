## API — 模型定价

所有路径在 `/api/picotera` 下。

### 通用类型

#### `Pricing`

```jsonc
{
  "currency": "USD",                  // ISO 4217；必须存在于 exchange_rate 表
  "tiers": [
    {
      "minInputTokens": 0,            // 升序；首项必须为 0
      "input": 3.0,                   // per 1M tokens，>=0
      "output": 15.0,
      "cacheRead": 0.3,
      "cacheWrite": 3.75,
      "cacheWrite1h": 6.0,
      "implicitCacheRead": 0.0
    }
  ]
}
```

未定价用：缺省字段 / `null` / `{ "currency": "USD", "tiers": [] }`。后端在反序列化时把后两种规整为 `nil`。

#### `ExchangeRateView`

```jsonc
{
  "code": "USD",
  "name": "US Dollar",
  "symbol": "$",
  "unitsPerUsd": 1.0
}
```

### 1. 汇率 CRUD

| Method | Path                              | Operation ID         |
| ------ | --------------------------------- | -------------------- |
| GET    | `/exchange-rates`                 | `listExchangeRates`  |
| GET    | `/exchange-rates/{code}`          | `getExchangeRate`    |
| PUT    | `/exchange-rates`                 | `putExchangeRate`    |
| POST   | `/exchange-rates/delete`          | `deleteExchangeRate` |

`putExchangeRate` body：完整 `ExchangeRateView`；upsert 语义。`unitsPerUsd > 0`，否则 400。

`deleteExchangeRate` body `{ code }`。`code === "USD"` 返回 400 `cannot delete base currency`。

### 2. 模型 — 增加 `pricing`

`ModelView` 扩展：

```jsonc
{
  "name": "claude-sonnet-4-6",
  "title": "Claude Sonnet 4.6",
  "developer": "Anthropic",
  "series": "Claude",
  "disabled": false,
  "pricing": null | Pricing
}
```

- `GET /models` / `GET /models/{name}` 在响应里多带 `pricing` 字段。
- `PUT /models` 接受 `pricing`。校验失败 400；通过则按 JSONB 落库。

### 3. Provider — `ProviderModelEntry.pricing`

`ProviderModelEntry` 扩展：

```jsonc
{
  "model": "claude-sonnet-4-6",
  "upstreamModelName": "claude-sonnet-4-6-20251001",
  "endpoints": ["/v1/messages"],
  "priority": 0,
  "annotations": { "tier": "preview" },
  "disabled": false,
  "pricing": null | Pricing
}
```

`createProvider` / `upsertProvider` 接受、`getProvider` / `listProviders` 返回。

### 4. 请求 — `RequestView` 增加成本字段

`RequestView` 扩展（META 与 UPSTREAM 两类行都会写）：

```jsonc
{
  "id": "...",
  // ... 既有字段 ...
  "modelCost": 0.000123,           // 用 model.pricing 计算的金额；可空
  "modelCostCurrency": "USD",      // ISO 4217；与 modelCost 同步空/非空
  "upstreamCost": 0.000130,        // 用 provider.providerModels[].pricing 计算的金额；可空
  "upstreamCostCurrency": "USD"
}
```

精度：6 位小数。后端在 `updateRequestOnComplete` 时计算并落库，路径如下：
- 找 `request.model` 对应的 `model` 行 → 计算模型成本。
- 找 `request.providerId` 的 `provider.providerModels[]` 中 `model == request.model` 的条目 → 计算上游成本。
- 任何一边缺定价或缺必要 token 计数 → 该侧两列保持 `NULL`。

`listRequests` / `getRequest` / `listRequestSpans` 三个接口的查询都需要 SELECT 新列；不新增独立 endpoint。
