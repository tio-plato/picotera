# Plan

## Step 1 — 后端：去重函数与调用点

**文件**：`pkg/server/handle_unified_gateway.go`

1. 新增 `betterRow(a, b db.GetProvidersByEndpointTypesAndModelRow, srcType int32) bool`：
   - srcType 命中作为最优；
   - 否则按 `EndpointType ∈ {AnthropicMessages, OpenAIChatCompletions}` 排序权重；
   - 否则 `EndpointPath` 字典序升序。
   返回 `true` 表示 `a` 应当胜出。

2. 新增 `dedupeUnifiedRows(rows []db.GetProvidersByEndpointTypesAndModelRow, srcType int32) []db.GetProvidersByEndpointTypesAndModelRow`：
   - 用 `map[int32]int` 把 `ProviderID → 选中行的下标`（同一 provider 同一 model，因为 `model_name` 在 query 里是常量）。
   - 遍历 `rows`，若新行 `betterRow` 当前选中行则替换。
   - 用 slice 收集结果，保持 provider 间的相对顺序（按首次出现顺序，确保后续 priority 排序的稳定性）。

3. 调整 `resolveProvidersByTypes(ctx, model, types)` 签名为 `resolveProvidersByTypes(ctx, model, types, srcType)`：
   - 在 `valid := …` 收集后、当前优先级冒泡排序前，调用 `dedupeUnifiedRows(valid, srcType)` 重新赋值 `valid`。
   - `srcType` 参数注入到 `dedupeUnifiedRows`。

4. 调用点 `handleUnifiedGenerate` 第 210 行附近，把 `sourceEndpointType(srcFormat)` 作为第四个参数传入。

## Step 2 — 后端：单元测试

**文件**：`pkg/server/handle_unified_gateway_test.go`

新增 `TestDedupeUnifiedRows`，构造 `[]db.GetProvidersByEndpointTypesAndModelRow` 字面量（不依赖 DB）：

| 用例 | 输入行（ProviderID / EndpointType / EndpointPath） | srcType | 期望保留 |
| --- | --- | --- | --- |
| single | (1, OpenAIChatCompletions, "/v1/chat") | OpenAIChatCompletions | 唯一一行 |
| src match | (1, AnthropicMessages, "/a"), (1, OpenAIChatCompletions, "/c") | OpenAIChatCompletions | "/c" |
| anthropic preferred | (1, OpenAIResponses, "/r"), (1, AnthropicMessages, "/a") | GeminiGenerateContent | "/a" |
| chat preferred | (1, OpenAIResponses, "/r"), (1, OpenAIChatCompletions, "/c") | GeminiGenerateContent | "/c" |
| path tiebreak | (1, OpenAIResponses, "/z"), (1, OpenAIResponses, "/a") | GeminiGenerateContent | "/a" |
| multi provider | (1, OpenAIChatCompletions, "/c"), (2, AnthropicMessages, "/a") | OpenAIChatCompletions | provider 1 → "/c"，provider 2 → "/a"，共两行 |

验证 `len(out)` 与每行的 `ProviderID + EndpointPath` 期望相符。

## Step 3 — 前端：合并优先级 section

**文件**：`dashboard/src/components/ModelUpstreamsPanel.vue`

1. 在 `<script setup>` 内新增 `mergedUpstreams` computed：
   ```ts
   const mergedUpstreams = computed(() =>
     [...props.upstreams].sort((a, b) => {
       const score = (b.priority + b.providerPriority) - (a.priority + a.providerPriority)
       if (score !== 0) return score
       return a.providerId - b.providerId
     }),
   )
   ```

2. 在模板里、现有「按端点分组」section 之前增加新的 section：
   ```html
   <section v-if="upstreams.length" class="flex flex-col gap-2">
     <div class="flex items-baseline justify-between">
       <span class="text-xs font-medium text-ink-muted uppercase tracking-[0.03em]">
         Unified 路由优先级
       </span>
       <span class="text-xs text-ink-faint tabular-nums">{{ mergedUpstreams.length }} 上游</span>
     </div>
     <ul class="list-none m-0 p-0 flex flex-col border border-line rounded-md bg-surface-0 overflow-hidden">
       <li
         v-for="(u, i) in mergedUpstreams"
         :key="`merged:${u.providerId}:${i}`"
         class="px-2.5 py-2 border-t border-line-soft first:border-t-0 flex items-center gap-1.5 flex-wrap"
         :class="(u.providerDisabled || u.entryDisabled) ? 'opacity-55' : ''"
       >
         <span class="text-2xs text-ink-faint tabular-nums w-5 text-right flex-none">
           {{ i + 1 }}
         </span>
         <span class="text-sm font-semibold text-ink">{{ u.providerName }}</span>
         <Tag v-if="u.providerDisabled" variant="muted">渠道已禁用</Tag>
         <Icon name="chevron-down" :size="12" class="-rotate-90 text-ink-faint" />
         <Tag variant="accent">{{ u.upstreamModelName }}</Tag>
         <Tag v-if="u.providerPriority + u.priority > 0" variant="more">
           P{{ u.providerPriority + u.priority }}
         </Tag>
         <Tag v-if="u.entryDisabled" variant="muted">上游已禁用</Tag>
       </li>
     </ul>
   </section>
   ```

3. 现有的「按端点分组」section 不动。把两个 section 包到外层 `<div class="flex flex-col gap-3">` 里拉开间距。

## Step 4 — 验证

1. 运行 `go test ./pkg/server/...`，确认新单元测试通过。
2. 运行 `go build -o /tmp/picotera ./cmd/picotera`，确认编译通过。
3. 启动 `mise run server` 与 `mise run web`，在面板里：
   - 选一个绑定多种 endpoint 的 provider，打开模型上游面板，确认上方出现「Unified 路由优先级」列表，按优先级降序排列。
   - 禁用其中一个 provider 或 entry，确认对应行置灰并显示 Tag。
4. 不需要调用 `mise run openapi` / `pnpm --dir dashboard generate-openapi`，本次没有改契约或新增 endpoint。

## Step 5 — TODO 收尾

完成后从 `TODO.md` 移除前两条。提交分两段：

- `feat(unified): dedupe candidate endpoints per (provider, model)` —— 后端改动 + 测试。
- `feat(dashboard): show unified routing priority in model upstream panel` —— 前端改动。
