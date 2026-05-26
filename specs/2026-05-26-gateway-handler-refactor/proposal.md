# 原始需求：Gateway Handler 重构

阅读 `pkg/server/handle_gateway.go` 和 `pkg/server/handle_unified_gateway.go`。这两份代码有一些相似之处，更重要的是，前者有一个高达 500 行的 `ServeHTTP` 函数。它虽然工作，但太大了。

希望对这两个文件进行一次大规模地重构，使得它们能共享部分代码，并将大函数拆分为较小的部分，同时能正确使用 context（而不是随处都用 background）并处理错误。

本规格提出合理的重构计划。
