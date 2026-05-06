## Design — 模型价格匹配

### 目标

在模型定价编辑位置增加“匹配价格”能力。用户点击按钮后，前端把当前模型名发给后端；后端从 `pkg/pricing/pricing.json` 读取内置价格表，计算若干个最匹配的候选，并把候选价格转换为 PicoTera 现有 `Pricing` 合约形状返回。用户在对话框里选择一个候选并确认后，前端把该候选的 `pricing` 填入当前表单。

### 后端结构

新增 `pkg/pricing` Go 包：

- `pricing.json` 通过 `go:embed` 嵌入二进制。
- 只解析匹配和转换所需字段：provider `id/name`，model `id/name/aliases/currency/unit/prices`，以及每个 price 的 `type/price/tiers/min_input_tokens`。
- 不解析 `sources`、`metadata`、source URL、source 文本、地区说明等展示或溯源字段。
- 包内暴露 `Match(target string, limit int) ([]contract.PricingMatchView, error)`。

`pricing.json` 的输入格式不是公开 API 合约。后端负责把它转换成现有定价结构：

```json
{
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
```

### 价格转换规则

只接受 `unit == "per_1m_tokens"` 的价格条目；其他 unit 的模型不进入候选结果。

字段映射固定如下：

| pricing.json field | PicoTera field |
| --- | --- |
| `input` | `PricingTier.input` |
| `output` | `PricingTier.output` |
| `cache_read` | `PricingTier.cacheRead` |
| `cache_write` | `PricingTier.cacheWrite` |
| `cache_write_long` | `PricingTier.cacheWrite1h` |
| `implicit_cache_read` | `PricingTier.implicitCacheRead` |

`flat` price 转换为 `minInputTokens = 0` 的值。缺失或 `null` 的价格字段按以下规则补齐：

- `input` 缺失：置 0。
- `output` 缺失：置 0。
- `cache_read` 缺失：使用同一 tier 的 `input`。
- `cache_write` 缺失：使用同一 tier 的 `input`。
- `cache_write_long` 缺失：使用同一 tier 的 `input`。
- `implicit_cache_read` 缺失：使用同一 tier 的 `cache_read`；如果 `cache_read` 也缺失，则使用同一 tier 的 `input`。

`tiered` price 只使用 `basis == "input_tokens"` 且 tier 中存在 `min_input_tokens` 的数据。转换时取该模型所有可用价格字段里的 `min_input_tokens` 并集，按升序生成 PicoTera tiers；每个 tier 的字段值使用“该价格字段中 `min_input_tokens <= 当前 tier.minInputTokens` 的最后一个价格”。这样可以把 input/output/cache 各自的阶梯合并成 PicoTera 以输入 token 档位驱动的统一阶梯。生成结果必须通过 `contract.Pricing.Validate()`，否则该模型不进入候选。

补齐规则在 tiered 合并完成后逐 tier 执行，因此继承值使用同一 `minInputTokens` 档位上已经解析出的字段值。

### 匹配算法

后端使用第三方库 `github.com/agnivade/levenshtein` 计算 Levenshtein 编辑距离。该库只负责距离计算；候选过滤、字段转换、排序和 API 合约仍由 PicoTera 代码控制。匹配严格使用传入字符串和价格表里的原始字符串，不做 trim、大小写折叠、空字符串默认值、分词猜测或近似格式修正。

对每个可转换价格的模型计算：

- `modelIdDistance`：`targetModel` 与 pricing model `id` 的 Levenshtein 距离。
- `aliasDistance`：`targetModel` 与每个 alias 的 Levenshtein 距离；没有 alias 时忽略。
- `score`：`modelIdDistance` 与所有 `aliasDistance` 的最小值。

排序规则：

1. `score` 升序。
2. pricing model `id` 与 `targetModel` 完全相等的候选排在同分候选前。
3. provider `id` 升序。
4. pricing model `id` 升序。

接口固定返回最多 8 个候选。`targetModel == ""` 返回 400。

### API 合约

新增合约文件 `pkg/contract/pricing_match.go`，定义请求、响应和 Huma operation。响应里的候选包含用于前端选择和展示的元数据，但价格本身使用现有 `Pricing` 类型。

新增 handler `pkg/server/handle_pricing_match.go`，注册到管理 API group。该接口只读，不访问数据库，不写请求历史。

### 前端结构

`PricingEditor.vue` 增加可选 prop：

- `matchTarget?: string`

当 `matchTarget` 非空时，定价编辑器显示“匹配价格”按钮。点击后调用新增接口，展示候选选择对话框。确认候选后，编辑器直接 `emit('update:modelValue', structuredClone(candidate.pricing))`。

挂载位置：

- `ModelForm.vue`：`<PricingEditor v-model="form.pricing" :match-target="form.name" />`
- `ProviderModelsPanel.vue`：每个上游条目使用 `row.upstreamModelName || row.modelName` 作为 match target。

新增 `PricingMatchDialog.vue` 作为普通 Teleport 对话框，不复用 `ConfirmDialog`，因为该流程需要列表选择而不是单个 destructive confirmation。对话框使用现有 `Overlay`、`Button`、`DataTable`、`Tag`、`MoneyDisplay` 等 primitives，展示 provider、pricing model id、score、输入价、输出价、阶梯数量。点击行选择候选，底部“填入价格”确认。

### OpenAPI 与类型

新增接口后需要运行：

```bash
mise run openapi
pnpm --dir dashboard generate-openapi
```

dashboard 继续通过 `openapi-fetch` 调用接口，不手写 API 类型。

### 不在范围

- 不从外部网络刷新价格表。
- 不把候选匹配结果持久化到数据库。
- 不修改 `pricing.json` 的生成流程或原始 schema。
- 不引入兼容旧定价格式的读取分支。
