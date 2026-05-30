# 桑基图层显示/隐藏 设计文档

## 系统架构

### 层定义

概览桑基图包含两类：

1. **Token 组成桑基图** (`tokenComposition`)：固定三层结构
   - Layer 0: 根节点 (总 Token)
   - Layer 1: 输入/输出
   - Layer 2: 细分类别 (缓存读取、缓存写入等)

2. **多维流向桑基图** (`tokensIn`, `tokensOut`, `costIn`, `costOut`)：由 `buildDimensionSankey` 动态生成，使用 5 层维度：
   - `provider` → `upstreamModel` → `model` → `apiKey` → `project`
   - 或反向：`project` → `apiKey` → `model` → `upstreamModel` → `provider`

### 图例组件

新增 `SankeyLayerLegend` 组件，位于 `OverviewSankey.vue` 下方，接收层定义数组和隐藏状态集合，渲染可点击的图例项。

### 隐藏层后的链接计算

对于多维流向桑基图，当某一层被隐藏时：

1. 遍历每一行原始数据
2. 找出该行所有可见层的索引
3. 生成链接时跳过被隐藏的层：
   - 若第一个可见层索引 > 0：root → 第一个可见层
   - 连续可见层之间：上一可见层 → 当前可见层
4. 相同起点和终点的链接自动聚合

对于 Token 组成桑基图：
- 隐藏 Layer 1（输入/输出）时，root 直接连接到 Layer 2 的各节点
- 隐藏 Layer 2 时，Layer 1 的节点直接连接到聚合节点
- 由于结构固定，实现逻辑相对简单

### 交互设计

- 图例项左侧显示圆点（使用对应层的主色调）
- 隐藏状态的图例项显示为灰色/半透明，并带删除线
- 点击图例项切换显示/隐藏状态
- 图例项横向排列，居中于桑基图下方

### 颜色映射

复用 `groupColor(index)` 为每一层分配颜色，其中 index 对应层深度。

### 中文标签映射

```
provider      → 渠道
upstreamModel → 上游模型
model         → 请求模型
apiKey        → 密钥
project       → 项目
root          → 根节点
input         → 输入
output        → 输出
in_uncached   → 未缓存输入
in_cache_read → 缓存读取
in_cache_write → 缓存写入
in_cache_write_1h → 长期缓存写入
```
