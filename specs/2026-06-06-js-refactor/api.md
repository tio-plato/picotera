# API：JS ctx 与 hook 数据结构

本文件定义重构后 JS 侧可见的 `ctx` 结构、各 hook 的 waterfall 值（输入 = 输出），
以及对应的 Go 类型。所有结构均 JSON 可序列化（为将来 go-plugin 的 gRPC 传输预留）。

## 共享类型（ctx 各字段）

```go
// EndpointSummary —— 同现状，ctx.endpoint。
type EndpointSummary struct {
    Name                string `json:"name"`
    Path                string `json:"path"`
    ModelPath           string `json:"modelPath"`
    CredentialsResolver int32  `json:"credentialsResolver"`
    EndpointType        int32  `json:"endpointType"` // 格式枚举，注意区别于 ctx.endpointType
}

// ModelSummary —— ctx.routedModel。
type ModelSummary struct {
    Name        string            `json:"name"`
    Annotations map[string]string `json:"annotations"`
}

// RequestShape —— ctx.request（客户端请求）。
type RequestShape struct {
    Path     string              `json:"path"`
    Method   string              `json:"method"`
    Headers  map[string][]string `json:"headers"`
    Model    string              `json:"model"`
    PathVars map[string]string   `json:"pathVars,omitempty"`
    Body     json.RawMessage     `json:"body,omitempty"` // 仅 application/json 且可解析时存在
}

// ApiKeySummary —— ctx.apiKey。原始 key 不暴露。
type ApiKeySummary struct {
    ID          int32             `json:"id"`
    Name        string            `json:"name"`
    Annotations map[string]string `json:"annotations"`
    Disabled    bool              `json:"disabled"`
}

// ProviderSummary —— ctx.provider，以及 CandidateView.Provider。凭据不暴露。
type ProviderSummary struct {
    ID          int32             `json:"id"`
    Name        string            `json:"name"`
    Priority    int32             `json:"priority"`
    Annotations map[string]string `json:"annotations"`
    Disabled    bool              `json:"disabled"`
}

// ProviderModel —— ctx.providerModel，以及 CandidateView.ProviderModel。
// 当前候选解析后的“单 endpoint”模型配置（取代旧 CandidateMPE）。
type ProviderModel struct {
    Name              string            `json:"name"`              // 模型名（旧 modelName）
    UpstreamModelName string            `json:"upstreamModelName"`
    Endpoint          string            `json:"endpoint"`          // 已解析单 endpoint path（旧 endpointPath）
    Priority          int32             `json:"priority"`
    Annotations       map[string]string `json:"annotations"`
    UpstreamFormat    string            `json:"upstreamFormat"`    // 仅 unified 有意义
}

// AttemptState —— ctx.attempt，每次尝试更新。
type AttemptState struct {
    CurrentRetryCount int        `json:"currentRetryCount"`
    TotalAttemptCount int        `json:"totalAttemptCount"`
    LastError         *LastError `json:"lastError"` // 首次为 null
}

type LastError struct {
    ProviderID int    `json:"providerId"`
    StatusCode int    `json:"statusCode"`
    Message    string `json:"message"`
}
```

## ctx 形状

```jsonc
ctx = {
  "endpointType": "gateway",   // | "unified"，路由形态
  "endpoint":      EndpointSummary | null,
  "requestModel":  "",         // 原始请求模型名
  "routedModel":   ModelSummary | null,
  "request":       RequestShape | null,
  "apiKey":        ApiKeySummary | null,
  "provider":      ProviderSummary | null,
  "providerModel": ProviderModel | null,
  "attempt":       AttemptState | null,
  "annotations":   { },        // 预合并便利 map，逐阶段重算
  "stream":        false,      // 一次性
  "sourceFormat":  ""          // 一次性，unified 用
}
```

`PatchContext` 用到的 Go 侧补丁结构（指针字段，仅非 nil 的浅合并到 `globalThis.ctx`）：

```go
type ContextPatch struct {
    EndpointType  *string            `json:"endpointType,omitempty"`
    Endpoint      *EndpointSummary   `json:"endpoint,omitempty"`
    RequestModel  *string            `json:"requestModel,omitempty"`
    RoutedModel   *ModelSummary      `json:"routedModel,omitempty"`
    Request       *RequestShape      `json:"request,omitempty"`
    ApiKey        *ApiKeySummary     `json:"apiKey,omitempty"`
    Provider      *ProviderSummary   `json:"provider,omitempty"`
    ProviderModel *ProviderModel     `json:"providerModel,omitempty"`
    Attempt       *AttemptState      `json:"attempt,omitempty"`
    Annotations   *map[string]string `json:"annotations,omitempty"`
    Stream        *bool              `json:"stream,omitempty"`
    SourceFormat  *string            `json:"sourceFormat,omitempty"`
    UpstreamResponse json.RawMessage `json:"upstreamResponse,omitempty"` // 仅 rewriteProviderModels
}
```

---

## 各 hook 契约

每个 hook 是 waterfall：宿主传入 `initial` 值，依次过各 tap，tap 返回非 `undefined`
即替换；最终值回传宿主。下表「输入 = 输出」即该 waterfall 值的形状。

### 1. `rewriteModel`（一次性，路由前）

