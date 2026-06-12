# Proposal: JS 大 body 的 Proxy 化处理

## 原始需求

我想修改我们现在的 js 对 body 大请求的处理。我想将大对象比如 request 这种,用 Proxy 封装起来,给每个对象分配一个 key,在 js 里面,我们读 key 或者枚举的时候呢,proxy 到 golang 这边,来读它对应的 key 和 value 这些。包括写也是,直接 proxy 到 golang 侧,这样就不需要来回 JSON 序列化了,现在仅在 rewrite request 的 body 里,和 ctx.request 生效。然后 JSON 序列化这个逻辑还是正常走,只是识别到这种 Proxy 对象的时候,JSON 自动变成类似 `{__picotera_object: 1}` 这样的东西,golang 识别到之后,就可以用内部对象,直接替换。这是为了避免用户直接返回一个新对象,导致跟踪丢失的问题。

## 澄清与补充(用户确认)

1. **Proxy 范围**:仅 `ctx.request.body` 与 rewriteRequest 的 `pending.body` 为 Proxy;ctx.request 的其余字段(path/method/headers 等)照旧急切嵌入为普通 JS 对象。
2. **datamask**:JS 边界的 data-url 脱敏逻辑整体移除(Proxy 已按需传输,读不到的字段不跨边界);`pkg/datamask` 包本身保留,后续另有用途。
3. **Proxy 间直接赋值**(如 `body.a = body.b`):直接报错。用户如确需拷贝,显式使用 `JSON.parse(JSON.stringify(x))` 手动深拷贝。
