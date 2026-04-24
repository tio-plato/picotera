# Dashboard Tailwind 化与 UI 组件库

## 原始需求

看下前端，现在都是手写 css，我想改成 tailwind css 的，然后该复用的组件都抽出来，最好形成一套可复用的组件库，设计上要和现在的对齐，然后更规范化一点，比如边距需要用固定的 design tokens 这样，颜色可以定义一套 tailwind 的 theme 以配合现在的颜色。

## 关键决策（已确认）

- **组件库位置**：内部组件目录 `src/ui/`，不单独拆 monorepo 包。
- **Design tokens**：沿用 Tailwind 默认 scale（0.25rem 基准），只在 `@theme` 内小幅补充缺失档位（如 `3xs`/半档圆角等）。现有手写 CSS 里的非标数值（0.4375rem, 0.6875rem 等）统一四舍五入到最近的 Tailwind 档位。
- **颜色 theme**：保留现有 `:root[data-theme="…"]` 的 CSS 变量系统（light / dark / solarized-light / solarized-dark 四套），通过 `@theme` 把它们映射为 Tailwind 的 color tokens（`bg-surface-0`, `text-ink`, `bg-accent` 等），使 Tailwind utility 自动随主题切换。
- **迁移策略**：一次性大改，单 PR 内完成 tokens 定义 → 组件库抽取 → 视图替换。
