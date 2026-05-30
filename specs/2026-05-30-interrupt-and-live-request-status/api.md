# API 设计

所有操作位于 `/api/picotera` 组下。请求行 ID 即 `request` 表的 `id`（meta ID 或 upstream ID）。

## 1. 打断请求

```
POST /api/picotera/requests/{id}/interrupt
```

打断指定请求行对应的进行中处理。

- meta ID：取消整条链路（含当前 upstream 尝试），不再尝试任何 provider。
- upstream ID：取消该次尝试；headers 之前 → 走下一个 provider，headers 之后 → 直接中断流。

**Path 参数**

| 名称 | 类型 | 说明 |
| --- | --- | --- |
| `id` | string | 请求行 ID |

**请求体**：无

**响应体**

```json
{ "interrupted": true }
```

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `interrupted` | bool | `true`=找到进行中条目并已触发取消；`false`=该行已不在飞行中（无操作） |

被打断的请求行其 `finish_reason` 记为 `FinishReasonDashboardCancelled`（值 `7`），与客户端断开导致的 `Cancelled`（值 `2`）区分。meta 打断时级联取消的 upstream 行也会回落到该原因。

Huma 类型：

```go
type InterruptRequestRequest struct {
    ID string `path:"id"`
}
type InterruptRequestResponse struct {
    Body struct {
        Interrupted bool `json:"interrupted"`
    }
}
var OperationInterruptRequest = huma.Operation{
    OperationID: "interruptRequest",
    Method:      http.MethodPost,
    Path:        "/requests/{id}/interrupt",
    Summary:     "Interrupt an in-flight request (meta or upstream)",
}
```

## 2. 实时状态快照

```
GET /api/picotera/requests/{id}/live
```

返回该请求行在本进程内存中的实时进度。仅进行中的行有数据；已结束/不存在返回 `inFlight=false`。

**Path 参数**

| 名称 | 类型 | 说明 |
| --- | --- | --- |
| `id` | string | 请求行 ID |

**响应体**

```json
{
  "inFlight": true,
  "kind": "upstream",
  "phase": "streaming",
  "headersReceived": true,
  "statusCode": 200,
  "bytesReceived": 4096,
  "body": "data: {...}\n\n…",
  "startedAt": "2026-05-30T08:00:00Z",
  "lastChunkAt": "2026-05-30T08:00:03Z"
}
```

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `inFlight` | bool | 该行是否仍在本进程处理中 |
| `kind` | string | `meta` / `upstream`（`inFlight=false` 时为空） |
| `phase` | string | `pending`（未收到 headers）/ `headerReceived`（收到 headers 未出字节）/ `streaming`（已在出字节） |
| `headersReceived` | bool | 是否已收到上游 headers |
| `statusCode` | int | 上游响应状态码（收到 headers 后有值，否则 0） |
| `bytesReceived` | int | 至今收到/回传的字节数 |
| `body` | string | 响应体至今内容（完整，未截断；客户端可见字节） |
| `startedAt` | string (RFC3339) | 进度开始时间 |
| `lastChunkAt` | string (RFC3339) | 最近一次收到字节的时间 |

meta 行：在某次 upstream 尝试进入流式前 `phase=pending`、`body` 为空；之后镜像该 upstream 的进度。

Huma 类型：

```go
type GetRequestLiveRequest struct {
    ID string `path:"id"`
}
type RequestLiveView struct {
    InFlight        bool   `json:"inFlight"`
    Kind            string `json:"kind,omitempty"`
    Phase           string `json:"phase,omitempty"`
    HeadersReceived bool   `json:"headersReceived"`
    StatusCode      int    `json:"statusCode,omitempty"`
    BytesReceived   int64  `json:"bytesReceived"`
    Body            string `json:"body,omitempty"`
    StartedAt       string `json:"startedAt,omitempty"`
    LastChunkAt     string `json:"lastChunkAt,omitempty"`
}
type GetRequestLiveResponse struct {
    Body RequestLiveView
}
var OperationGetRequestLive = huma.Operation{
    OperationID: "getRequestLive",
    Method:      http.MethodGet,
    Path:        "/requests/{id}/live",
    Summary:     "Get in-memory live status of an in-flight request",
}
```
