# Proposal: 控制台偏好设置菜单

在控制台前端左下角，增加一个设置按钮，点击可以打开一个菜单。菜单里可以切换主题、弹窗样式、边距。主题有亮色、Solarized、暗色三种；弹窗样式有自动（现在这样）、固定在右侧打开、固定弹窗；边距有宽、适中（现在）、窄。"已连接"去掉。

## 澄清后的细节

- Solarized 主题拆成 Solarized Light 与 Solarized Dark 两项，主题共 4 个：亮色、Solarized Light、Solarized Dark、暗色。
- 弹窗样式「固定弹窗」= 居中 Modal 遮罩。
- 边距「宽/适中/窄」= 全局密度，影响主内容 padding、表格行高与单元格、表单字段、侧边栏、卡片等，通过 CSS 变量整体缩放。
- 设置菜单使用 PrimeVue 的 `Menu` 组件（popup 模式，见 https://primevue.org/llms/components/menu.md）。
