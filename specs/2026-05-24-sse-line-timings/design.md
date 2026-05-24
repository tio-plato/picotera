# 设计：SSE 响应逐行时间记录

## 概述

在 SSE 流式响应过程中，按行记录每行到达的时间偏移量，将其嵌入 response artifact，供 dashboard 在 events 视图和 raw 视图中展示。

## 后端

### LineTimingRecorder

新增 `LineTimingRecorder`（位于 `pkg/server/line_timing_recorder.go`），实现 `io.Reader`。它包装上游 reader，在每次 `Read()` 返回的字节中扫描 `\n`，记录每个 `\n` 到达时的时间偏移（毫秒，float64，相对于 startTime）。

时间精度：同一次 `Read()` 调用返回的所有 `\n` 共享同一个时间戳（因为它们确实在同一时刻到达）。

### 插入位置

`ResponseExtractor` 已经在读取上游字节的 reader 链中。将 `LineTimingRecorder` 放在 `ResponseExtractor` 之后、`UpstreamTee` 之前：

```
internalBody → ResponseExtractor → LineTimingRecorder → UpstreamTee → upstreamCapture
```

这样 timings 数组与 `upstreamCapture` 中的字节一一对应（按 `\n` 计数）。

对 `streamSuccess`（path-based gateway）同理：

```
internalBody → ResponseExtractor → LineTimingRecorder → idleTimeoutReader → captureBuf
```

### Artifact Payload 变更

在 `pkg/artifacts/payload.go` 的 `Payload` struct 新增：

```go
Timings []float64 `json:"timings,omitempty"`
```

修改内部 `buildResponse` 函数签名增加 `timings []float64` 参数，并将其赋值给 `Payload.Timings`。更新所有公开的 Build 函数（`BuildResponse`、`BuildResponseWithAggregated`、`BuildResponseWithLogs`、`BuildResponseWithLogsAndAggregated`）以透传该参数。

### 上传调用变更

修改 `uploadResponseArtifactWithAggregation` 和 `uploadMetaResponseArtifactWithAggregation` 以接受 `timings []float64` 参数，传入 Build 函数。

在 `unifiedStreamSuccess` 和 `streamSuccess` 中：
- upstream artifact 传入 `timingRecorder.Timings`
- meta artifact：非 bridging 时传入相同的 timings；bridging 时传 nil（bridging 后字节结构变化，行级 timing 不再对应）

## 前端

### 类型变更

`dashboard/src/components/artifactTypes.ts` 的 `ArtifactPayload` 新增：

```typescript
timings?: number[]
```

### SSE Events 视图

`useSSEParser.ts`：
- `ParsedSSEEvent` 增加 `timeMs?: number` 字段
- `parseSSEEventsForDisplay` 增加可选 `timings` 参数
- 解析时追踪已消耗的 `\n` 数量，用该计数索引 timings 数组，取每个 event 第一行的 timing 作为该 event 的到达时间

`SSEEventsVirtualList.vue`：
- 在每个 event 卡片的 header 中，index 之后显示 `timeMs`（格式如 `+1234ms`）

`ResponseArtifactView.vue`：
- 将 `payload.timings` 传入 `parseSSEEventsForDisplay`

### Raw 视图

`ResponseArtifactView.vue`：
- raw 子视图顶部增加一个复选框："显示到达时间"
- 不勾选时，保持现有 `<pre>` 渲染
- 勾选时，将 body 按 `\n` 分行，结合 timings 渲染为表格，左列为时间值（ms），右列为该行内容（monospace）
- 新增组件 `TimedRawView.vue` 负责表格渲染，使用 `@tanstack/vue-virtual` 做虚拟滚动（与 SSEEventsVirtualList 一致）
