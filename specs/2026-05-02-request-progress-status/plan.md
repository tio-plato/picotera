# Plan

## 1. `db/queries/request.sql`

新增两条单字段 update 查询：

```sql
-- name: UpdateRequestEndpoint :exec
UPDATE request SET endpoint_path = $2 WHERE id = $1;

-- name: UpdateRequestModel :exec
UPDATE request SET model = $2 WHERE id = $1;
```

跑 `sqlc generate`，重新生成 `pkg/db/`。

## 2. `pkg/server/gateway_helpers.go`

仿照 `updateRequestOnHeader` 加两个 wrapper（错误仅日志）：

```go
func (s *Server) updateRequestEndpoint(ctx context.Context, arg db.UpdateRequestEndpointParams)
func (s *Server) updateRequestModel(ctx context.Context, arg db.UpdateRequestModelParams)
```

## 3. `pkg/server/handle_gateway.go`

在已有流程中插入两次回填，使用 `bgCtx`（与其它后台 update 一致，避免请求被 cancel 时丢字段）：

- 步骤 1 `resolveEndpoint` 成功之后、读取 body 之前：
  ```go
  // 此时 metaID 还没插入，先把 endpoint 字段改成在 insertRequest 时直接带上。
  ```
  实际写法：把 `EndpointPath: pgtype.Text{Valid: false}` 改成在路径匹配后用 `endpoint.Path` 直接初始化。**不需要单独 update 查询用于 endpoint 的"步骤 1 之后"**——因为 metaID 是步骤 3 才生成，endpoint 在那之前已经知道。所以 endpoint 字段在 `insertRequest` 调用处直接赋值即可：
  ```go
  EndpointPath: pgtype.Text{String: endpoint.Path, Valid: true},
  ```
  → 删除 `UpdateRequestEndpoint` 查询和 wrapper。仅保留 model 那条。

- 步骤 5 `extractModel` 成功之后：
  ```go
  h.updateRequestModel(bgCtx, db.UpdateRequestModelParams{
      ID:    metaID,
      Model: pgtype.Text{String: modelName, Valid: modelName != ""},
  })
  ```

注：`streamSuccess` 内对 meta 的 `updateRequestOnHeader` 仍会带上 model（与原始 modelName 一致），保留即可，幂等。

> 计划修订：仅新增 `UpdateRequestModel` 一条 SQL；endpoint 通过 insert 时直接赋值搞定。

## 4. `dashboard/src/views/RequestsView.vue`

- 抽出 `requestState(row: RequestView)` 工具函数（脚本顶部）：
  ```ts
  type RequestState = 'pending' | 'ok' | 'warn' | 'err'
  function requestState(r: RequestView): RequestState {
    // status: 0=Pending 1=HeaderReceived 2=Completed 3=Failed
    if (r.status === 0) return 'pending'
    return statusVariant(r.statusCode)   // header/completed/failed 都按 statusCode 取色
  }
  ```
- `cell-status` 模板分支：`requestState === 'pending'` 渲染中性"处理中"徽章；其它分支沿用现有 ok/warn/err 三色。
  ```html
  <span v-if="requestState(row) === 'pending'"
        class="inline-flex items-center px-1.5 py-0.5 rounded-[5px] font-mono text-2xs leading-[1.2] bg-surface-100 text-ink-muted border border-line-soft">
    处理中
  </span>
  <span v-else
        class="inline-flex items-center px-1.5 py-0.5 rounded-[5px] font-mono text-2xs leading-[1.2] border border-transparent"
        :class="{ 'bg-ok-faint text-ok-ink': requestState(row) === 'ok',
                  'bg-warn-faint text-warn-ink': requestState(row) === 'warn',
                  'bg-err-faint text-err-ink': requestState(row) === 'err' }">
    {{ row.statusCode }}
  </span>
  ```
  顺手删除原来 `{{ row.statusCode || 'ERR' }}` 这种"缺值落到 ERR"写法。

- 删 / 调整 `statusVariant` 中 `code === undefined` 时返回 `'err'` 的分支：改为返回 `'err'` 仅在 status === Failed 时使用；上面 requestState 已经把 undefined 分流到 pending。

## 5. `dashboard/src/components/RequestDetailsPanel.vue`

- 同样定义 `requestState(r)` / `statusBadgeClass(r)`，复用相同语义。
- 顶部卡片（meta、upstream 列）的状态码徽章和 overview 的"状态码"字段：
  - `requestState === 'pending'` 时渲染"处理中"中性徽章；
  - 否则按 `statusCodeClass(statusCode)` 渲染数字。
- "状态" Field（用 `statusLabel(status)`）保留 — 它本来就显示 status 文本（pending/header/completed/failed），现状已正确。仅把 Tag variant 也接 requestState：pending 走 muted，其它沿用 statusVariantTag。

## 6. 验证

- `sqlc generate` 通过。
- `go build ./...` 通过。
- `mise run openapi` 跑一次（schema 没动 contract，理论上 diff 为空；以防万一）。
- `pnpm --dir dashboard type-check && pnpm --dir dashboard build` 通过。
- 手动 smoke：
  1. `docker compose up -d && mise run server`，启动 dashboard `mise run web`。
  2. 配置一个故意慢的 upstream（或直接用本地 echo），打一次请求。
  3. 在请求列表刷新观察：在途行 endpoint / model 立刻有值，状态徽章是中性"处理中"。
  4. 完成后徽章变成 200 绿。
  5. 故意打一个不存在的 model（触发 `extractModel` 之前路径已匹配，但 model 提取失败/无 provider）—— meta 行 endpoint 已写、model 依失败时机有/无；状态徽章按 statusCode 显示对应红/黄。

## 7. 文档

无 README/docs 改动。CLAUDE.md 也不需要更新。
