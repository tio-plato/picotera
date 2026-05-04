# Design

两个修改共享一条核心规则：**一个 (provider, model) 在 unified 路由里只贡献一个候选 endpoint**，挑选规则按客户端请求格式 → AnthropicMessages → OpenAIChatCompletions → `endpoint.path` 字典序逐级降级。后端在生成 candidate 列表前应用规则；前端在面板里把同一规则可视化为「合并优先级」列表。

## 1. 后端：unified 候选去重

### 1.1 现状

`pkg/server/handle_unified_gateway.go:676` 的 `resolveProvidersByTypes` 通过 `GetProvidersByEndpointTypesAndModel` 查到 `[]Row`，每行是一个 `(provider, provider_endpoint, model_entry)` 三元组。一个 provider 若同时绑定了多种格式的 endpoint（例如 `anthropicMessages` 和 `openaiChatCompletions`），同一个 model 会产生多行；这些行随后被加入 candidate 列表，`beforeRequest` 重试循环可能在同一 (provider, model) 上多次尝试不同 endpoint，与「同一上游只该尝试一次」的预期相悖。

### 1.2 改动

新增纯函数 `dedupeUnifiedRows(rows []db.GetProvidersByEndpointTypesAndModelRow, srcType int32) []db.GetProvidersByEndpointTypesAndModelRow`，在 `pkg/server/handle_unified_gateway.go` 内：

1. 按 `(ProviderID, ModelName)` 分桶。
2. 每桶内挑出唯一一行 `best`，比较函数 `betterRow(a, b, srcType) bool` 逐级判定：
   - 若 `a.EndpointType == srcType` 且 `b.EndpointType != srcType` → `a` 胜出（反之亦然）。
   - 否则 `a.EndpointType == AnthropicMessages` 优先于其他非 srcType 类型。
   - 否则 `a.EndpointType == OpenAIChatCompletions` 优先。
   - 否则按 `EndpointPath` 字典序升序。
3. 桶内 `best` 收集为切片返回。

调用点：在 `resolveProvidersByTypes` 内、合法性过滤之后、当前的优先级排序之前，先调用 `dedupeUnifiedRows`，然后再做 priority 排序。`srcType` 由调用方注入，把 `resolveProvidersByTypes(ctx, model, types)` 改为 `resolveProvidersByTypes(ctx, model, types, srcType)`，调用点（`handleUnifiedGenerate`）传 `sourceEndpointType(srcFormat)`。

### 1.3 与 SQL 的关系

不改 `db/queries/routing.sql`，不改 sqlc 生成代码。`endpoint.path` 字典序 tiebreak 用现有 `EndpointPath` 字段就够，因此不需要新增 `endpoint.id` 列到 SELECT。

### 1.4 不变量

- 路径式网关（`handle_gateway.go`）不受影响：它只查单个 endpoint 路径，本来就一行一 (provider, model)。
- JS hooks 看到的 `Candidate` 列表减少了重复项。`sortProviders` / `beforeRequest` 仍可重排，行为对脚本作者更直观。
- `priority + provider_priority` 排序仍然在 dedup 之后做，结果只受去重影响、不受去重内的取舍影响。

## 2. 前端：模型上游面板的合并列表

### 2.1 现状

`dashboard/src/components/ModelUpstreamsPanel.vue` 接收 `upstreams: Upstream[]`，按 `endpointPaths` 拆并按端点分组渲染。用户看不到「unified 路由会优先选哪个上游」。

`upstreams` 的来源 `dashboard/src/views/ModelsView.vue:83-110` 已经是「一个 (provider, model) 对应一项 `Upstream`」，因此**合并列表的行级数据已经齐了**，只是没有按合并优先级展示。

### 2.2 改动

#### 2.2.1 `ModelUpstreamsPanel.vue`

在「按端点分组」section 之前新增一个 section「Unified 路由优先级」：

- 标题行：kicker `合并优先级`，右侧 `{N} 上游` 计数（N = `props.upstreams.length`，含禁用项）。
- 列表项（`<ul>`）：每行一个 `Upstream`，按 `(providerPriority + priority) desc, providerId asc` 排序（`providerId` 升序仅作 stable tiebreak，不与后端「endpoint.path 字典序」对应，因为合并列表里不暴露 endpoint 信息）。
  - 内容：`{providerName}` （粗体）→ chevron-right 图标 → `Tag variant="accent"` 包 `{upstreamModelName}` → 可选 `Tag variant="more"` `P{combinedPriority}`（仅当合并优先级 > 0）。
  - 禁用展示：`providerDisabled || entryDisabled` 时整行 `opacity-55`，并附 `Tag variant="muted"` 「渠道已禁用」/「上游已禁用」。
- 空态：当 `props.upstreams.length === 0` 时不渲染该 section（保留底部「按端点分组」section 的现有空态逻辑）。

不增加新参数：合并列表所需信息（`providerId`、`providerName`、`upstreamModelName`、`priority`、`providerPriority`、`providerDisabled`、`entryDisabled`）已在 `Upstream` 类型中。

#### 2.2.2 `ModelsView.vue`

`upstreamIndex` 当前会把 `entry.endpoints` 为空时回退展开为「该 provider 已绑定的全部已路由化端点」。这部分逻辑保持不变；合并列表不依赖 `endpointPaths`。

#### 2.2.3 不引入新 endpoint / 类型 / 包

合并列表纯前端 derive。`Upstream` 类型不动。新增的 section 复用 `Tag`、`Icon`、`SidePanel` 等现有原语。

### 2.3 与后端规则的一致性

合并列表只展示 `(provider, modelName, upstreamModelName)`，不暴露具体被选中的 endpoint。后端 dedup 在桶内的取舍只影响「实际命中哪条 endpoint」，不影响行的存在与否，所以前端列表无需感知 srcFormat 即可与 unified 路由的「行集合」对齐。combinedPriority 排序与后端 `priority + provider_priority` 一致。

## 3. 测试

- 后端：在 `pkg/server/handle_unified_gateway_test.go` 增 `TestDedupeUnifiedRows`，覆盖五种情形：
  1. 单行直通；
  2. 同一 (provider, model) 上两条 endpoint，srcType 命中其中之一；
  3. srcType 都不命中、AnthropicMessages 优先；
  4. srcType 与 AnthropicMessages 都不命中、OpenAIChatCompletions 优先；
  5. 全部 fallback 到 `endpoint.path` 字典序 tiebreak。
- 前端：现行无单元测试；通过 `mise run web` 手测面板。

## 4. 范围外

- 不动 `handle_gateway.go`（路径式网关）的 candidate 构建。
- 不为 `unified` 路由额外暴露 dedup 后的命中信息到 JS hooks（`Candidate.MPE` 仍然是单个 endpoint，自然反映 dedup 结果）。
- 不在面板内提供与 `endpointType` 相关的过滤（例如「只看 AnthropicMessages」）。
