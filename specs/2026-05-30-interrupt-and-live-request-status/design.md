# 设计：打断进行中请求 + 实时状态

## 概述

两块能力共用一个新的**进程内进行中请求注册表**（`liveRequestRegistry`）：

1. **打断**：注册表为每个进行中的请求行（meta + 每次 upstream 尝试）保存一个取消函数；管理 API 调用它取消对应的上下文。控制流差异（走下一个 provider / 直接中断）由「取消发生在收到 headers 之前还是之后」自然产生，无需额外分支。
2. **实时状态**：注册表为每次 upstream 尝试保存一份内存中的进度（是否收到 headers、字节数、至今响应体）。meta 行在某次尝试开始流式回传后镜像该尝试的进度。管理 API 读取快照返回。

注册表只在内存中，单实例生效，请求结束即清理。不涉及 DB schema、不新增 `request` 字段。

## 进行中请求注册表

新文件 `pkg/server/live_requests.go`。

map 直接用 `github.com/orcaman/concurrent-map/v2`（字符串键），不手写 `sync.RWMutex`。`liveProgress` 仍保留一把小锁，因为它内含 `bytes.Buffer`——并发的 `recordChunk`（流式 goroutine 写）与 `Snapshot`（API goroutine 读）必须互斥，concurrent-map 只保护 map 结构本身、不保护值内部。依赖 `go get github.com/orcaman/concurrent-map/v2`（当前未在 `go.mod`）。

```go
import cmap "github.com/orcaman/concurrent-map/v2"

type liveRequestRegistry struct {
    entries cmap.ConcurrentMap[string, *liveEntry] // key: 请求行 ID（meta ID 或 upstream ID）
}

func newLiveRequestRegistry() *liveRequestRegistry {
    return &liveRequestRegistry{entries: cmap.New[*liveEntry]()}
}

type liveEntryKind int
const ( liveKindMeta liveEntryKind = iota; liveKindUpstream )

type liveEntry struct {
    kind   liveEntryKind
    cancel context.CancelFunc // meta → 取消 flow 上下文；upstream → 取消该尝试上下文

    // upstream 行自身的进度；meta 行此字段为 nil
    progress *liveProgress

    // 仅 meta 行使用：指向当前正在流式回传的 upstream 的 progress（nil 表示还没有 upstream 进入流式）
    active atomic.Pointer[liveProgress]

    // 被 dashboard 主动打断时写入的停止原因（finish reason 值；0 表示未被打断）
    stopReason atomic.Int32
}

type liveProgress struct {
    mu              sync.Mutex
    headersReceived bool
    statusCode      int
    bytes           int64
    body            bytes.Buffer // 完整保留至今全部内容
    startedAt       time.Time
    lastChunkAt     time.Time
}
```

注册表方法：

- `RegisterMeta(id string, cancel context.CancelFunc) *liveEntry`
- `RegisterUpstream(id string, cancel context.CancelFunc) *liveEntry`（内部建 `progress`）
- `Remove(id string)`
- `Interrupt(id string, reason int32) bool`：取 entry，`stopReason.Store(reason)`，调用 `cancel()`，返回是否存在。
- `StopReason(id string) int32`：取 entry 的 `stopReason`（不存在返回 0）。
- `Snapshot(id string) (liveSnapshot, bool)`：取 entry；meta 行跟随 `active` 指针读取对应 upstream 的 progress（无 active 则为 pending 阶段）；upstream 行读自身 progress。在 `liveProgress.mu` 下拷贝出 body 字符串与计数。

`liveProgress` 方法：

- `markHeaders(statusCode int)`：置 `headersReceived=true`、`statusCode`。
- `recordChunk(p []byte)`：`body.Write(p)`、`bytes += len(p)`、更新 `lastChunkAt`。

`liveEntry` 与 `*liveProgress` 在注册时即创建，调用方在尝试/链路结束时 `Remove`，进度内存随之释放。

注册表实例挂在 `*Server`（`server.go`）上：`liveRequests *liveRequestRegistry`，`NewServer` 中初始化。`gatewayHandler` 内嵌 `*Server`，可直接用 `h.liveRequests`；Huma handler 是 `*Server` 方法，用 `s.liveRequests`。

