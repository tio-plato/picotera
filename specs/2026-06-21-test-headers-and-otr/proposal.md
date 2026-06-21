# 测试页面：自定义 Headers 与 OTR 选项

给测试页面（`TestView.vue`）增加两项能力，**仅作用于网关测试，短路测试不改**：

1. **自定义请求头编辑** —— 复用 `AnnotationsEditor`（key/value 编辑器，含交互/批量/JSON 三种模式），让用户在发起网关测试时附带任意自定义请求头。
2. **OTR 选项** —— 增加一个数据记录模式选择控件，按所选值注入 `X-PicoTera-OTR` 请求头（取值 `none` / `body` / `body-and-message`，与用户设置共用同一套值）。默认「跟随设置」时不发送该 header。

OTR 选项作为独立控件呈现（而非让用户手动敲 header）。后端 OTR header 校验与 `X-PicoTera` 前缀清理已存在，本次无需后端改动。
