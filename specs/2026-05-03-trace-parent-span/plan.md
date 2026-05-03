# Plan — Trace Parent Span

## 步骤

### 1. 新增 `extractParentSpanID` helper

文件：`pkg/server/gateway_helpers.go`

新增函数：

```go
func extractParentSpanID(h http.Header) string {
    if v := strings.TrimSpace(h.Get("X-Claude-Code-Session-Id")); v != "" {
        return v
    }
    for k, vs := range h {
        if !strings.EqualFold(k, "conversation_id") {
            continue
        }
        for _, v := range vs {
            if s := strings.TrimSpace(v); s != "" {
                return s
            }
        }
    }
    return ""
}
```

如果 `gateway_helpers.go` 还没 import `strings`，补上。

### 2. 在网关入口提取 parent span id

文件：`pkg/server/handle_gateway.go`

在 `ServeHTTP` 中，紧接 `metaReqHeader := r.Header.Clone()` 之后插入：

```go
parentSpanID := extractParentSpanID(metaReqHeader)
parentSpanIDPg := pgtype.Text{String: parentSpanID, Valid: parentSpanID != ""}
```

### 3. 把 parent span id 写入 meta 行

文件：`pkg/server/handle_gateway.go`（约 65 行）

把 meta 行 `InsertRequestParams` 中的：

```go
ParentSpanID:  pgtype.Text{Valid: false},
```

替换为：

```go
ParentSpanID:  parentSpanIDPg,
```

### 4. 把 parent span id 写入 upstream 行

文件：`pkg/server/handle_gateway.go`（约 335 行，重试循环内）

同样把 upstream 行 `InsertRequestParams` 中的：

```go
ParentSpanID:  pgtype.Text{Valid: false},
```

替换为：

```go
ParentSpanID:  parentSpanIDPg,
```

`parentSpanIDPg` 是 `ServeHTTP` 顶层局部变量，闭包能直接捕获。

### 5. 验证

- `go build ./...` 通过。
- 启动后端：`mise run server`（依赖 `docker compose up -d`）。
- 用 curl 模拟两种 header 命中：

  ```bash
  curl -H 'X-Claude-Code-Session-Id: sid-abc' ...   # 期望 parent_span_id = sid-abc
  curl -H 'conversation_id: conv-xyz' ...           # 期望 parent_span_id = conv-xyz
  curl -H 'X-Claude-Code-Session-Id: sid-abc' \
       -H 'conversation_id: conv-xyz' ...           # 期望 sid-abc 优先
  curl ...                                          # 期望 parent_span_id = NULL
  ```

- 在 dashboard 请求详情面板确认 "Parent Span" 字段渲染；同一 sid 下 meta 与所有 upstream 行均带该 parent。
- SQL 抽查：

  ```sql
  SELECT id, type, span_id, parent_span_id FROM request ORDER BY created_at DESC LIMIT 10;
  ```

## 不在范围内

- 不新增迁移、sqlc 查询、合同字段或 TS 类型。
- 不改 dashboard 渲染逻辑（已有 `v-if` 条件）。
- 不为 parent header 名提供运行时配置；如需扩展再单独立项。