## 取消上下文改造

`pkg/server/gateway_flow_context.go` 的 `gatewayContexts`：

- 现状 `Request = r.Context()`（不可主动取消）。改为：`Request, cancelRequest := context.WithCancel(r.Context())`，`gatewayContexts` 保存 `cancelRequest`，并暴露给 flow。
- `persistBase` 仍由 `context.WithoutCancel(requestCtx)` 派生，**不受 flow 取消影响**，保证打断后落库与 artifact 上传照常完成。
- `run()` 中 `defer` 调用 `cancelRequest`（避免上下文泄漏）。

每次 upstream 尝试的 `attemptCtx, cancel := context.WithCancel(f.ctxs.Request)` 不变（派生自可取消的 flow 上下文）。

### meta 打断

- `insertMetaRequest` 写入 meta 行后：`f.h.liveRequests.RegisterMeta(metaID, f.ctxs.cancelRequest)`。
- `run()` 结束 `defer f.h.liveRequests.Remove(metaID)`。
- `Interrupt(metaID, FinishReasonDashboardCancelled)` → 取消 flow 上下文 → 级联取消当前 `attemptCtx`。
- 为确保不再尝试下一个 provider，在 `runAttempts` 主循环顶部加守卫：`if f.ctxs.Request.Err() != nil { break }`。

flow 上下文被取消后：
- 收到 headers 前：`forwardRequest` 返回 `context.Canceled`，记录失败后回到循环顶部，守卫 `break`。
- 收到 headers 后：流式读取中断，success handler 完成两行后链路结束。

两种情况的 finish reason 都经下文「停止原因」统一解析为 `FinishReasonDashboardCancelled`。

### upstream 打断

- `insertUpstreamAttempt` 写入 upstream 行后：`entry := f.h.liveRequests.RegisterUpstream(upstreamID, cancel)`，把 `entry`/`entry.progress` 透传进 `attemptInput`，供流式循环记录进度。
- `runSingleAttempt` 在拿到 `input` 后 `defer f.h.liveRequests.Remove(input.UpstreamID)`，覆盖成功与各失败返回路径。
- `Interrupt(upstreamID, FinishReasonDashboardCancelled)` → 仅取消该 `attemptCtx`，**不影响 flow 上下文**。

控制流差异由时机决定，无需显式分支：
- **headers 之前**：`attemptCtx` 取消 → `forwardRequest` 返回 `context.Canceled`。此时 `reqCtx`（flow）未取消，记录失败后主循环**继续走下一个 provider**。
- **headers 之后**：`attemptCtx` 取消 → 上游 body 读取报错 → 流式循环中断，客户端收到截断响应；success handler 完成该行，链路结束，**不再尝试下一个 provider**。

### 停止原因（dashboardCancelled）

新增 finish reason 常量 `FinishReasonDashboardCancelled = 7`（`pkg/db/request_constants.go`），与已有的 `FinishReasonCancelled`（客户端断开 / 上下文取消）区分开——前者明确表示「dashboard 主动打断」。

落库时不依赖上下文取消的默认归类，而是统一查询注册表的停止原因。新增 flow 辅助：

```
func (f *gatewayFlow) finishReasonFor(rowID string, fallback int32) int32 {
    // 该行自身被打断 → 用其停止原因
    if r := f.h.liveRequests.StopReason(rowID); r != 0 { return r }
    // meta 打断会级联取消正在进行的 upstream：upstream 行回落到 meta 的停止原因
    if r := f.h.liveRequests.StopReason(f.meta.ID); r != 0 { return r }
    return fallback
}
```

- meta 行：`finishReasonFor(metaID, 默认归类)`。
- upstream 行：`finishReasonFor(upstreamID, 默认归类)` —— upstream 自身被打断时命中自身；meta 被打断级联取消时回落到 meta 的停止原因，使两行一致显示 `dashboardCancelled`。

