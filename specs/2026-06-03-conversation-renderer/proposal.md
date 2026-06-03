# 对话渲染组件

## 原始需求

增加一个渲染对话的组件。支持渲染：用户消息、系统消息、助手消息、工具调用。其中工具调用只需要显示名字，展开查看 JSON 即可；其它内容需要支持 Markdown。另外助手消息需要支持渲染思考过程。

然后，在请求详情页面，在原始请求 / 原始响应 / 日志之后增加一个 Tab，叫「对话」，用这个组件渲染请求和响应。需要支持 OpenAI、Anthropic、Gemini 等格式。

可以用子 Agent 研究是否可以使用 Vercel AI SDK 的代码来做解析，不能的话就自己写。

## 调研结论：自己写解析器

经调研，Vercel AI SDK（`ai` / `@ai-sdk/*`）不适用：

- 它只做「统一格式 → provider 出站格式」的转换，且不导出可单独调用的解析函数；不存在「provider 原生 payload → 统一消息结构」的入站解析能力。
- 引入四种 provider 包体积超过 9MB，且仍得不到所需功能。

因此自行编写解析器，字段定义参照仓库已有的 `pkg/llmbridge` / `pkg/llmbridgeimpl`（基于 `axonhub/llm`）与各 provider 官方格式。

## 已确认的设计决策

- **布局**：合并为单条对话流——把请求中的 system/user/assistant/tool 历史消息，加上响应中模型的最终回复，按顺序拼成一条连续对话渲染。
- **兜底**：「对话」Tab 始终存在；当请求/响应不是已知 LLM 格式、解析失败或为二进制时，显示「无法解析为对话，请查看原始请求/响应」的提示。

## 支持的格式

- OpenAI Chat Completions
- OpenAI Responses
- Anthropic Messages
- Google Gemini GenerateContent（含 stream 变体）
