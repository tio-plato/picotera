# API 设计

本改造不新增 HTTP 操作，仅扩展既有契约与新增一个全局设置键。

## ProjectView 扩展

`pkg/contract/project.go` 的 `ProjectView` 增加字段：

```go
type ProjectView struct {
    // ...既有字段...
    AutoCreated bool `json:"autoCreated"`
}
```

`ToProjectView` 把 `db.Project.AutoCreated` 映射到该字段。`listProjects` / `getProject` / `upsertProject`
响应据此带出 `autoCreated`。

## 全局设置键：`project.autoCreate`

复用既有全局设置 CRUD（`GET/PUT /settings`、`GET/DELETE /settings/{key}`），不新增端点。

- key：`project.autoCreate`
- value：JSON 布尔（`true` / `false`）
- 缺省（键不存在）视为 `false`，即关闭自动创建。

仪表盘 `SettingsView.vue` 通过 `getGlobalSetting('project.autoCreate')` 读取、`upsertGlobalSetting`
写入，与 `app.title` 同模式。

## 内部 SQL（非 HTTP API）

- `MatchProjectByPaths(candidate_paths text[]) :one` → 返回命中的最长项目 `id`。
- `InsertAutoCreatedProject(name, paths) :one` → 插入 `auto_created = true` 的项目。
- 删除 `ListProjectPaths`。
