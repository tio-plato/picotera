# 执行计划：SSE 流内错误识别

## 1. `ResponseExtractor` 增加错误捕获

文件：`pkg/server/response_extractor.go`

- 在 `ResponseExtractor` 结构体增加字段 `streamError string`。
- 增加方法 `func (e *ResponseExtractor) StreamError() string { return e.streamError }`。
- 增加方法 `detectStreamError(payload string)`：
  - 用 `gjson.Get(payload, "error.message")` 取值；
  - 若 `e.streamError == "" && v.Exists() && v.Type == gjson.String && v.String() != ""`，则 `e.streamError = v.String()`。
- 在 `processSSEEvent` 中，跳过 `[DONE]` 之后、现有 `extractOpenAISSE` 等调用旁，加 `e.detectStreamError(payload)`。

## 2. 新增 finish_reason 常量

文件：`pkg/db/request_constants.go`

- 在 finish reason 常量块增加 `FinishReasonStreamError = 6`。

## 3. 路径网关完成分支

文件：`pkg/server/gateway_flow_success.go`

- 修改 `completeGatewaySuccess` 签名，新增参数 `streamErr string`。
- 在函数内计算分支变量：
  - 若 `streamErr != ""`：`status := db.RequestStatusFailed`、`errMsg := pgtype.Text{String: streamErr, Valid: true}`、`fr := int32(db.FinishReasonStreamError)`。
  - 否则：`status := db.RequestStatusCompleted`、`errMsg := pgtype.Text{Valid: false}`、`fr := finishReason`（沿用入参）。
- 上游行与 meta 行的 `UpdateRequestOnCompleteParams` 改用 `status` / `errMsg` / `fr`（`StatusCode` 不变，仍为真实上游码）。
- 修改调用处 `streamSuccess`：从 `extractor.StreamError()` 取值，传入 `completeGatewaySuccess`。

## 4. 统一网关完成分支

文件：`pkg/server/gateway_unified_helpers.go`

- 在 `unifiedStreamSuccess` 中，`m := extractor.Metrics()` 之后取 `streamErr := extractor.StreamError()`。
- 计算与第 3 步相同的分支变量 `status` / `errMsg` / `fr`（`fr` 的非错误分支沿用已算出的 `finishReason`）。
- 上游行与 meta 行两个 `UpdateRequestOnCompleteParams` 改用 `status` / `errMsg` / `fr`，`StatusCode` 保持真实上游码。

## 5. 仪表盘 finish_reason 文案

文件：`dashboard/src/components/RequestDetailsContent.vue`

- `finishReasonLabel` 增加 `case 6: return '流式错误'`。
- `finishReasonVariant` 对 `6` 返回 `'default'`（非 `ok`）。

## 6. 测试

文件：`pkg/server/response_extractor_test.go`

- 新增用例：喂入含 Anthropic 风格 `data: {"type":"error","error":{"message":"..."}}` 的 SSE 流，断言 `StreamError()` 返回该 message。
- 新增用例：正常 SSE 流（无 error 事件）断言 `StreamError() == ""`。
- 新增用例：OpenAI Chat 风格 `data: {"error":{"message":"..."}}` 断言被捕获。
- 新增用例：多个 error 事件时取首个 message。

## 7. 验证

- `go build ./...` 与 `go test ./pkg/server/...` 通过。
- `pnpm --dir dashboard type-check` 通过。
- 无需 `sqlc generate`、`mise run openapi`、`generate-openapi`：未改查询与契约。
