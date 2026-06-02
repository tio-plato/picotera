# 项目合并功能

为“项目”增加一个“合并”功能：选择某个项目，点合并，在弹出的窗口中，选择要合并去的目标项目。提交之后，所有源项目的 trace 和 requests 都将自动修改为目标项目，源项目将自动被删除。

## 用户澄清后的细节

- **数据迁移**：单事务内依次执行：
  1. 更新目标项目：`paths` = 源与目标 paths 的 DISTINCT 并集；`first_seen_at` = 两个项目的最小值；`last_seen_at` = 两个项目的最大值。
  2. `UPDATE request SET project_id = $target WHERE project_id = $source`（影响所有 trace 的所有 span，因为 trace 在数据库里是 request 行的聚合视图，不存在独立 `trace.project_id`）。
  3. `DELETE FROM project WHERE id = $source`。
- **禁止自合并**：`source_id == target_id` 时后端返回 `400 Bad Request`。
- **UI 入口**：在 `ProjectsView` 表格的“操作”列加一个“合并”图标按钮（`git-merge` 图标），点击后弹出 `SidePanel`（即 `MergeProjectForm.vue`），里面只有一个下拉选择器（目标项目，必须排除源项目自身）。
- **执行模型**：同步单事务，前端 await 后端响应。失败时给出错误提示。
