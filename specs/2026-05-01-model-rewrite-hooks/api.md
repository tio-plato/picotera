# JS API：rewriteModel + beforeRequest（扩展）

## rewriteModel（新）

在 picotera 解析出客户端请求中的模型名之后、检索 model→provider 映射之前调用。脚本可以根据客户端请求的形态（路径、headers、body）改写 modelName，从而切换到不同的 MPE 路由。

### 注册

```js
picotera.hooks.rewriteModel.tap(name, fn, priority?)
```

### 回调签名

```js
function (ctx, modelName) → string | undefined
```

#### ctx 字段

| 字段 | 类型 | 说明 |
|---|---|---|
| `request` | ClientRequest | **只读**，客户端原始请求快照 |

`ClientRequest` 形状与 rewriteRequest 中一致：

```ts
type ClientRequest = {
  path: string,
  method: string,
  headers: Record<string, string[]>,   // key 全小写
  model: string,                        // 与第二个参数 modelName 同值（rewriteModel 的入参）
  body?: any,                           // 仅当 content-type 是 application/json 时存在
}
```

#### 返回语义

- 返回字符串 → 作为下一个 tap 的输入；最终值替换 modelName。
- 返回 undefined / 不 return → 沿用上一 tap 的值。
- 返回非字符串（数字、null、对象、数组）→ 视作"不改"。

#### 副作用

最终 modelName 与入参不同时：

1. picotera 用新 modelName 重新检索 MPE（`resolveProviders(endpoint.Path, newModelName)`），可能落到不同的 provider 集合。
2. 客户端请求 body 的 `model` 字段被同步改写为新值（sjson 替换），后续 hook 的 `ctx.request.body.model` 与 `ctx.request.model` 都是新值。

### 示例

```js
// 把所有 claude-3-haiku 请求路由到带版本号的 MPE
picotera.hooks.rewriteModel.tap('haiku-pin', (ctx, model) => {
  if (model === 'claude-3-haiku') return 'claude-3-haiku-20240307'
})

// 按客户端的 X-Tenant 切到企业版模型
picotera.hooks.rewriteModel.tap('tenant-route', (ctx, model) => {
  const tenant = ctx.request.headers['x-tenant']?.[0]
  if (tenant === 'enterprise' && model === 'gpt-4o') return 'gpt-4o-enterprise'
})
```

---

## beforeRequest（扩展返回值）

beforeRequest 的注册和 ctx 不变，仅在返回值里加一个可选字段 `upstreamModel`。

### 返回值新形状

```ts
type BeforeRequestDecision = {
  next?: boolean,           // 同现状：跳过当前候选
  delay?: number,           // 同现状：sleep 毫秒
  upstreamModel?: string,   // 新增：本次 attempt 写入 upstream body.model 的值
}
```

`upstreamModel` 解析规则：

- 缺省 / undefined / null / 空字符串 / 非字符串 → 沿用 `mpe.upstreamModelName`（非空时）或当前 modelName（兜底）。
- 非空字符串 → 替换为该值。

替换发生在 `buildUpstreamRequest` 之前；buildUpstreamRequest 通过 sjson 把这个值写入 upstream body 的 `model` 字段。再之后才会进入 rewriteRequest hook（rewriteRequest 看到的 `pendingRequest.body.model` 已是这个最终值）。

### 示例

```js
// 根据 provider 动态选择带日期的版本
picotera.hooks.beforeRequest.tap('versioned', (ctx) => {
  if (ctx.provider.name === 'openai-direct' && ctx.mpe.upstreamModelName === 'gpt-4o') {
    return { upstreamModel: 'gpt-4o-2024-08-06' }
  }
})

// 重试时切到 fallback 模型并加 200ms 延迟
picotera.hooks.beforeRequest.tap('retry-fallback', (ctx) => {
  if (ctx.currentRetryCount > 0 && ctx.request.model.startsWith('claude-3-5')) {
    return {
      delay: 200,
      upstreamModel: ctx.request.model.replace('claude-3-5', 'claude-3'),
    }
  }
})
```

---

## 整体调用顺序

```
extractModel
  ↓
rewriteModel                        ← 一次
  ↓
resolveProviders
  ↓
sortProviders                       ← 一次
  ↓
[per attempt]
  beforeRequest（可返回 upstreamModel）
  buildUpstreamRequest（写 body.model）
  rewriteRequest
  forwardRequest
```
