# JSON 查看器与 SSE Events 视图设计

## 范围

本次改动只涉及 dashboard 请求详情里的 artifact 展示：

- 原始请求 body。
- 原始响应 body 的 Raw / 聚合 / 渲染视图。
- SSE 响应 body 的新增 Events 视图。

后端 artifact 结构和 API 不变。

## JSON 查看组件

新增 `dashboard/src/components/JsonArtifactViewer.vue` 作为只读 JSON tree 查看器。组件接收已经解析好的 JSON 值，使用 `vanilla-jsoneditor` 创建 editor 实例，并固定为：

- `mode: 'tree'`
- `readOnly: true`
- 不展示可编辑入口

用户要求的库是 `https://github.com/josdejong/svelte-jsoneditor`。该仓库同时发布框架无关的 `vanilla-jsoneditor`，Vue 项目直接集成 `vanilla-jsoneditor`，避免在 Vue 应用中引入 Svelte 组件运行时。实现仍使用同一作者、同一 JSONEditor 项目的只读 tree 能力。

集成方式参考仓库内 `.references/example.vue`：组件挂载时调用 `createJSONEditor({ target, props })`，输入 JSON 或选项变化时调用 `editor.updateProps(updatedProps)`，卸载时调用 `editor.destroy()`。实现使用 Vue 3 `<script setup lang="ts">` 封装该生命周期，不直接复用示例里的 Options API 和调试日志。

组件负责生命周期管理：挂载时创建 editor，输入 JSON 变化时更新内容，卸载时销毁实例。样式包裹在本地容器内，容器使用现有 surface、line、radius token，并限制最大高度以匹配当前 `<pre>` body 展示。

## JSON 类型探测

新增 artifact body 展示辅助逻辑：

- `isJsonContentType(headers)` 根据响应/请求 headers 的 `content-type` 严格探测 JSON 类型。
- 探测时先按 `;` 分离 media type，因此 `application/json; charset=utf-8` 识别为 JSON。
- 命中条件包括 media type 为 `application/json` 和以 `+json` 结尾的类型，例如 `application/problem+json`。
- header 名大小写不敏感，header 值按原值读取，不做宽松猜测。
- 只有 `bodyEncoding !== 'base64'` 且 content-type 命中 JSON 时才尝试 `JSON.parse(body)`。
- `JSON.parse` 失败时显示明确的 JSON 解析失败状态，并保留 Raw 视图可查看原始文本。

请求和响应 Raw body 不再自动 pretty-print JSON。Raw 始终显示后端记录的原始文本；JSON 选项显示 tree 查看器。

## 视图切换

请求原始 body：

- 非二进制且 content-type 为 JSON 时显示 `Raw / JSON` 切换。
- 默认显示 JSON tree；用户可切换 Raw 查看原文。
- 非 JSON 或 JSON 解析失败时只显示 Raw，解析失败时给出状态提示。

响应 body：

- Raw 始终存在，显示原始文本。
- 非 SSE JSON 响应显示 `Raw / JSON / 渲染`，默认显示 JSON。
- SSE 响应保持现有 `Raw / 聚合 / 渲染`，并新增 `Events`。
- SSE 聚合成功且聚合结果为 JSON 时，聚合视图使用 JSON tree；聚合失败显示现有失败状态。
- `渲染` 继续用于 LLM 内容 Markdown 展示。

## SSE Events 视图

`useSSEParser.ts` 导出 SSE event 解析结果，供 UI 展示每个 event：

```ts
export interface ParsedSSEEvent {
  index: number
  event: string | null
  data: string
  json: unknown | null
}
```

解析规则沿用当前 SSE 聚合逻辑：

- 以空行分隔 event。
- 读取 `event:` 字段作为事件类型。
- 合并多行 `data:` 字段。
- 跳过 `data: [DONE]`。
- 每个 event 的 `data` 尝试 `JSON.parse`；成功则填充 `json`，失败则保持 `json: null`。

Events 视图使用左侧 event 列表和右侧内容区：

- 列表展示序号、event 类型、内容类型。
- 点击 event 后在右侧展示内容。
- `json !== null` 时使用 JSON tree 查看器。
- 非 JSON event 使用现有代码块样式显示 `data`。
- 没有可解析 event 时显示 `StateText`。

## 依赖

在 `dashboard/package.json` 增加 `vanilla-jsoneditor` 依赖，并通过 pnpm 更新锁文件。该依赖来自用户指定的 JSONEditor 项目，提供 Vue 可直接挂载的框架无关 editor。

## 验证

实现后运行：

- `pnpm --dir dashboard type-check`
- `pnpm --dir dashboard build`

如果依赖安装改变锁文件，再检查 `pnpm-lock.yaml` 中只包含预期的依赖更新。
