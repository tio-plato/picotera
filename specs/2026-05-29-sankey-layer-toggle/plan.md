# 桑基图层显示/隐藏 执行计划

## 任务列表

1. **修改 `buildDimensionSankey` 函数** — 支持传入 `hiddenLayerIndices` 参数，在生成链接时跳过被隐藏的层
2. **修改 `tokenCompositionSankey` 计算逻辑** — 支持传入 `hiddenLayerIndices`，处理固定结构的层隐藏
3. **修改 `OverviewSankey.vue` 组件** — 添加图例渲染和交互，管理隐藏状态
4. **修改 `OverviewView.vue`** — 为每个桑基图变体添加层定义和隐藏状态，传递给组件
5. **本地构建验证** — 运行 `pnpm --dir dashboard type-check` 和 `pnpm --dir dashboard build`

## 详细步骤

### 步骤 1: 修改 `buildDimensionSankey`

- 添加第四个参数 `hiddenLayerIndices: number[] = []`
- 生成链接前，先计算可见层的索引序列 `visibleIndices`
- 对于 root → 第一个可见层：生成 root → layer[visibleIndices[0]]
- 对于连续可见层：生成 layer[prev] → layer[curr]
- 其余逻辑不变（fold、top N、节点去重等）

### 步骤 2: 修改 `tokenCompositionSankey`

- 添加 `hiddenLayerIndices` 参数
- 当 Layer 1 隐藏时，root 直接连接到 Layer 2 的节点
- 当 Layer 2 隐藏时，Layer 1 的节点直接聚合连接到 root（或直接省略）
- 由于数据结构固定，可以简化处理

### 步骤 3: 修改 `OverviewSankey.vue`

- 添加新的 props: `layers?: { key: string; label: string }[]`
- 添加内部状态 `hiddenLayers: Set<number>`
- 在图表下方渲染图例：
  - 使用 flex 布局横向排列图例项
  - 每个图例项包含圆点和文字
  - 点击切换 `hiddenLayers` 中的索引
  - 隐藏状态添加删除线和透明度降低
- 将 `hiddenLayers` 以事件形式暴露给父组件，或通过 v-model 双向绑定
- 根据 `hiddenLayers` 过滤 nodes 和 links（在父组件中重新计算）

### 步骤 4: 修改 `OverviewView.vue`

- 为每种桑基图变体定义层信息：
  - `tokenComposition`: [{key:'root', label:'总Token'}, {key:'io', label:'输入/输出'}, {key:'detail', label:'细分'}]
  - `tokensIn`: [{key:'provider', label:'渠道'}, {key:'upstreamModel', label:'上游模型'}, {key:'model', label:'请求模型'}, {key:'apiKey', label:'密钥'}, {key:'project', label:'项目'}]
  - `tokensOut`: 反向
  - `costIn`/`costOut`: 同上
- 添加 `hiddenLayers` 状态（每个变体独立，使用 `Map<SankeyVariant, Set<number>>`）
- 将 `hiddenLayers` 传给 `buildDimensionSankey` 或 `tokenCompositionSankey`
- 将层定义传给 `OverviewSankey` 组件以渲染图例

### 步骤 5: 验证

- 运行 `pnpm --dir dashboard type-check`
- 修复类型错误
- 运行 `pnpm --dir dashboard build`
- 确认无编译错误
