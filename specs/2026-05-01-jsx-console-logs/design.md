# Design — JSX Console Logs in Artifacts

## 总体

JSX 脚本（`pkg/jsx/sdk.js` 暴露的 `console.{log,info,warn,error,debug}`）当前只会落到结构化日志 `logx`。这个特性把整个 gateway 处理过程中产生的所有 JS 日志缓冲在 `Session` 上，请求结束时把它们整体写进 **meta 请求的 response artifact** JSON 里面（`logs` 字段），前端在请求详情侧栏新增一个「日志」Tab 进行展示。

约束：

- 只往 meta response artifact 里塞日志，不分散到各 upstream 或单独 artifact key。
- 一个 Session 对应一次 gateway 调用，覆盖 `sortProviders` + 每次重试的 `beforeRequest` / `rewriteRequest` + 任意脚本顶层执行（脚本 eval 时）。所有这些日志合并成一个有序数组。
- 日志条目结构化：`{level, message, ts}`。

## Session 缓冲

`pkg/jsx/session.go` 中 `Session` 增加：

```go
type LogEntry struct {
    Level   string    `json:"level"`   // "info" | "warn" | "error"
    Message string    `json:"message"`
    Ts      time.Time `json:"ts"`      // RFC3339Nano UTC
}

type Session struct {
    // ... 现有字段
    logsMu  sync.Mutex
    logs    []LogEntry
    logsBytes int  // 累计 message 字节，用于裁剪
}
```

新增方法：

- `(*Session).appendLog(level, message string)` —— 加锁追加，处理裁剪（见下）。
- `(*Session).Logs() []LogEntry` —— 返回当前缓冲快照（拷贝），由 gateway 在写 artifact 前取。

`registerConsole` 改造：构造时持有 `*Session`，在原有 `logx` 输出之外调用 `s.appendLog`。`level` 归一化到三个值 `info|warn|error`（`debug` 当前 SDK 已映射为 `info`，保持不变）。`ts` 使用 `time.Now().UTC()`。

## 防爆裁剪

恶意或失控脚本可能产出海量日志，artifact 大小要受控。固定上限（不开放配置，避免过度灵活）：

- 条数上限：`maxLogEntries = 1000`
- 累计 message 字节上限：`maxLogBytes = 256 * 1024`
- 单条 message 长度上限：`maxLogMessageLen = 8 * 1024`（超过则截断 + 后缀 `... [truncated]`）

触达上限后丢弃后续条目，并保证最后一条始终是哨兵：

```json
{ "level": "warn", "message": "[picotera] log buffer truncated", "ts": "..." }
```

哨兵只追加一次（用 `tainted` 风格的 bool 标记）。这条哨兵不计入裁剪后续判断。

## 时间线与并发

- `console` 调用从 QuickJS 单线程 runtime 进来，串行；但 `__picotera_fetch` 等异步 helper 在 Go 侧用独立 goroutine 跑回调，理论上回到 JS event loop 仍是单线程，但为防止未来扩展引入并发写，`appendLog` 用 `sync.Mutex` 保护。
- 网关只在 hook 同步返回后读 `Logs()`，没有并发读问题。
- `Session.Close()` 不清空 logs；`Logs()` 必须在 Close 之前调用。Gateway 已经把所有 artifact 上传调用排在 hook 链之后、`defer session.Close()` 仍在外层 `defer` 队列中——因此调用顺序天然安全（artifact 上传先取 logs，之后 defer 触发 Close）。

## Artifact Payload 扩展

`pkg/artifacts/payload.go` 现有 `Payload` 增加一个可选字段：

```go
type Payload struct {
    // ... 现有字段
    Logs []LogEntry `json:"logs,omitempty"`
}
```

`LogEntry` 在 artifacts 包里再定义一份，避免 jsx 包反向依赖（artifacts 不依赖 jsx）。jsx 包内部用相同结构，序列化形状一致。Gateway 在调用前把 `[]jsx.LogEntry` 转成 `[]artifacts.LogEntry`（一行 for 循环）。

新增构建函数：

```go
func BuildResponseWithLogs(statusCode int, header http.Header, body []byte, logs []LogEntry) ([]byte, error)
```