涉及点（把原先直接传入的 `classifyForwardError` / `classifyStreamFinishReason` 结果改为经 `finishReasonFor` 解析）：
- `runSingleAttempt` 的 `forwardRequest` 错误分支（`recordAttemptFailure` 的 finish reason）。
- `pipePathResponse` → `completeGatewaySuccess`（meta + upstream 两行）。
- `unifiedStreamSuccess` 流式结束后的完成逻辑（meta + upstream 两行）。

不复用现有 status：被取消的请求按既有规则仍是 `Failed`（失败尝试）或在 success 路径写入对应状态，仅 finish reason 体现 dashboard 取消。

## 流式集成点

实时进度在两条 success 路径上记录。

### 路径网关 `gateway_flow_success.go`

- `markPathHeadersReceived(input)`：除现有 DB 回填外，
  - `up := liveRequests.entry(upstreamID)`；`up.progress.markHeaders(statusCode)`；
  - `meta := liveRequests.entry(metaID)`；`meta.active.Store(up.progress)`（meta 行开始镜像该 upstream）。
- `pipePathResponse` 主循环：在 `captureBuf.Write(buf[:n])` 旁，`up.progress.recordChunk(buf[:n])`。
- 流式结束：finish reason 经 `finishReasonFor`（见上）解析后传入 `completeGatewaySuccess`。

### 统一网关 `gateway_unified_helpers.go`

- `unifiedStreamSuccess` 收到 headers（写 `RequestStatusHeaderReceived` 处）：同样 `markHeaders` + 设置 meta `active`。
- 客户端写出主循环（`clientCapture.Write(buf[:n])` 旁）：`up.progress.recordChunk(buf[:n])`，记录**客户端可见字节**（与浏览器所见一致；桥接时为转换后内容）。
- 流式结束：finish reason 经 `finishReasonFor` 解析后写入。

记录在「客户端可见字节」上，使实时响应体与最终用户看到的内容一致。

## API

新增两个 `/api/picotera` 操作（详见 `api.md`）：

- `POST /requests/{id}/interrupt`：打断指定请求行（meta 或 upstream），返回是否成功打断。
- `GET /requests/{id}/live`：返回该请求行的内存实时快照（是否进行中、阶段、字节数、响应体至今内容等）。

两者均按请求行 ID 工作；ID 是否在飞行中由注册表判定。`live` 对已结束/不存在的行返回 `inFlight=false`（其余字段为空），便于前端优雅降级；`interrupt` 返回 `interrupted=false` 表示该行已不在飞行中（竞态下的无操作），不报错。

## 前端

- `src/api/client.ts`：新增 `getRequestLive(id)`、`interruptRequest(id)`，并配套 `src/api/queryKeys.ts` 的 `requestLive` key 与失效函数。
- `RequestDetailsContent.vue`（span 列表所在组件）：
  - 判断某 span 是否进行中：复用现有 `status === 0 || status === 1`。
  - 对进行中的 span 显示「打断」按钮；点击调用 `interruptRequest(span.id)`，成功后失效 spans 与 live 查询。
  - 选中某进行中 span 时，在详情区展示实时快照：阶段（pending / 已收到 headers / 流式中）、字节数、响应体至今内容（文本）。
  - 新增「刷新」动作，手动重新拉取 spans 与当前选中行的 live 快照。**不做自动轮询。**
  - live 查询用 vue-query，但 `enabled` 仅在选中行进行中时开启，且不设 `refetchInterval`（手动触发）。
- 改完后端 contract 后按项目流程执行 `mise run openapi` + `pnpm --dir dashboard generate-openapi`，再写前端。

## 内存与清理

- 每个进行中请求的响应体完整驻留内存，`Remove` 后释放。单实例、并发量有限，可接受（用户已确认）。
- `Remove` 在 `run()`（meta）与 `runSingleAttempt`（upstream）的 defer 中执行，覆盖所有正常与异常退出路径，无泄漏。

## 非目标

- 多实例 / 跨进程打断与状态查询。打断与实时状态仅对处理该请求的本进程有效。
- 不持久化实时状态，不新增 DB 字段或 status 值。
