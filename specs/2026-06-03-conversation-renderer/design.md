# 设计：对话渲染组件

纯前端改动，无后端 / OpenAPI 变更。所有文件位于 `dashboard/`。

## 总览

1. 新增一个**纯展示组件** `ConversationView.vue`，输入一个归一化的消息数组 `ConversationMessage[]`，渲染成对话气泡流。
2. 新增**解析模块** `conversation.ts`，把存储的请求体 JSON 与响应体（聚合结果或原始 JSON）解析成 `ConversationMessage[]`。支持 OpenAI Chat Completions、OpenAI Responses、Anthropic Messages、Gemini GenerateContent。
3. 新增**容器组件** `ConversationArtifactView.vue`，加载请求与响应两个 artifact，调用解析模块，合并为单条对话流，交给 `ConversationView.vue` 渲染。无法解析时显示兜底提示。
4. 在 `RequestDetailsContent.vue` 的 Tab 列表中、「原始响应」之后插入「对话」Tab，并扩展 `useRequestDetailUiState` 的 `DetailTab` 类型，使其自动继承既有的 Tab 持久化能力。

## 归一化数据模型

定义于 `conversation.ts`，作为解析器输出与组件输入的契约：

```ts
export type ConversationRole = 'system' | 'user' | 'assistant' | 'tool'

export type ConversationPart =
  | { kind: 'text'; text: string }
  | { kind: 'thinking'; text: string }
  | { kind: 'toolCall'; id: string | null; name: string; input: unknown }
  | { kind: 'toolResult'; id: string | null; name: string | null; output: unknown; isError: boolean }
  | { kind: 'media'; mediaType: string; label: string }

export interface ConversationMessage {
  role: ConversationRole
  parts: ConversationPart[]
}
```

- `parts` 保持源 payload 中的出现顺序（思考、文本、工具调用可交错）。
- `text` / `thinking` 内容按 Markdown 渲染。
- `toolCall` 仅显示 `name`，展开查看 `input`（JSON）。
- `toolResult` 显示工具名（若有），展开查看 `output`（JSON），`isError` 标红。
- `media` 用于图片 / 音频 / 文件等非文本内容，只显示一个占位徽标（如 `[image]`），不渲染原始数据。

## 格式检测

请求体的源格式在前端不由后端字段可靠下发，因此用**结构特征**检测；响应体优先采用后端已提供的 `aggregated.format`，无聚合结果时同样按结构检测。

`detectFormat(json, kind)`（`kind: 'request' | 'response'`）按以下顺序判定：

| 特征 | 格式 |
| --- | --- |
| 顶层有 `contents`（请求）或 `candidates`（响应） | Gemini |
| 顶层有 `input`（请求）或 `object === 'response'` / `output` 数组（响应） | OpenAI Responses |
| 顶层有 `choices`（响应） | OpenAI Chat |
| 顶层有 `messages` 且出现 `system`/`tool`/`developer` 角色或消息带 `tool_calls`（请求） | OpenAI Chat |
| 顶层有 `messages` 且有顶层 `system` 字段，或 content 块含 `type: tool_use/tool_result/thinking`（请求） | Anthropic |
| 响应有 `content` 数组且 `type === 'message'` / `role === 'assistant'` | Anthropic |

检测不出已知格式时返回 `null`，由容器走兜底分支。

## 各格式解析规则

字段对照来源：`pkg/llmbridgeimpl` 与 `third_party/axonhub/llm/transformer/*/model.go`。

### OpenAI Chat Completions

- 请求 `messages[]`：`role` 直接映射（`system`/`user`/`assistant`/`tool`/`developer→system`）。`content` 为字符串 → 一个 `text`；为数组 → 逐项映射（`text` → text，`image_url`/其它 → media）。`message.reasoning_content` / `message.reasoning` → `thinking`。`message.tool_calls[]` → `toolCall`（`id`、`function.name`、`function.arguments` 解析为 JSON）。`role: 'tool'` 消息 → `toolResult`（`tool_call_id` → id，`content` → output）。
- 响应：`choices[0].message`，同上规则提取 text / thinking / toolCall，组成一条 `assistant` 消息。

### OpenAI Responses

- 请求 `input`：字符串 → 单条 `user` 文本；数组 → 逐 `Item` 映射：`type: 'message'` 按 `role` + `content[].text`；`function_call` → `toolCall`（`call_id`、`name`、`arguments`）；`function_call_output` → `toolResult`。顶层 `instructions` → 一条 `system` 消息（置于最前）。
- 响应 `output[]`：`type: 'message'` → assistant 文本；`type: 'reasoning'` → `thinking`（取 `summary[].text`）；`type: 'function_call'` → `toolCall`。

### Anthropic Messages

