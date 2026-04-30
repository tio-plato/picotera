# Plan — JSX Console Logs in Artifacts

## 1. JSX session 日志缓冲

**文件**：`pkg/jsx/session.go`

- 在文件顶部新增 `LogEntry` 类型：

  ```go
  type LogEntry struct {
      Level   string    `json:"level"`
      Message string    `json:"message"`
      Ts      time.Time `json:"ts"`
  }
  ```

- 在 `Session` struct 增加字段：

  ```go
  logsMu       sync.Mutex
  logs         []LogEntry
  logsBytes    int
  logsTrunc    bool   // 已经追加过哨兵
  ```

  导入 `sync`。

- 新增方法：

  ```go
  func (s *Session) appendLog(level, message string)
  func (s *Session) Logs() []LogEntry
  ```

  - `appendLog`：
    1. `level` 归一化：仅接受 `info|warn|error`；其他映射到 `info`。
    2. message 长度 > 8KB → 截断到 8KB - len(suffix) 后追加 `... [truncated]`。
    3. 加锁后判断 `logsTrunc`：true → return。
    4. 判断 `len(s.logs) >= 1000` 或 `s.logsBytes + len(message) > 256*1024`：
       - 是 → 追加哨兵 `{level: "warn", message: "[picotera] log buffer truncated", ts: now}`，置 `logsTrunc = true`，return。
       - 否 → 追加正常条目，`s.logsBytes += len(message)`。
  - `Logs`：加锁，`out := append([]LogEntry(nil), s.logs...)` 返回拷贝。

## 2. console 接到 session 缓冲

**文件**：`pkg/jsx/helpers.go`

- `registerHelpers(s)` 现已传 `*Session`，把 `registerConsole` 签名改成 `registerConsole(s *Session)` 并在内部读 `s.requestID` / `s.rt.Context()`。
- 在 `c.SetFunc("__picotera_console", ...)` 回调里，写 logx 之后追加：

  ```go
  s.appendLog(level, msg)
  ```

  注意 `level` 在 logx 里是把空字符串当 `info`，appendLog 内部归一化；保持一致。

## 3. Artifact payload 扩展

**文件**：`pkg/artifacts/payload.go`

- 新增类型（与 jsx 包 LogEntry 字段、tag 完全对齐）：

  ```go
  type LogEntry struct {
      Level   string    `json:"level"`
      Message string    `json:"message"`
      Ts      time.Time `json:"ts"`
  }
  ```

- `Payload` 增加：

  ```go
  Logs []LogEntry `json:"logs,omitempty"`
  ```

- 新增构建函数：

  ```go
  func BuildResponseWithLogs(statusCode int, header http.Header, body []byte, logs []LogEntry) ([]byte, error) {
      p := Payload{
          StatusCode: statusCode,
          Headers:    normalizeHeader(header),
          Logs:       logs,
      }
      encodeBody(&p, body)
      return marshalAndCompress(&p)
  }
  ```

  `BuildResponse` 不动。

## 4. Gateway 接入

**文件**：`pkg/server/handle_gateway.go`

- 新增 helper：

  ```go
  func (h *gatewayHandler) uploadMetaResponseArtifact(
      ctx context.Context, id string, ts time.Time,
      statusCode int, header http.Header, body []byte,
      logs []artifacts.LogEntry,
  ) {
      if !h.artifacts.Enabled() {
          return
      }
      payload, err := artifacts.BuildResponseWithLogs(statusCode, header, body, logs)
      if err != nil {
          logx.WithContext(ctx).WithError(err).WithField("id", id).Warn("artifact: build meta response failed")
          return
      }
      h.artifacts.Put(ctx, artifacts.ResponseKey(id, ts), payload)
  }
  ```

- 在 `ServeHTTP` 顶部把 `var session *jsx.Session` 提前到 step 4 之前（紧挨着 `metaCreatedAt` 后），并在原本创建处赋值。

- 新增 helper（闭包内部使用）：

  ```go
  collectLogs := func() []artifacts.LogEntry {
      if session == nil { return nil }
      raw := session.Logs()
      out := make([]artifacts.LogEntry, len(raw))
      for i, l := range raw {
          out[i] = artifacts.LogEntry{Level: l.Level, Message: l.Message, Ts: l.Ts}
      }
      return out
  }
  ```

