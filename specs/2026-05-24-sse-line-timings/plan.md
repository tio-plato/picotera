# 执行计划：SSE 响应逐行时间记录

## Step 1: LineTimingRecorder

新建 `pkg/server/line_timing_recorder.go`：

```go
type LineTimingRecorder struct {
    inner     io.Reader
    startTime time.Time
    Timings   []float64
}
```

实现 `io.Reader`：每次 `Read()` 返回后，扫描 `p[:n]` 中的 `\n` 字符，对每个 `\n` 追加 `time.Since(startTime)` 的毫秒值到 `Timings`。同一次 `Read()` 中的所有 `\n` 共享同一时间戳。

## Step 2: Artifact Payload 增加 Timings 字段

修改 `pkg/artifacts/payload.go`：

1. `Payload` struct 增加 `Timings []float64 \`json:"timings,omitempty"\``
2. `buildResponse` 内部函数增加 `timings []float64` 参数，赋值给 `p.Timings`
3. 更新四个公开 Build 函数（`BuildResponse`、`BuildResponseWithAggregated`、`BuildResponseWithLogs`、`BuildResponseWithLogsAndAggregated`）增加 `timings []float64` 参数并透传

## Step 3: 上传辅助函数适配

修改 `pkg/server/handle_gateway.go`（`gateway_helpers.go` 中的辅助函数部分）：

1. `uploadResponseArtifact` 增加 `timings []float64` 参数
2. `uploadResponseArtifactWithAggregation` 增加 `timings []float64` 参数
3. `uploadMetaResponseArtifact` 增加 `timings []float64` 参数
4. `uploadMetaResponseArtifactWithAggregation` 增加 `timings []float64` 参数

将 timings 传入对应的 Build 函数。

## Step 4: 在 streamSuccess 中接入 LineTimingRecorder

修改 `pkg/server/handle_gateway.go` 的 `streamSuccess`：

1. 在 `NewResponseExtractor` 之后创建 `LineTimingRecorder` 包装 extractor
2. 将 `timingRecorder`（而非原 extractor）传给 `newIdleTimeoutReader`
3. 调用 `uploadResponseArtifactWithAggregation` 和 `uploadMetaResponseArtifactWithAggregation` 时传入 `timingRecorder.Timings`

## Step 5: 在 unifiedStreamSuccess 中接入 LineTimingRecorder

修改 `pkg/server/handle_unified_gateway.go` 的 `unifiedStreamSuccess`：

1. 在 `NewResponseExtractor` 之后创建 `LineTimingRecorder` 包装 extractor
2. 将 `asReadCloser(timingRecorder, internalBody)` 传给 `NewUpstreamTee`（替换原来的 `asReadCloser(extractor, internalBody)`）
3. upstream artifact 传入 `timingRecorder.Timings`
4. meta artifact：如果 `!transforming` 传入相同 timings，否则传 nil

## Step 6: 更新所有其他 upload 调用点

搜索所有调用 `uploadResponseArtifact`、`uploadResponseArtifactWithAggregation`、`uploadMetaResponseArtifact`、`uploadMetaResponseArtifactWithAggregation` 的地方，在非流式路径传 `nil` 作为 timings。

## Step 7: 前端类型更新

1. `dashboard/src/components/artifactTypes.ts`：`ArtifactPayload` 增加 `timings?: number[]`
2. `dashboard/src/composables/useSSEParser.ts`：
   - `ParsedSSEEvent` 增加 `timeMs?: number`
   - `parseSSEEventsForDisplay` 增加可选参数 `timings?: number[]`
   - 解析时追踪已消耗的换行数，用来索引 timings 数组，将每个 event 的第一行 timing 赋给 `timeMs`

## Step 8: SSEEventsVirtualList 展示时间

修改 `dashboard/src/components/SSEEventsVirtualList.vue`：

在 event 卡片 header 的 `#{{ event.index + 1 }}` 后面增加时间显示：如果 `event.timeMs != null`，显示 `+{timeMs}ms` 或格式化为 `+{seconds}s`（超过 1000ms 时），使用 `text-ink-faint` 样式。

## Step 9: Raw 视图增加 "显示到达时间" 功能

修改 `dashboard/src/components/ResponseArtifactView.vue`：

1. 增加 `showTimings` ref（boolean，默认 false）
2. 在 raw 子视图的条件中：当 `payload.timings?.length` 时，渲染一个复选框 "显示到达时间"
3. 勾选时，切换到 `TimedRawView` 组件；不勾选时，保持现有 `<pre>` 渲染

## Step 10: 新建 TimedRawView 组件

新建 `dashboard/src/components/TimedRawView.vue`：

- props: `body: string`、`timings: number[]`
- 将 body 按 `\n` 分行，与 timings 数组对齐
- 使用 `@tanstack/vue-virtual` 虚拟滚动
- 每行渲染为一行，左侧固定宽度列显示 timing 值（ms），右侧显示行内容（monospace，保留空白）
- 最大高度 480px，与现有 raw 视图一致
