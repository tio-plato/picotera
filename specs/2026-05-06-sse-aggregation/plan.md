# 执行计划

1. 扩展 artifact payload 类型
   - 在 `pkg/artifacts/payload.go` 增加 `AggregatedResponse` 结构。
   - 给 `Payload` 增加 `Aggregated *AggregatedResponse` 字段。
   - 增加 `BuildResponseWithAggregated` 和 `BuildResponseWithLogsAndAggregated`，保持现有 `BuildResponse` / `BuildResponseWithLogs` 调用语义不变。
   - 聚合 body 使用 `json.RawMessage`，写入前用 `json.Valid` 校验。

2. 在 `pkg/llmbridge` 增加聚合入口
   - 新建 `aggregate.go`。
   - 实现 `StreamAggregationKind(format, contentType)`，集中判断不聚合、SSE decoder、JSONL decoder 或不支持的 stream media type。
   - 实现 `AggregateStream(ctx, format, contentType, body, profile)`。
   - `FormatGeminiGenerateContent` 永远返回不聚合。
   - OpenAI Chat、OpenAI Responses、Anthropic 只有 `text/event-stream` 才聚合；`application/json` 是普通 non-streaming response，不写 `aggregated`。
   - 根据 normalized content type 选择 decoder。
   - `text/event-stream` 使用 `httpclient.NewDefaultSSEDecoder` 把 body 解成 `[]*httpclient.StreamEvent`。
   - `application/jsonl`、`application/x-ndjson`、`application/jsonlines`、`application/ndjson` 使用新增 JSONL decoder，把每个非空 JSON 行转换成 `StreamEvent{Data: line}`。
   - 当 `format == FormatGeminiStreamGenerateContent` 且 content type 是 `application/json` 时使用 JSONL decoder。
   - 未识别的 stream content type 返回明确错误。
   - 使用 `outboundFor(format, profile).AggregateStreamChunks(ctx, req, chunks)` 输出 non-streaming JSON。
   - 添加单元测试覆盖 OpenAI Chat、OpenAI Responses、Anthropic SSE、Gemini SSE、Gemini JSONL/NDJSON 五类最小 stream。

3. 增加 endpoint type 到 llmbridge format 的 server 辅助函数
   - 在 server 层新增 `responseAggregationFormat(endpointType int32) (llmbridge.Format, bool)`。
   - 覆盖四个流式 generation endpoint type：Anthropic Messages、OpenAI Chat Completions、OpenAI Responses、Gemini StreamGenerateContent。
   - `GeminiGenerateContent` 和非 generation endpoint type 返回 `false`。

4. 改造路径网关 artifact 上传
   - 新增 `uploadResponseArtifactWithAggregation`，保留 `uploadResponseArtifact` 作为无聚合写入入口。
   - `streamSuccess` 增加入参 `endpointType`，调用点传入 `endpoint.EndpointType`。
   - `streamSuccess` 在捕获完整 `respBytes` 后，根据 endpoint type 获取 format，再结合 response content type 判断是否聚合。
   - 判定结果为 SSE 或 JSONL 时，使用 `DefaultOutboundProfileForFormat(format)` 调用 `llmbridge.AggregateStream`。
   - 聚合成功写入 `AggregatedResponse.Body`。
   - 聚合失败写入 `AggregatedResponse.Error` 并记录 warn log。
   - 判定结果为不聚合时不写 `aggregated` 字段。

5. 改造 unified gateway artifact 上传
   - upstream artifact 使用 `a.upFormat`、`upstreamBytes`、`resp.Header.Get("Content-Type")` 和 `a.outboundProfile` 聚合。
   - meta artifact 使用 `a.srcFormat`、`clientBytes`、`metaRespHeader.Get("Content-Type")` 和 `DefaultOutboundProfileForFormat(a.srcFormat)` 聚合。
   - 仅当 `StreamAggregationKind` 判定为 SSE 或 JSONL 时调用聚合。
   - 保持 non-stream bridge 分支只写普通 JSON body，不增加 `aggregated`。

6. 更新 dashboard artifact 类型
   - 在 `RawArtifactView.vue`、`ResponseArtifactView.vue` 和 `LogsArtifactView.vue` 中统一或补齐 `ArtifactPayload` 类型的 `aggregated` 字段。
   - 定义 `AggregatedResponse` TS 类型，避免组件之间重复发散。

7. 删除前端协议级 stream 聚合
   - 从 `dashboard/src/composables/useSSEParser.ts` 删除 `aggregateOpenAIChat`、`aggregateOpenAIResponses`、`aggregateAnthropic` 和 `aggregateSSE`。
   - 保留 `parseSSEEventsForDisplay`、`isSSEContentType`、`renderMarkdown`。
   - 删除 SSE delta 内容聚合逻辑，新增 `extractContentFromAggregated` 只从后端聚合 JSON 提取渲染内容。

8. 改造响应聚合视图
   - `ResponseArtifactView.vue` 的聚合 tab 改为读取 `payload.aggregated`。
   - 有 `aggregated.body` 时展示 `JsonArtifactViewer`。
   - 有 `aggregated.error` 时展示错误。
   - 无 `aggregated` 时显示无后端聚合结果。
   - format label 使用后端 format 枚举。

9. 改造渲染视图内容提取
   - 优先从 `payload.aggregated.body` 提取 thinking 和 reply。
   - 按 OpenAI Chat、OpenAI Responses、Anthropic、Gemini Stream 聚合后的四种 non-stream JSON shape 实现严格字段读取。
   - 无聚合 JSON 时显示无可渲染内容。

10. 验证
    - 运行 `go test ./pkg/llmbridge ./pkg/server`。
    - 运行 `pnpm --dir dashboard type-check`。
    - 运行 `pnpm --dir dashboard build`。
    - 检查 Gemini stream JSONL artifact 能生成 `aggregated.body`，Gemini non-stream artifact 不生成 `aggregated`。
    - 检查新增 spec 文档没有犹豫措辞，检查 artifact Raw / Events / 聚合 / 渲染视图均可用。
