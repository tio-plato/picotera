# 设计

## 概述

把项目路径匹配从「内存前缀缓存」改成「在 Go 侧生成祖先候选路径数组、用一条 SQL 让数据库匹配」，
并新增受全局开关控制的项目自动创建能力。匹配与自动创建都在请求热路径上同步完成（`insertMetaRequest`），
保证匹配到/新建的 `project_id` 能写入本次请求的 meta 行。

路径匹配与项目自动创建都收敛在 `pkg/server/project_extractor.go` 中。`projectRouter`（内存缓存）整体删除。

## 匹配语义变化

旧逻辑用 `strings.HasPrefix(候选, 项目路径)` 做字符级前缀匹配（可能在非路径分隔边界误命中，例如项目路径
`/path/to/fo` 会错误命中请求路径 `/path/to/foo`）。

新逻辑改为：在 Go 侧把请求路径展开成「自身 + 各级祖先目录」的候选数组，再让数据库找出**等于**某个候选项的
项目路径。等价于「项目路径必须是请求路径本身或其按分隔符对齐的祖先目录」，命中最长（最具体）者。这同时修掉了
旧逻辑的非边界误命中问题。

## 候选祖先路径生成

对每个 JSON 解码后的真实路径，生成候选数组：

- 先把路径本身加入。
- 循环：在当前串中找最后一个 `/` 或 `\` 的位置 `i`：
  - `i < 0`：停止。
  - `i == 0`：把根分隔符（首字符，`/` 或 `\`）加入后停止。
  - 否则：把 `cur[:i]` 加入，并以它为新的 `cur` 继续。

示例：

- Unix：`/path/to/foo/bar` → `[/path/to/foo/bar, /path/to/foo, /path/to, /path, /]`
- Windows：`C:\Users\foo` → `[C:\Users\foo, C:\Users, C:]`

多条正则可能提取出多个不同的根路径；各自展开后并入同一个去重候选数组，一次性查询。

## 数据库匹配

新增 sqlc 查询 `MatchProjectByPaths`，对候选数组与项目 `paths`（JSONB 文本数组展开值）做相等匹配，
按命中路径长度降序取最长者：

```sql
SELECT p.id
FROM project AS p
CROSS JOIN LATERAL jsonb_array_elements_text(p.paths) AS path
WHERE path = ANY(@candidate_paths::text[])
ORDER BY length(path) DESC, p.id ASC
LIMIT 1;
```

`jsonb_array_elements_text` 返回的是解码后的真实值，与 Go 侧 JSON 解码后的候选路径形式一致，无需额外转义。

旧查询 `ListProjectPaths`（仅供内存缓存加载）删除。

## 自动创建项目

匹配未命中时：

1. 读取全局设置 `project.autoCreate`（JSON 布尔）。仅在此未命中分支读取，命中时不查设置。
2. 开关为 `true` 且存在某个 JSON 解码后的真实路径其长度 > 5 时触发创建（多条时取最长的那条作为来源路径）。
3. 项目名取来源路径最后一个 component（`strings.TrimRight(p, "/\\")` 后取最后一个 `/` 或 `\` 之后的部分）。
4. 插入 `project`（`auto_created = true`，`paths = [来源路径]`）。遇到名称唯一约束冲突时，给名称追加
   一个随机十六进制后缀（`name-xxxxxxxx`）后重试，最多重试若干次。
5. 返回新项目 id，写入本次请求 meta 行；后续请求会经正常路径匹配命中该项目。

## 数据模型变更

`project` 表新增列：

```sql
ALTER TABLE project ADD COLUMN auto_created BOOLEAN NOT NULL DEFAULT false;
```

`ProjectView` 增加 `autoCreated bool` 字段。

## 随机后缀

使用 `crypto/rand` 生成 4 字节并以十六进制编码（8 个字符）作为后缀，避免引入第三方库，也无需考虑
`math/rand` 的可重现性问题。

## 受影响组件

- 删除：`pkg/server/project_router.go`、`db/queries/project.sql` 中的 `ListProjectPaths`。
- 改写：`pkg/server/project_extractor.go`（持有 `*db.Queries`，承载候选生成、SQL 匹配、自动创建）。
- 改写：`pkg/server/server.go`（去掉 `projectRouter` 字段与构造，`newProjectExtractor` 改传 `queries`）。
- 改写：`pkg/server/handle_project.go`（删除两处 `projectRouter.Invalidate()` 调用）。
- 新增：迁移 `030_project_auto_created.sql`、查询 `MatchProjectByPaths`、`InsertAutoCreatedProject`。
- 改写：`pkg/contract/project.go`（`ProjectView.autoCreated`、`ToProjectView`）。
- 改写：仪表盘 `SettingsView.vue` 增加开关；重新生成 OpenAPI 与 TS 类型。