- ctx 已填充：`endpointType, endpoint, requestModel, request, apiKey, annotations(model+apiKey), stream, sourceFormat`。`routedModel/provider/providerModel/attempt = null`。
- waterfall 值（输入 = 输出）：`string`（模型名，初始 = 路由模型 = requestModel）。
- 语义：返回非 string 视为不改（保留原值）。无模型端点（如 count_tokens）上返回非空字符串 → 宿主报错。

```jsonc
// 输入 / 输出
"claude-3-5-sonnet"
```

### 2. `sortProviders`（一次性，路由后）

- ctx 已填充：上一阶段 + `routedModel`。`provider/providerModel/attempt = null`。
- waterfall 值（输入 = 输出）：`CandidateView[]`。

```go
type CandidateView struct {
    Provider      ProviderSummary   `json:"provider"`
    ProviderModel ProviderModel     `json:"providerModel"`
    Annotations   map[string]string `json:"annotations"` // 该候选的合并 map
}
```

```jsonc
// 输入 / 输出（重排 / 过滤后的同形列表）
[
  { "provider": {…}, "providerModel": {…}, "annotations": {…} },
  …
]
```

- 语义：直返数组，或返回 `{ "providers": CandidateView[] }`，两种都接受。空数组 = 无可用 provider（宿主返回 502）。passthrough（未改）= 保持原列表。

### 3. `beforeRequest`（每次尝试）

- ctx 已填充：`endpointType, endpoint, routedModel, request, apiKey, provider, providerModel, attempt, annotations(候选合并), stream, sourceFormat`。
- waterfall 值（输入 = 输出）：

```go
type BeforeRequestDecision struct {
    Next          bool   `json:"next"`          // true = 跳到下一个候选
    Delay         int    `json:"delay"`         // ms，Go 侧按 JSMaxDelay 截断
    UpstreamModel string `json:"upstreamModel"` // 非空 = 覆盖本次尝试的上游模型
}
```

```jsonc
// 输入（初始）：next = (attempt.currentRetryCount > 0)，delay = 0，upstreamModel = ""
// 输出：
{ "next": false, "delay": 0, "upstreamModel": "" }
```

- 语义：非 string 的 `upstreamModel` 在 JS 边界丢弃（= ""，保持默认）。

### 4. `rewriteRequest`（每次尝试）

- ctx 已填充：同 `beforeRequest`（客户端请求在 `ctx.request`，不再单独传 clientRequest）。
- waterfall 值（输入 = 输出）：

```go
type PendingRequestShape struct {
    URL     string              `json:"url"`
    Method  string              `json:"method"`
    Headers map[string][]string `json:"headers"`
    Body    json.RawMessage     `json:"body,omitempty"` // 仅 application/json 时存在
}
```

```jsonc
// 输入 / 输出（必须返回完整对象，不支持部分覆盖）
{ "url": "https://…", "method": "POST", "headers": {…}, "body": {…} }
```

- 语义：tap 若把 `body` 留成非字符串（对象 / 数组），SDK 在边界 `JSON.stringify`，Go 端始终收到 JSON 字符串 token。

### 5. `beforeTransform`（每次尝试，仅 unified）

- ctx 已填充：同 `beforeRequest`，且 `stream`、`sourceFormat`、`providerModel.upstreamFormat` 可读。
- waterfall 值（输入 = 输出）：

```go
type OutboundProfile struct {
    Type   string         `json:"type"`
    Config map[string]any `json:"config"`
}
```

```jsonc
// 输入（初始 = 该上游格式的默认 profile）/ 输出
{ "type": "openai", "config": {} }
```

- 语义（严格，因结果直接驱动 bridge 构造）：必须返回 object；含 `type` 时必须为 string，含 `config` 时必须为 object。

### 6. `rewriteProviderModels`（管理路由 fetch-models，独立于 meta request）

- ctx 已填充：`provider, annotations(provider), upstreamResponse`。`endpoint/routedModel/request/apiKey/providerModel/attempt = null`。
- waterfall 值（输入 = 输出）：`ProviderModelEntry[]`（**配置项**，`endpoints` 复数，区别于 `ProviderModel`）。

```go
type ProviderModelEntry struct {
    Model             string            `json:"model"`
    UpstreamModelName string            `json:"upstreamModelName,omitempty"`
    Endpoints         []string          `json:"endpoints,omitempty"`
    Priority          int32             `json:"priority,omitempty"`
    Annotations       map[string]string `json:"annotations,omitempty"`
    Disabled          bool              `json:"disabled,omitempty"`
}
```

```jsonc
// 输入（默认聚合后的列表）/ 输出
[
  { "model": "gpt-4o", "upstreamModelName": "gpt-4o", "endpoints": ["openaiChatCompletions"] },
  …
]
```

- 语义：返回非数组 / `undefined` → 保留输入。`ctx.upstreamResponse` 为上游 `/models` 原始 JSON。

---

## SDK（`sdk.js`）对外接口

```jsonc
globalThis.ctx          // 持久共享上下文（见上）
globalThis.picotera = {
  hooks: {              // 六个同步 Waterfall：rewriteModel / sortProviders /
                        // beforeRequest / rewriteRequest / beforeTransform /
                        // rewriteProviderModels
    <name>: { tap(name, fn, priority?) }   // fn(ctx, value) -> value | undefined
  },
  kv:    { get, set, setex, ttl, del },    // 同步
  fetch: function (url, init?) -> { status, headers, body }  // 同步（不再 Promise）
}
globalThis.console      // log/info/warn/error/debug
// 移除：基于 Promise 的 setTimeout
```
