# 凭证解析重构

我希望把网关凭证功能改成这样：

- **读取凭证时**：优先读取指定类型的凭证，如果读不到，则自动兼容其它所有支持的凭证。
- **发送凭证时**：仅根据 endpoint 的配置发送凭证。
- 在往上游发送请求的时候，除了抹去所有的凭证相关 headers，也自动抹去凭证相关的 search query（其它 search query 照样拼接转发）。
- 在 provider 绑定 endpoint 的时候，也支持指定凭证类型，如果指定，那么这会覆盖默认发送凭证的类型。

## 澄清结论

1. 当 endpoint resolver 是 `generalApiKey` / `unknown` 时，发送给上游仍按当前实现：优先匹配客户端使用的位置；缺乏线索时同时写三个 header（Authorization / X-Api-Key / X-Goog-Api-Key）。
2. 抹去的凭证 search query 只限 `key=` 一个键。
3. 非凭证类客户端 query 参数应转发给上游：与 upstream URL 自带的 query 合并，冲突时 upstream URL 一侧胜出。
4. `provider_endpoint` 上的 `credentialsResolver` 覆盖只影响 **发送** 方向；**读取** 方向永远使用 `endpoint.credentialsResolver`。
