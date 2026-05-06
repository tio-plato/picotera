## Plan — 模型价格匹配

1. 后端合约
   - 新增 `pkg/contract/pricing_match.go`。
   - 定义 `MatchPricingRequest`、`MatchPricingResponse`、`PricingMatchCandidate`。
   - 注册 `OperationMatchPricing`，路径为 `POST /pricing/matches`。

2. 后端价格表读取与转换
   - 在 `pkg/pricing` 新增 Go 源文件，通过 `go:embed` 嵌入 `pricing.json`。
   - 定义只包含必要字段的内部 struct。
   - 实现 `Match(target string, limit int)`。
   - 实现 flat/tiered price 到 `contract.Pricing` 的转换。
   - 实现缺失价格字段补齐：缓存读取、缓存写入、长缓存写入默认继承输入价；隐式缓存读取默认继承缓存读取价，缓存读取缺失时继承输入价。
   - 引入 `github.com/agnivade/levenshtein` 计算 Levenshtein 编辑距离。
   - 实现确定性排序。
   - 跳过 unit 不是 `per_1m_tokens`、tiered basis 不是 `input_tokens`、转换后无法通过 `Pricing.Validate()` 的模型。

3. 后端 handler
   - 新增 `pkg/server/handle_pricing_match.go`。
   - 在 handler 中严格拒绝空 `targetModel`。
   - 调用 `pricing.Match(input.Body.TargetModel, 8)` 并返回候选。
   - 在 `pkg/server/server.go` 的 `registerOperations()` 注册新 operation。

4. 后端验证
   - 为 `pkg/pricing` 增加单元测试，覆盖：
     - exact id 匹配排第一。
     - flat 价格转换为单 tier。
     - input_tokens tiered 价格转换为多个 PicoTera tiers。
     - 缺失缓存写入、长缓存写入、隐式缓存读取时按设计规则补齐。
     - 空 target 返回 handler 级 400。
   - 运行 `go test ./pkg/pricing ./pkg/server`。

5. OpenAPI 与 dashboard 类型
   - 运行 `mise run openapi`。
   - 运行 `pnpm --dir dashboard generate-openapi`。
   - 确认 `dashboard/src/openapi-types.d.ts` 和 `dashboard/src/api/openapi.ts` 包含 `matchPricing`。

6. 前端候选对话框
   - 新增 `dashboard/src/components/PricingMatchDialog.vue`。
   - 使用现有 primitives 构建候选表格、选中状态、空结果状态和确认按钮。
   - 候选行展示 provider、modelId、score、输入价、输出价、阶梯数量。
   - 确认时把选中候选的 `pricing` 返回给调用方。

7. 前端定价编辑器集成
   - `PricingEditor.vue` 增加 `matchTarget?: string` prop。
   - 当 `matchTarget` 非空时显示“匹配价格”按钮。
   - 点击按钮调用 `POST /api/picotera/pricing/matches`。
   - 请求成功后打开 `PricingMatchDialog`；确认后把候选 `pricing` emit 到 v-model。
   - 请求失败时在编辑器内显示简短错误文本。

8. 前端挂载点
   - `ModelForm.vue` 把 `form.name` 传给 `PricingEditor` 的 `matchTarget`。
   - `ProviderModelsPanel.vue` 把 `row.upstreamModelName || row.modelName` 传给 `PricingEditor` 的 `matchTarget`。

9. 前端验证
   - 运行 `pnpm --dir dashboard type-check`。
   - 运行 `pnpm --dir dashboard lint`。
   - 对模型表单和 provider model 展开行手动检查：按钮可见、候选对话框可选、确认后表单内定价字段更新。
