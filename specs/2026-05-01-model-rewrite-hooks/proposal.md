# 模型重写 hooks

## 需求

1. 增加一个 JS hook 点，在请求解析出模型之后调用。传入的上下文只包括客户端原始请求，输入是模型名，输出是新的模型名。如果 JS 输出了不同于老模型名的新模型名，则按新的模型名去检索映射关系（MPE）。
2. 在重写请求体之前，允许 JS 改写最终发往 upstream 的模型名——直接合并进 `beforeRequest`，让它的返回值多带一个改写后的模型名字段，不再单独加 hook。

## 决策约束

- **rewriteModel（新 hook）**：在 retry loop 之外、`extractModel` 之后、`resolveProviders` 之前，一次性调用。ctx 仅含 `request`（客户端原始请求快照），第二个参数是当前 modelName。返回字符串作为新 modelName。
- **beforeRequest（扩展现有 hook）**：返回值由原 `{next, delay}` 扩展为 `{next, delay, upstreamModel}`。`upstreamModel` 非空字符串时替换当前候选要写入 upstream body 的 model 字段。
- **rewriteModel 同步更新 body**：modelName 改写后，body 里的 `model` 字段同步用 sjson 改成新值，后续 hook（包括 rewriteRequest 看到的 clientRequest）以及 upstream 请求保持一致。
- **下游 hook 看到新 modelName**：sortProviders / beforeRequest / rewriteRequest 的 `ctx.request.model` 都是 rewriteModel 之后的最终值。
