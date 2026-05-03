# Google credential resolvers

## 原始需求

鉴权解析，通用解析模式增加 `?key=` 和 `x-goog-api-key` 这两种（query 和 header）；同时增加两种凭证解析类型 一个叫 searchKey 一个叫 googApiKey。

## 澄清

- `validateClientAuth` 与 endpoint 的 `credentialsResolver` 绑定：
  - `generalApiKey`：`Authorization: Bearer`、`X-Api-Key`、`?key=`、`X-Goog-Api-Key` 任一存在即合法。
  - `bearerToken` / `xApiKey` / `searchKey` / `googApiKey`：仅接受对应位置的客户端凭证。
- `searchKey` 解析器固定使用查询参数名 `key`，不暴露为可配置项。
- `generalApiKey` 在客户端使用 `?key=` 时，转发到上游的 URL 中以"覆盖/设置"语义写入 `key=<provider creds>`。
- `generalApiKey` 缺线索（如 fetch-models 流程或客户端未带任何已识别凭证）时，上游同时写入三种 header：`Authorization: Bearer <creds>`、`X-Api-Key: <creds>`、`X-Goog-Api-Key: <creds>`；不主动写 `?key=` 查询参数。
