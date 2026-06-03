# 执行计划：对话渲染组件

全部为 `dashboard/` 前端改动，无后端 / OpenAPI 变更。

## 步骤 1 — 解析模块 `conversation.ts`

文件：`dashboard/src/composables/conversation.ts`

1. 定义归一化类型：`ConversationRole`、`ConversationPart`（含 `text` / `thinking` / `toolCall` / `toolResult` / `media`）、`ConversationMessage`，并 `export`。
2. 抽出防御式工具函数 `asRecord` / `asArray` / `nonEmpty`（与 `useSSEParser` 一致），以及 `parseMaybeJson(s)`（字符串尝试 `JSON.parse`，失败返回原字符串）。
3. 实现 `detectFormat(json, kind)`，返回 `'openaiChat' | 'openaiResponses' | 'anthropic' | 'gemini' | null`，规则见 design.md。
4. 实现四个请求解析器：`parseOpenAIChatRequest`、`parseOpenAIResponsesRequest`、`parseAnthropicRequest`、`parseGeminiRequest`，各返回 `ConversationMessage[]`。
5. 实现四个响应解析器：`parseOpenAIChatResponse`、`parseOpenAIResponsesResponse`、`parseAnthropicResponse`、`parseGeminiResponse`，各返回 `ConversationMessage[]`（通常一条 assistant 消息）。
6. 导出两个入口：
   - `parseRequestConversation(json): ConversationMessage[] | null`（检测格式后分派；不可识别返回 `null`）。
   - `parseResponseConversation(json, format?): ConversationMessage[] | null`（`format` 来自 `aggregated.format` 时直接用，否则检测）。
   - 注意 `aggregated.format` 用 `'openaiChatCompletions' | 'openaiResponses' | 'anthropicMessages' | 'geminiStreamGenerateContent'`，需映射到内部格式标识。

## 步骤 2 — 纯展示组件 `ConversationView.vue`

文件：`dashboard/src/components/ConversationView.vue`

1. props：`{ messages: ConversationMessage[] }`。
2. 顶层 `v-for` 遍历消息；每条消息渲染角色标签（用 `Tag` 或带色徽标区分 system/user/assistant/tool）与 `parts`。
3. `parts` 内 `v-for` 按 `kind` 分支：
   - `text`：`<div class="prose prose-sm max-w-none" v-html="renderMarkdown(part.text)" />`。
   - `thinking`：受控 `<details>`「思考过程」（默认收起），内部同 Markdown 渲染，样式参照 `ResponseArtifactView` 思考块。
   - `toolCall`：一行「图标 + 名称」，`<details>` 展开后用 `JsonArtifactViewer :value="part.input"`。
   - `toolResult`：一行工具名（缺省「工具结果」），`isError` 标红；`<details>` 展开 `JsonArtifactViewer :value="part.output"`。
   - `media`：灰色徽标显示 `[mediaType]`。
4. 复用 `renderMarkdown`（从 `@/composables/useSSEParser` 导入）与 `JsonArtifactViewer`。
5. 样式只用 Tailwind v4 工具类与既有设计令牌（`bg-surface-*`、`text-ink-*`、`border-line-*` 等），不引第三方 UI。

## 步骤 3 — 容器组件 `ConversationArtifactView.vue`

文件：`dashboard/src/components/ConversationArtifactView.vue`

1. props：`{ requestUrl?: string; responseUrl?: string }`。
2. `const reqQuery = useArtifact(() => props.requestUrl)`、`const resQuery = useArtifact(() => props.responseUrl)`。
3. computed `requestMessages`：从 `reqQuery.data` 取 payload，二进制或非 JSON content-type 返回 `null`；否则 `parseJsonBody` 成功后 `parseRequestConversation(json)`。
4. computed `responseMessages`：优先 `payload.aggregated`（传 `aggregated.body` + 映射后的 format 给 `parseResponseConversation`）；否则二进制 / 非 JSON 返回 `null`，JSON 则 `parseResponseConversation(json)`。
5. computed `merged`：`[...(requestMessages ?? []), ...(responseMessages ?? [])]`。
6. 加载态：任一 query `isLoading` 显示 `StateText` 加载中。
7. computed `unparsable`：两侧都为 `null` 且非加载中 → 显示兜底 `StateText`「无法解析为对话，请查看原始请求 / 原始响应」。
8. 否则渲染 `<ConversationView :messages="merged" />`。

## 步骤 4 — 接入请求详情页

1. `dashboard/src/composables/useRequestDetailUiState.ts`：`DetailTab` 联合类型加 `'conversation'`。
2. `dashboard/src/components/RequestDetailsContent.vue`：
   - import `ConversationArtifactView`。
   - `detailTabs` 在 `{ value: 'response', label: '原始响应' }` 之后插入 `{ value: 'conversation', label: '对话' }`（位于 `if (isMeta) base.push(logs)` 之前，对所有 span 显示）。
   - 模板在 `response` 分支之后、`logs` 分支之前加：
     ```vue
     <ConversationArtifactView
       v-else-if="detailTab === 'conversation'"
       :request-url="selected.requestArtifactUrl"
       :response-url="selected.responseArtifactUrl"
     />
     ```

## 步骤 5 — 验证

1. `pnpm --dir dashboard type-check`（vue-tsc 通过）。
2. `pnpm --dir dashboard lint`（oxlint + eslint 通过）。
3. `pnpm --dir dashboard build`（构建通过）。
4. 本地跑 dashboard，打开一个真实请求详情，逐一验证四种格式（OpenAI Chat / Responses、Anthropic、Gemini）的请求与响应能正确渲染成对话；验证工具调用折叠、思考过程折叠、Markdown 渲染；验证二进制 / 非 LLM 请求走兜底提示；验证 Tab 切换持久化正常。

## 不做的事

- 不改后端、不改 OpenAPI、不动 `aggregated` 的生成逻辑。
- 不引入 Vercel AI SDK 或其它第三方解析 / UI 库。
- 不做多模态原始数据（图片/音频/文件）的内联预览，仅占位徽标。
- 不加兼容层或旧路径分支。