- 请求顶层 `system`（字符串或块数组）→ 一条 `system` 消息。`messages[]`：`role` 为 `user`/`assistant`；`content` 为字符串 → text；为块数组逐块映射：`text` → text，`thinking` → thinking，`tool_use` → toolCall（`id`、`name`、`input`），`tool_result` → toolResult（`tool_use_id` → id，`content` → output，`is_error`），`image` → media。
- 响应 `content[]`：同块映射规则，组成一条 `assistant` 消息。

### Gemini GenerateContent

- 请求顶层 `systemInstruction`（`Content`）→ 一条 `system` 消息。`contents[]`：`role` 为 `user`/`model→assistant`；`parts[]` 逐项：`thought === true` 的 `text` → thinking，普通 `text` → text，`functionCall` → toolCall（`name`、`args`），`functionResponse` → toolResult（`name`、`response`），`inlineData`/`fileData` → media。
- 响应 `candidates[0].content.parts[]`：同 parts 映射规则，组成一条 `assistant` 消息。

## 组件设计

### `ConversationView.vue`（纯展示）

- props：`{ messages: ConversationMessage[] }`。
- 按消息顺序渲染。每条消息一个块，左侧/顶部标注角色（system/user/assistant/tool），角色用既有设计令牌着色区分（参考 `StateText`、`Tag` 的色板）。
- `text` / `thinking` 经 `renderMarkdown`（复用 `useSSEParser` 的 `renderMarkdown`，已含 DOMPurify）渲染为 HTML，套 `prose prose-sm`（与 `ResponseArtifactView` 的渲染视图一致）。
- `thinking` 用受控 `<details>`「思考过程」折叠，默认收起，样式参照 `ResponseArtifactView` 现有思考块。
- `toolCall`：一行显示工具图标 + 名称；点开 `<details>` 用 `JsonArtifactViewer` 显示 `input`。
- `toolResult`：一行显示工具名（缺省「工具结果」）；`isError` 时标红；点开用 `JsonArtifactViewer` 显示 `output`。
- `media`：显示一个灰色徽标 `[mediaType]`。

### `ConversationArtifactView.vue`（容器）

- props：`{ requestUrl?: string; responseUrl?: string }`。
- 用 `useArtifact(() => requestUrl)` 与 `useArtifact(() => responseUrl)` 分别加载两个 artifact（vue-query 按 url 缓存，互不干扰）。
- 请求侧：取 `requestPayload.body`，二进制（`bodyEncoding==='base64'`）或非 JSON 直接判为不可解析；否则 `parseJsonBody` → `detectFormat(json,'request')` → 对应解析器得到输入消息数组。
- 响应侧：优先 `responsePayload.aggregated`（用其 `format` + `body`）；否则解析 `responsePayload.body` JSON → `detectFormat(json,'response')`。得到 assistant 输出消息追加到数组末尾。
- 两侧都无法解析 → 显示兜底 `StateText`：「无法解析为对话，请查看原始请求 / 原始响应」。一侧可解析、另一侧不可解析时，仅渲染可解析的部分（例如请求未存或响应二进制）。
- 把合并后的 `ConversationMessage[]` 传给 `ConversationView.vue`。

### 接入 `RequestDetailsContent.vue`

- `detailTabs` 数组在「原始响应」后、「日志」前插入 `{ value: 'conversation', label: '对话' }`。「对话」对 meta 与 upstream 均显示。
- 模板新增分支：`v-else-if="detailTab === 'conversation'"` 渲染 `<ConversationArtifactView :request-url="selected.requestArtifactUrl" :response-url="selected.responseArtifactUrl" />`。
- `useRequestDetailUiState.ts`：`DetailTab` 增加 `'conversation'`。既有的模块级 ref 持久化与 `watch(detailTabs)` 防御回退自动覆盖新 Tab。

## 解析健壮性

- 所有解析器对缺失 / 异常字段宽松处理（沿用 `useSSEParser` 中 `asRecord`/`asArray`/`nonEmpty` 的防御式写法），缺字段则跳过该 part，绝不抛错——这是对**存储数据**的只读展示，不是对用户输入的校验，因此不适用「严格拒绝输入」规则。
- 工具调用 `arguments` 为字符串时尝试 `JSON.parse`，失败则原样作为字符串展示。

## 复用与新增清单

复用：
- `useArtifact`（加载 artifact）
- `renderMarkdown`（`useSSEParser`，Markdown + DOMPurify）
- `JsonArtifactViewer.vue`（工具调用 / 结果的 JSON 展开）
- `parseJsonBody` / `isJsonContentType`（`artifactBody.ts`）
- UI 基元 `StateText` / `Tag` / `Icon` / `Field`

新增：
- `dashboard/src/composables/conversation.ts`（类型 + 解析器，纯函数）
- `dashboard/src/components/ConversationView.vue`（纯展示组件）
- `dashboard/src/components/ConversationArtifactView.vue`（容器）

修改：
- `dashboard/src/components/RequestDetailsContent.vue`（加 Tab + 分支）
- `dashboard/src/composables/useRequestDetailUiState.ts`（`DetailTab` 加 `'conversation'`）
