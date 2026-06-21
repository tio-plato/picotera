# 设计：测试页面自定义 Headers 与 OTR

纯前端改动，集中在 `dashboard/src/views/TestView.vue` 与 `dashboard/src/api/client.ts`。无后端、无 OpenAPI 改动：网关测试本就是发往真实网关的原始 `fetch`，自定义 header 直接生效；`X-PicoTera-OTR` 的合法性校验与 `X-PicoTera*` 前缀的上游清理均已在网关侧实现。

## 作用范围

两项控件**只在网关测试（`mode === 'gateway'`）下渲染并参与发送**。短路测试（direct）的表单、`postTestDirect` 调用、payload 结构均不变。

## 自定义 Headers

- 在 `TestView.vue` 新增 `gatewayHeaders = ref<Record<string, string>>({})`，用 `AnnotationsEditor` 双向绑定编辑。`AnnotationsEditor` 已产出 `Record<string, string>`，语义契合，直接复用。
- `postGatewayTest` 增加可选 `headers?: Record<string, string>` 形参，合并进 `fetch` 的 headers：

  ```
  headers: { 'Content-Type': 'application/json', ...headers, Authorization: `Bearer ${apiKey}` }
  ```

  合并次序：默认 `Content-Type` 在前（允许用户覆盖），用户自定义 header 居中，`Authorization` 置于末尾**强制覆盖**——网关测试以所选 API Key 鉴权，不允许用户头意外破坏鉴权。

## OTR 选项

- 用 `SegmentedControl` 呈现四档：`跟随设置`（值 `''`）/ `none` / `body` / `body-and-message`，与 `SettingsView.vue` 的标签文案对齐（完整记录 / 不记录内容 / 不记录内容和梗概）。
- `otrOverride = ref<'' | 'none' | 'body' | 'body-and-message'>('')`，默认 `''` 表示「跟随设置」，**不发送** `X-PicoTera-OTR`。
- 发送时组装最终 header 表：以 `gatewayHeaders` 为基底，若 `otrOverride` 非空则补上 `X-PicoTera-OTR: <otrOverride>`。OTR 控件生成的 header 优先于手敲的同名 header（先展开自定义头，再写入 OTR）。

## send() 集成

`send()` 中网关分支组装合并后的 header 表并传入 `postGatewayTest`。direct 分支不变。
