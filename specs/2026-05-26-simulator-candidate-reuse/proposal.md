# 原始需求：Simulator 复用 Gateway Candidate Helper

从 gateway handler 重构（`specs/2026-05-26-gateway-handler-refactor`）中拆出的独立任务。

重构后 `pkg/server/gateway_flow_candidates.go` 提供了标准化的 `buildPathCandidateSet` / `buildUnifiedCandidateSet` helper。`handle_simulate.go` 内部有类似的 candidate 构造逻辑，应当在 gateway 重构完成后评估是否可以复用这些 helper，避免两套并行的构造代码。

如果 helper 接口自然对齐则复用；如果需要为 simulator 加额外参数或条件分支，则保持 simulator 独立不做强行复用。Simulator 的响应语义和排序行为保持不变。
