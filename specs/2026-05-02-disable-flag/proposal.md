# Disable flag for routing targets

## 原始需求

给渠道增加 disabled 功能，允许禁用某个渠道，禁用后不参与调度。另外给 provider 的 model 也添加一个 disabled 字段，禁用后也是不参与调度。干脆给 model 也加上吧。界面也同步更新。

## 澄清

- "渠道" 在本项目中即 `provider`（dashboard 侧栏标签为「渠道」）。
- 三个层级都要支持 `disabled`：
  1. `provider`（渠道）
  2. `provider.provider_models[modelName]`（渠道下某个模型条目）
  3. `model`（全局模型）
- 路由调度（`GetProvidersByEndpointAndModel`）需要同时排除以上三处任意一处被禁用的目标。
- UI：
  - `model` 表的禁用**只影响调度**，列表正常列出，名称旁附「（已禁用）」文本提示。
  - `provider` 与 `provider_models` 的禁用项 UI 中正常列出，附状态提示。
  - 切换入口同时提供：列表行内开关按钮 + 编辑表单 / 面板内的复选框。
- 迁移：新列默认 `false`；JSONB 中缺失 `disabled` 键视为 `false`，存量数据无需改写。
