# API: Google credential resolvers

## `EndpointView.credentialsResolver`

字符串枚举扩展为：

```
generalApiKey | bearerToken | xApiKey | searchKey | googApiKey | unknown
```

整型存储映射（`pkg/contract/endpoint.go`）：

| 字符串          | int32 |
| --------------- | ----- |
| `unknown`       | 0     |
| `generalApiKey` | 1     |
| `bearerToken`   | 2     |
| `xApiKey`       | 3     |
| `searchKey`     | 4     |
| `googApiKey`    | 5     |

## 网关凭证投递

| resolver        | 上游凭证位置                                       |
| --------------- | -------------------------------------------------- |
| `generalApiKey` | 嗅探客户端凭证位置后单点回写；缺线索时三头兜底（Bearer + X-Api-Key + X-Goog-Api-Key） |
| `bearerToken`   | `Authorization: Bearer <creds>`                    |
| `xApiKey`       | `X-Api-Key: <creds>`                               |
| `searchKey`     | URL 查询参数 `key=<creds>`（Set 语义，覆盖已有）   |
| `googApiKey`    | `X-Goog-Api-Key: <creds>`                          |

## 客户端凭证识别

`validateClientAuth(r, resolver)` 按 endpoint 的 `credentialsResolver` 决定可接受位置：

| resolver        | 接受位置                                                              |
| --------------- | --------------------------------------------------------------------- |
| `generalApiKey` | `Authorization: Bearer` / `X-Api-Key` / `?key=` / `X-Goog-Api-Key` 任一 |
| `bearerToken`   | 仅 `Authorization: Bearer`                                            |
| `xApiKey`       | 仅 `X-Api-Key`                                                        |
| `searchKey`     | 仅 URL 查询参数 `key`                                                 |
| `googApiKey`    | 仅 `X-Goog-Api-Key`                                                   |
| 其它（含 `unknown`） | 同 `generalApiKey`                                               |

未命中返回 `401` + `errorx.Unauthorized`。

## 上游请求剥离的客户端头

`buildUpstreamRequest` 不复制以下请求头到上游：

- `Authorization`
- `X-Api-Key`
- `X-Goog-Api-Key`（新增）
- `Host`
- `Content-Length`

客户端的 URL 查询参数不会被复制到上游，因为上游 URL 来源于 provider 配置（仅 `{name}` 占位替换）。
