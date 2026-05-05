# 执行计划

1. 安装 JSONEditor 依赖
   - 在 dashboard workspace 添加 `vanilla-jsoneditor`。
   - 确认 `dashboard/package.json` 与 `pnpm-lock.yaml` 更新。

2. 增加 JSON 查看组件
   - 新建 `dashboard/src/components/JsonArtifactViewer.vue`。
   - 参考 `.references/example.vue` 的 `createJSONEditor`、`updateProps`、`destroy` 生命周期用法。
   - 组件固定 tree 模式和 readonly。
   - 处理 mount、prop 更新、destroy 生命周期。
   - 使用 dashboard 现有 surface、line、radius、字体 token 包裹 editor。

3. 增加 artifact body 辅助函数
   - 新建或扩展 artifact 展示工具，提供 `contentTypeHeaderValue`、`isJsonContentType`、`parseJsonBody`。
   - header 名大小写不敏感。
   - `isJsonContentType` 先按 `;` 去除参数，使 `application/json; charset=utf-8` 命中 JSON。
   - Raw 展示返回原始 body 文本，不做 JSON pretty-print。

4. 改造请求 artifact body
   - 在 `RawArtifactView.vue` 的 request 分支增加 `Raw / JSON` 切换。
   - JSON content-type 且解析成功时默认显示 JSON tree。
   - 二进制 body 保持现有下载提示。
   - 非 JSON body 保持 Raw 代码块。

5. 改造响应 artifact body
   - 在 `ResponseArtifactView.vue` 中引入 JSON 查看组件。
   - 非 SSE JSON 响应增加 JSON 选项，默认显示 JSON。
   - Raw 选项显示原始文本。
   - 聚合视图改为 JSON tree 展示聚合结果。
   - 渲染视图保持现有 Markdown 内容展示。

6. 增加 SSE Events 数据导出
   - 从 `useSSEParser.ts` 导出 `ParsedSSEEvent` 类型和 `parseSSEEventsForDisplay(body)`。
   - 保持现有聚合、内容提取逻辑使用同一解析基础。
   - 每个 event 附带 index、event 名、原始 data、解析后的 json。

7. 增加 SSE Events UI
   - 在 SSE 响应的 `subViewOptions` 中加入 `Events`。
   - Events 视图展示 event 列表和当前 event 内容。
   - JSON event 使用 JSON tree。
   - 非 JSON event 使用代码块。
   - 切换 artifact 或 subview 时保证选中 event 不越界。

8. 验证
   - 运行 `pnpm --dir dashboard type-check`。
   - 运行 `pnpm --dir dashboard build`。
   - 修复类型或构建问题。
