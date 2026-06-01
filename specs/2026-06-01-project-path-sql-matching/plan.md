# 执行计划

## 1. 数据库迁移

- 新增 `db/migrations/030_project_auto_created.sql`：
  - Up：`ALTER TABLE project ADD COLUMN auto_created BOOLEAN NOT NULL DEFAULT false;`
  - Down：`ALTER TABLE project DROP COLUMN IF EXISTS auto_created;`

## 2. sqlc 查询

- 编辑 `db/queries/project.sql`：
  - 删除 `ListProjectPaths`。
  - 新增 `MatchProjectByPaths :one`（候选数组 `@candidate_paths::text[]`，`CROSS JOIN LATERAL
    jsonb_array_elements_text(paths)`，`WHERE path = ANY(...)`，`ORDER BY length(path) DESC, id ASC LIMIT 1`）。
  - 新增 `InsertAutoCreatedProject :one`（`INSERT ... (name, paths, auto_created) VALUES ($1, $2, true) RETURNING *`）。
- 运行 `sqlc generate`，确认 `pkg/db/` 更新（`Project.AutoCreated`、新方法、`Querier` 接口）。

## 3. 删除内存缓存

- 删除文件 `pkg/server/project_router.go`。
- `pkg/server/server.go`：去掉 `projectRouter` 字段、`newProjectRouter` 调用，`newProjectExtractor`
  改为接收 `queries`。
- `pkg/server/handle_project.go`：删除 `handleUpsertProject` / `handleDeleteProject` 中两处
  `s.projectRouter.Invalidate()`。

## 4. 改写 project_extractor.go

- `projectExtractor` 改持有 `queries *db.Queries`；`newProjectExtractor(q *db.Queries)`。
- 保留正则提取 + `decodeJSONString` 得到去重后的真实路径列表（`extractProjectCandidates`）。
- 新增 `ancestorPaths(p string) []string`：按设计生成「自身 + 各级祖先」，分隔符为 `/` 或 `\`
  （`strings.LastIndexAny(cur, "/\\")`，`i==0` 时加根分隔符后停止）。
- `Extract(ctx, body) (int32, bool, error)`：
  1. 取真实路径列表；为空返回 `(0,false,nil)`。
  2. 各路径 `ancestorPaths` 并入去重候选数组。
  3. 调 `MatchProjectByPaths`；命中返回 id。`pgx.ErrNoRows` 视为未命中。
  4. 未命中 → 调 `maybeAutoCreate(ctx, 真实路径列表)`。
- 新增 `maybeAutoCreate`：
  1. 读 `GetGlobalSetting(ctx, "project.autoCreate")`，解析 JSON 布尔；键不存在/为假返回 `(0,false,nil)`。
  2. 从真实路径列表选出长度 > 5 的最长路径；无则返回未命中。
  3. `name = lastPathComponent(path)`（`strings.TrimRight(p, "/\\")` 后取最后一个 `/` 或 `\` 之后部分）。
  4. `paths = json.Marshal([]string{path})`，调 `InsertAutoCreatedProject`；
     遇唯一约束冲突（`isUniqueViolation`）时 `name = base + "-" + randomSuffix()` 重试，最多 5 次。
  5. 成功返回新 id。
- 新增 `randomSuffix() string`：`crypto/rand` 读 4 字节 → `hex.EncodeToString`。

## 5. Server 入口

- `pkg/server/gateway_helpers.go` 的 `extractProjectID` 不变（仍调用 `s.projectExtractor.Extract`），
  确认对 path-based 与 unified 两条网关均生效（共用 `gatewayFlow.insertMetaRequest`）。

## 6. 契约与 OpenAPI

- `pkg/contract/project.go`：`ProjectView` 增 `AutoCreated bool \`json:"autoCreated"\``；`ToProjectView`
  填充 `p.AutoCreated`。
- 运行 `mise run openapi` 重新生成 `openapi.yaml`。

## 7. 仪表盘

- `pnpm --dir dashboard generate-openapi` 重新生成 TS 类型。
- `dashboard/src/views/SettingsView.vue`：新增「允许自动创建项目」开关（复选框），读写
  `project.autoCreate`（JSON 布尔），保存与 `app.title` 同一保存按钮或独立保存，遵循现有 UI 原语
  （`@/ui`，参考 MEMORY：仅用本地 Tailwind 原语）。
- 可选：在项目列表/详情对自动创建项目展示标记（`autoCreated`）。

## 8. 测试与验证

- 更新 `pkg/server/project_extractor_test.go`：
  - 保留既有候选提取用例。
  - 新增 `ancestorPaths` 用例（Unix 多级、Windows 多级、根、尾部分隔符、混合分隔符）。
  - 新增 `lastPathComponent` 用例（Unix / Windows / 尾部分隔符）。
- `go build ./...`、`go test ./pkg/server/... ./pkg/llmbridge/...`。
- `pnpm --dir dashboard type-check && pnpm --dir dashboard lint`。
- 手动验证（可选）：开启 `project.autoCreate`，发一条带 cwd 的网关请求，确认自动建项目且 meta 行带
  `project_id`；再发同路径请求确认走匹配而非重复创建。

## 影响范围与注意

- 路径匹配从字符前缀改为分隔符对齐的祖先相等匹配，修掉旧的非边界误命中；行为对正常按目录配置的项目一致。
- 匹配与自动创建在请求热路径同步执行：命中走一条 SELECT；未命中且开关开启时额外一次设置读取 + 一次 INSERT，
  之后该路径请求转为命中，不再重复创建。
- 不引入兼容层；旧内存缓存路径整体移除。