- 把所有写 meta response artifact 的位置替换：
  - `failMetaResponse`（line 90）：`h.uploadMetaResponseArtifact(bgCtx, metaID, metaCreatedAt, statusCode, w.Header().Clone(), respBody, collectLogs())`
  - `failHook`（line 101）：同上替换。
  - 创建 session 失败那条 502 响应（line 150）：同上（此时 session 仍为 nil，logs 为 nil）。
  - 兜底 502（line 366）：同上。
  - `streamSuccess`：把 `metaCreatedAt` 那次 meta response 上传换成 `uploadMetaResponseArtifact`，需要把 `session` 通过参数传进函数（新增 `metaLogs []artifacts.LogEntry` 参数；调用 `streamSuccess` 处先 `metaLogs := collectLogs()` 再传入）。

  注意：`streamSuccess` 内部的 `uploadResponseArtifact(... upstreamID ...)` 调用保持不变（upstream 不带 logs）。

- 不需要改造 upstream artifact、request artifact 上传路径。

## 5. 前端组件

**文件**：`dashboard/src/components/LogsArtifactView.vue` （新建）

- props：`{ url?: string }`
- 内部 `payload`、`loading`、`error`，`watch(() => props.url, load, { immediate: true })`。
- `load()`：`fetch(props.url)` → JSON → 取 `data.logs ?? []`。404 → `error = 'artifact 不可用'`。
- 渲染：
  - `!url` → `<StateText compact>未启用 artifact 记录</StateText>`
  - `loading` → 加载中
  - `error` → 错误
  - `!logs.length` → `<StateText compact>无日志</StateText>`
  - 否则：列表，每条一行（不再用 DataTable，节省垂直空间）：

    ```html
    <div class="font-mono text-2xs flex flex-col gap-1">
      <div v-for="(l, i) in logs" :key="i" class="flex items-start gap-2 py-1 border-b border-line-soft last:border-0">
        <span class="inline-flex items-center px-1.5 py-0.5 rounded-[5px] uppercase text-2xs"
              :class="levelClass(l.level)">{{ l.level }}</span>
        <span class="text-ink-faint shrink-0 tabular-nums">{{ formatTs(l.ts) }}</span>
        <span class="text-ink whitespace-pre-wrap break-all">{{ l.message }}</span>
      </div>
    </div>
    ```

  - `levelClass`:
    - `info` → `bg-surface-50 text-ink-muted`
    - `warn` → `bg-warn-faint text-warn-ink`
    - `error` → `bg-err-faint text-err-ink`
  - `formatTs`：`new Date(iso).toLocaleTimeString(undefined, { hour12: false })` + `.SSS`（手动取毫秒补三位）。

**文件**：`dashboard/src/components/RequestDetailsPanel.vue`

- `DetailTab` 类型加 `'logs'`。
- 把 `detailTabs` 改成 `computed`：

  ```ts
  const isMeta = computed(() => !!selected.value && selected.value.id === selected.value.spanId)
  const detailTabs = computed(() => {
    const base = [
      { value: 'overview', label: '概览' },
      { value: 'request', label: '原始请求' },
      { value: 'response', label: '原始响应' },
    ]
    if (isMeta.value) base.push({ value: 'logs', label: '日志' })
    return base
  })
  ```

- import `LogsArtifactView`，在 `<Tabs>` 渲染分支末尾追加：

  ```html
  <LogsArtifactView
    v-else-if="detailTab === 'logs'"
    :url="selected.responseArtifactUrl"
  />
  ```

- 在 `watch(selectedId, ...)` 里继续把 tab 重置为 `'overview'`。新增保险：在 `watch(detailTabs, (tabs) => { if (!tabs.find(t => t.value === detailTab.value)) detailTab.value = 'overview' })`，覆盖切换 selected 后 tab 数量变化导致的 stale 状态。

## 6. 校验

- 后端：`go build ./...` 通过；手动测试：
  - 写一个启用脚本，在 `picotera.hooks.rewriteRequest.tap('t', (ctx, req) => { console.log('hello', ctx.currentRetryCount); console.warn('warn'); console.error('err'); return req })`。
  - 触发一次 gateway 请求，从 dashboard 拉取 meta response artifact 看 `logs` 字段。
- 前端：`pnpm --dir dashboard type-check`、`pnpm --dir dashboard build` 通过；UI 校验：
  - meta 请求详情显示「日志」tab，三条不同 level 染色正确。
  - upstream 请求详情不显示「日志」tab。
  - 切换 selected 后 tab 重置回「概览」。
- 边界：
  - 脚本不打日志 → tab 仍可见，显示「无日志」。
  - 触发 1001 条日志 → 第 1000 条之后是哨兵 `[picotera] log buffer truncated`。
  - artifact 关闭（`PICOTERA_S3_ENDPOINT` 为空）→ 显示「未启用 artifact 记录」。

## 7. 提交分块

1. `feat(jsx): buffer console output on session with hard caps`
2. `feat(artifacts): add Logs field and BuildResponseWithLogs`
3. `feat(gateway): write jsx logs into meta response artifact`
4. `feat(dashboard): show jsx logs in request details panel`
