# Design

## 现状

`request.status` 当前四值：`Pending(0)` / `HeaderReceived(1)` / `Completed(2)` / `Failed(3)`。

Meta 请求 lifecycle（见 `pkg/server/handle_gateway.go`）：

1. 入口插入 meta，`status=Pending`，`endpoint_path` / `model` / `provider_id` 全为 NULL。
2. 后续 endpoint 解析、auth 校验、模型提取、provider 选择、retry loop 全过程中 meta 行**不动**。
3. 仅在 `streamSuccess` 内首次写入 `endpoint_path` / `model` / `provider_id`，并切到 `HeaderReceived`，最终 `Completed`。
4. 中途任意失败路径走 `failMeta`，写 `Failed` + statusCode + error，但 endpoint/model 仍为 NULL。

UI（`dashboard/src/views/RequestsView.vue`、`dashboard/src/components/RequestDetailsPanel.vue`）只看 `statusCode`：缺失即标红 "ERR"。在途请求与系统错误视觉无差异。

## 问题

- 处理时间长（流式 LLM 几十秒）的请求在列表里看不出 endpoint / model，因为这两列直到结束才回填。
- 故障路径（如 "all providers failed"、auth 失败、model 提取失败）的 meta 行也没有 endpoint/model，事后排查只能翻 artifact。
- "在途" 与 "出错" 在 UI 上视觉一致，让人误以为系统大面积出错。

## 方案

### 1. 及时回填 meta 字段

新增两个轻量级 SQL 更新，分别在已知字段时立刻调用：

- 路径匹配成功后（`resolveEndpoint` 返回 `endpoint`）：写 `endpoint_path`。
- 模型提取成功后（`extractModel` 返回 `modelName`）：写 `model`（即客户端原始 model；后续 rewriteModel hook 改写不覆盖此值——保持现有 `originalModelName` 语义一致）。

> 注：`provider_id` / `upstream_model` 仍按现状只在最终成功的那一次（streamSuccess）写入。每次 retry 改写一次反而误导，且我们在 upstream 子请求里已经分别记录每次尝试。

UPDATE 失败仅记日志，不影响请求处理（与 `updateRequestOnHeader` / `updateRequestOnComplete` 现有约定一致）。

### 2. 不改 DB schema

不新增 status 值。新增的 SQL 只更新具体列，不动 `status`：

- 路径匹配后：仍是 `Pending`。
- 模型提取后：仍是 `Pending`。
- streamSuccess 第一次写入 provider 时：`HeaderReceived`（沿用现状）。

UI 把 status ∈ {Pending, HeaderReceived} 且没有 statusCode 的行视作"处理中"，与 Failed 区分开。

### 3. UI 渲染规则

新增小工具 `requestState(row)`，返回 `'pending' | 'ok' | 'warn' | 'err'`：

```
if (status === Completed || status === HeaderReceived) statusCode 取色（200=ok, 4xx=warn, 5xx=err）
else if (status === Failed) statusCode 取色；缺 statusCode 时 'err'
else 'pending'   // status === Pending 或未知
```

> 视觉：`pending` 用中性灰底 + 蓝点（或文本"处理中"），与 ok/warn/err 三种结果状态明确区分。配色用现有 `bg-surface-100 text-ink-muted` 一类中性 token，不引入新色板。

替换两处用到 `statusCode` 的徽章：
- `RequestsView.vue` 列表 `cell-status`：当 `requestState(row) === 'pending'` 渲染"处理中"徽章；否则保持原有 statusCode 数字。
- `RequestDetailsPanel.vue` 选项卡和 overview 状态码字段：同样用 requestState 判断，pending 时显示"处理中"而非 `—`/红色。

## 不涉及

- 不动 `RequestStatus*` 常量值或 schema 迁移。
- 不动 upstream 子请求的字段回填（一开始就有完整 provider/endpoint/model）。
- 不重排 retry loop / hooks 调用顺序。
- 不增加 polling/SSE 推送（前端仍靠手动刷新看新状态）。
