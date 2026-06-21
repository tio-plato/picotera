# 执行计划

## 1. `dashboard/src/api/client.ts` —— 扩展 `postGatewayTest`

- 在 `body` 与 `signal` 之间增加形参 `headers?: Record<string, string>`。
- 合并 headers：`{ 'Content-Type': 'application/json', ...headers, Authorization: \`Bearer ${apiKey}\` }`。
- `postTestDirect` 不动。

## 2. `dashboard/src/views/TestView.vue` —— 控件与状态

- import `AnnotationsEditor`（`@/components/AnnotationsEditor.vue`）。
- 新增状态：
  - `const gatewayHeaders = ref<Record<string, string>>({})`
  - `const otrOverride = ref<'' | 'none' | 'body' | 'body-and-message'>('')`
- 新增 OTR 选项常量（标签对齐 `SettingsView.vue`）：
  ```ts
  const otrOptions = [
    { value: '', label: '跟随设置' },
    { value: 'none', label: '完整记录' },
    { value: 'body', label: '不记录内容' },
    { value: 'body-and-message', label: '不记录内容和梗概' },
  ]
  ```
- 在 `send()` 的网关分支组装 header 表：
  ```ts
  const headers: Record<string, string> = { ...gatewayHeaders.value }
  if (otrOverride.value) headers['X-PicoTera-OTR'] = otrOverride.value
  res = await postGatewayTest(substitutePath(targetPath), selectedApiKey.value!.key, body, headers, controller.signal)
  ```

## 3. `dashboard/src/views/TestView.vue` —— 模板

在网关模式表单块（`<template v-else>` 内，端点/路径选择之后、路径变量之前）增加：

- `Field` 包裹的 OTR `SegmentedControl`（`v-model="otrOverride"` `:options="otrOptions"`），附一行说明：跟随设置时使用用户默认的数据记录模式。
- `Field`（`as="div"`，label「自定义请求头」）包裹 `<AnnotationsEditor v-model="gatewayHeaders" />`。

两者均位于 `mode === 'gateway'` 的 `<template v-else>` 块内，短路测试不渲染。

## 4. 验证

- `pnpm --dir dashboard type-check`
- `pnpm --dir dashboard lint`
- 手动：网关测试下添加自定义 header 与各档 OTR，确认请求带上对应头；短路测试界面不出现这两个控件。