旧的 `BuildResponse` 保留：upstream response artifact 不带 logs。

## Gateway 接入点

`pkg/server/handle_gateway.go`：

- 新增 helper `uploadMetaResponseArtifact(ctx, id, ts, status, header, body, logs)`，与 `uploadResponseArtifact` 并存。区别只是构建路径走 `BuildResponseWithLogs`。
- 替换所有「写 meta response artifact」的调用点：
  - `failMetaResponse`（line 90）—— 此时 session 还没创建（出现在 session 创建之前的失败路径），传 `nil`。
  - `failHook`（line 101）—— session 存在，传 `session.Logs()`。
  - 创建 session 失败那条 503/502 路径（line 150）—— session 为 nil，传 `nil`。
  - `streamSuccess` 末尾的 meta response 上传（line 466）—— 传 `session.Logs()`。
  - 全部 provider 失败的兜底（line 366）—— 传 `session.Logs()`。
- 注意只有 meta response artifact 加 logs；upstream response artifact 仍走老路 `uploadResponseArtifact`。
- 由于 `failMetaResponse` 与 session 还没生成的路径是闭包形式，把 `session` 变量提到合适作用域并允许 nil 检查（`if session != nil { logs = session.Logs() }`）。

## 前端展示

`RequestDetailsPanel.vue` 已经有 `Tabs`：「概览 / 原始请求 / 原始响应」。新增第四个 Tab「日志」：

- Tab 仅在当前 `selected` 是 meta（`selected.id === selected.spanId`）时显示——upstream 没有自带 logs 的 artifact。
- 切换到「日志」时拉取 `selected.responseArtifactUrl`，从 JSON 里读 `logs` 字段。复用现有 `RawArtifactView` 的拉取/缓存能力会带来不必要的耦合，这里**新建一个组件** `LogsArtifactView.vue`，自己 fetch + 渲染。
- 渲染：等宽字体表格化展示，每行 `[level chip] [HH:mm:ss.SSS] message`。`level` 用现有 `StateText`/`Tag` 风格的色条：info 中性、warn 橙、error 红，颜色 token 与现有 `bg-warn-faint/text-warn-ink`、`bg-err-faint/text-err-ink` 保持一致（参考 `RequestDetailsPanel.statusCodeClass`）。
- 空数组：`StateText` 显示「无日志」。
- artifact 还没产生（pending 请求 / artifact 关闭）：复用 `RawArtifactView` 的两个降级提示词「未启用 artifact 记录」「artifact 不可用」。

`detailTabs` 改成 `computed`（依赖 `selected`），meta 时长度 4，upstream 时长度 3。`detailTab` watcher 在切换 selected 时已经回到 `'overview'`，无需额外处理「当前 tab 不存在」的情况，但保险起见加一行 fallback。

## 模块布局

- `pkg/jsx/session.go`：`Session` 增加日志字段 + `appendLog` + `Logs`。新增 `LogEntry` 类型（在该文件或 `types.go`）。
- `pkg/jsx/helpers.go`：`registerHelpers` 把 `*Session` 传给 `registerConsole`；console 函数改为既写 logx 又写 session 缓冲。
- `pkg/artifacts/payload.go`：`Payload.Logs` 字段、`LogEntry` 类型、`BuildResponseWithLogs`。
- `pkg/server/handle_gateway.go`：新增 `uploadMetaResponseArtifact`，替换 5 处 meta response artifact 上传调用。
- `dashboard/src/components/LogsArtifactView.vue`：新组件。
- `dashboard/src/components/RequestDetailsPanel.vue`：tab 列表改 computed，加分支渲染 `LogsArtifactView`。

## 不在范围

- 不暴露 logs 给 huma 管理 API（仍走 artifact presigned URL）。
- 不为 logs 引入单独的 artifact key / S3 对象（避免多一次上传 + 一次额外 fetch）。
- 不区分日志是哪个 hook 阶段产生的（用户未要求；时间戳已足够定位）。
- 不为前端 logs 加搜索/过滤（v1 朴素列表）。
- 不持久化到数据库——artifact 即为唯一存储。
