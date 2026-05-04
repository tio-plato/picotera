# Proposal: Unified 路由的 endpoint 去重 + 模型上游面板的合并列表

来自 `TODO.md` 的前两条，关联紧密、一起实现。

1. unified 请求时，对同一个 provider 的某个 model 条目，如果展开成了多个 endpoints，只应该选择其中一个。优先级：
   - 与客户端请求类型一致的 endpoint
   - Anthropic Messages 格式
   - OpenAI Chat Completions 格式
   - 按 `endpoint.path` 字典序

2. 在「模型 — 上游」面板里，增加一个**合并的上游列表**，与 unified 请求 chat completions 的逻辑保持一致，方便用户查看 unified 路由优先级。该列表只显示 provider 与 modelName / upstreamModelName，已禁用的 provider/entry 显示但置灰（与现有「按端点分组」视图一致）。
