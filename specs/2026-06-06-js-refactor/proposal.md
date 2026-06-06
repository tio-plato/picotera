# 对 js 脚本功能的重构

## 第三方依赖

原本依赖的 qjs 模块，是走 wazero wasm 的，有些固有问题。我想换成使用 https://pkg.go.dev/modernc.org/quickjs

这个 modernc 的 quickjs 不支持异步，但是没关系，我们实际上也用不到异步 API，直接改回同步即可。

做的时候为 js 相关功能增加一套接口，然后实现它，方便我们后续可以改造为 goplugin 的插件形式。

## 明确生命周期

希望每个 meta request 共享一个上下文，请求结束后销毁。

meta request 开始时，为环境注入全局的 ctx 变量：创建一个新的 Value，在各个过程中修改它，并在请求结束后销毁。

## Context 结构

```json
{
  "endpointType": "gateway", // 或者 unified，路由形态（与 endpoint.endpointType 的格式枚举不同）
  "endpoint": { // 对应 EndpointSummary 结构
  },
  "requestModel": "", // 原始请求的模型名字
  "routedModel": { // 对应 ModelSummary 结构，这里是模型改写后、被路由的模型的数据体
  },
  "request": { // 对应 RequestShape（客户端请求）结构
  },
  "apiKey": {}, // 对应 ApiKeySummary
  "provider": {}, // 对应 ProviderSummary
  "providerModel": { // 当前候选解析之后、provider 的单个模型配置
    "name": "",
    "upstreamModelName": "",
    "endpoint": "", // 已解析到的单个 endpoint path
    "priority": 0,
    "annotations": {},
    "upstreamFormat": "" // 仅 unified 时有意义
  },
  "attempt": { // 每次尝试都在变的状态，每次重试前重写
    "currentRetryCount": 0,
    "totalAttemptCount": 0,
    "lastError": null // 对应 LastError，首次尝试为 null
  },
  "annotations": {}, // 预先合并好的 annotations 便利 map（model+provider+entry+apiKey，后者覆盖前者），随各层填充逐阶段重算
  "stream": false, // 是否流式，模型解析阶段一次性确定
  "sourceFormat": "" // 源格式字符串，一次性确定（unified 用）
}
```

在 hook 运行的时候，根据阶段，可能会有某些字段是 null，这个没关系。

这里面的每个字段，都是确定后，才写进去的。比如在 sortProvider 的时候，
provider/providerModel/attempt 这些都是空的。

在某个字段变了之后（比如因为重试轮换到下一个 provider，或者被 js 改写），重写 ctx 的 JS Value，
使之反应现状。脚本在 ctx 上挂的自定义字段，在整个 meta request 期间保留。

## Output 结构

不再显式区分 input 和 output。每个 hook 是一个 waterfall：传入的值就是要被改写并返回的值，
和 context 重复的部分移除（移到 ctx 里读）。

每个 hook 的输入 / 输出结构如下（详细类型见 `api.md`）：

| hook | 阶段 | 输入 = 输出（waterfall 值） |
| --- | --- | --- |
| `rewriteModel` | 一次性，路由前 | `string`（模型名） |
| `sortProviders` | 一次性，路由后 | `CandidateView[]`（`{provider, providerModel, annotations}` 列表） |
| `beforeRequest` | 每次尝试 | `{ next, delay, upstreamModel }` |
| `rewriteRequest` | 每次尝试 | `PendingRequestShape`（`{url, method, headers, body}`） |
| `beforeTransform` | 每次尝试，仅 unified | `OutboundProfile`（`{type, config}`） |
| `rewriteProviderModels` | 管理路由 fetch-models | `ProviderModelEntry[]`（配置项，endpoints 复数） |

各 hook 运行时 ctx 已填充的字段，以及每个 waterfall 值的字段语义，详见 `api.md`。
