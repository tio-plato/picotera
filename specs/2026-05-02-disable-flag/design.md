# Design: Disable flag for routing targets

## 影响范围

`provider`、`model`、`provider.provider_models[*]` 三处新增 `disabled` 标记，唯一行为效果是：路由调度时被禁用项不再返回为候选。CRUD、列表、详情接口仍正常返回（含 `disabled` 字段），UI 显式标识被禁用项。

## 数据模型

### Schema 变化（新建迁移 `007_disable_flag.sql`）

- `provider`：`ADD COLUMN disabled BOOLEAN NOT NULL DEFAULT FALSE`
- `model`：`ADD COLUMN disabled BOOLEAN NOT NULL DEFAULT FALSE`
- `provider.provider_models` JSONB：在每个条目中新增可选键 `disabled?: boolean`。无 schema 变化，缺省值视为 `false`。Down 迁移仅 `DROP COLUMN`，不需要清洗 JSON。

### `ProviderModelEntry` JSON 形态

```ts
{
  upstreamModelName?: string
  endpoints?: string[]
  priority?: number
  annotations?: Record<string, string>
  disabled?: boolean   // 新增
}
```

## 路由筛选

`db/queries/routing.sql` 中的 `GetProvidersByEndpointAndModel` 需要同时排除三种禁用情形：

1. provider 层：`p.disabled = FALSE`
2. provider_models 条目层：`COALESCE((sub.pm ->> 'disabled')::boolean, false) = false`
3. model 层：`JOIN model AS m ON m.name = sqlc.arg('model_name')::text` 并加 `m.disabled = FALSE`

新增 JOIN model 后，输入的 `model_name` 必须存在于 `model` 表才能被路由——这与现状（`provider_models ? model_name` 已隐式假设 model 已注册）一致，但更显式。

## 后端 API 类型

- `ProviderView` / `CreateProviderRequest.Body` / `UpsertProviderRequest.Body`：新增 `Disabled bool \`json:"disabled"\``
- `ProviderModelEntry`：新增 `Disabled bool \`json:"disabled,omitempty"\``（保持 JSON 中可省略）
- `ModelView`：新增 `Disabled bool \`json:"disabled"\``

`To*View` / `From*View` 透传新字段。`UpdateProvider` sqlc 查询添加 `set_disabled` / `disabled` 入参；`UpsertModel` 添加 `disabled` 列；`CreateProvider` 多一个列。

## 前端

### 视觉规范

- 已禁用条目整体降级：`opacity-55`、`line-through` 不用，文字保持可读；额外渲染 `Badge variant="muted"`（或类似）显示「已禁用」/「(已禁用)」。
- 启用/禁用按钮使用 `IconButton`，图标 `eye-off` / `eye`（或 `power`），`title` 提示状态。
- 按钮位置：操作列首位，统一在编辑/删除之前。

### 受影响组件

| 文件 | 变更 |
|---|---|
| `dashboard/src/views/ProvidersView.vue` | 表头加「状态」列（或在名称下方加 Badge）；操作列加禁用切换按钮 |
| `dashboard/src/views/ModelsView.vue` | 名称右侧渲染「(已禁用)」灰色文本；操作列加禁用切换按钮 |
| `dashboard/src/components/ProviderForm.vue` | 表单加 disabled 复选框 |
| `dashboard/src/components/ModelForm.vue` | 表单加 disabled 复选框 |
| `dashboard/src/components/ProviderModelsPanel.vue` | 每个 model 条目卡片加禁用切换 + 编辑态复选框 |
| `dashboard/src/api.d.ts` | 由 `mise run openapi` 后的 `openapi-typescript` 重新生成 |

### UI 切换语义

- 列表行内按钮：直接 `PUT` 当前实体，仅切换 `disabled` 字段，其它字段从当前对象透传，无需打开表单。
- 表单复选框：与其它字段一同提交。
- 面板内 `provider_models` 条目：行内开关切换该条 entry 的 `disabled`，调用 `PUT /providers` 并传入完整 `providerModels` map。

## 不做的事

- 不引入级联禁用（禁用 provider 不会自动写入下属 entries）。路由查询天然处理：上层禁用即整体不可路由，无需 propagation。
- 不引入审计/历史记录。
- 不修改请求历史 (`request` 表) 行为。
