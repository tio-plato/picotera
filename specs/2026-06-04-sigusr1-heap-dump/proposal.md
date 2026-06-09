# SIGUSR1 Heap Dump

## 原始需求

我需要增加一个排查内存泄漏的功能,当收到 SIGUSR1 之后,就主动做一个 memory heap dump,包括本进程和插件子进程。

## 补充澄清(与用户确认)

- **Dump 格式**:pprof 格式,同时输出 heap、goroutine、allocs 三种 profile,可用 `go tool pprof` 直接分析。
- **插件子进程的触发方式**:主进程将 SIGUSR1 转发给插件子进程,插件自装信号处理器并自己写文件;不新增 gRPC RPC,不改 proto / ABI。
- **输出目录**:可配置,新增 `PICOTERA_HEAP_DUMP_DIR` 环境变量,默认系统临时目录(`os.TempDir()`)。文件名带时间戳和进程角色。
