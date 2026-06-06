# 设计:SIGUSR1 触发 Heap Dump

## 概述

主进程(picotera 网关)和 llmbridge 插件子进程各自安装 SIGUSR1 信号处理器。收到信号后,各自在进程内用 `runtime/pprof` 生成 heap、goroutine、allocs 三个 profile 文件,写入同一个输出目录。主进程收到 SIGUSR1 后,先写自己的 profile,再把 SIGUSR1 转发给插件子进程(若存活)。

不修改 gRPC proto,不 bump ABI 版本。

## 新增包:`pkg/heapdump/`

dump 写盘逻辑被主进程和插件二进制共用,放在独立包中:

```
pkg/heapdump/heapdump.go
```

- `func Write(dir, role string) ([]string, error)` — 依次写出三个文件并返回路径列表:
  - `picotera-<role>-<timestamp>-heap.pprof` — 先 `runtime.GC()` 再 `pprof.Lookup("heap").WriteTo(f, 0)`,保证 heap profile 反映 GC 后的存活集(排查泄漏的标准做法);
  - `picotera-<role>-<timestamp>-allocs.pprof` — `pprof.Lookup("allocs").WriteTo(f, 0)`;
  - `picotera-<role>-<timestamp>-goroutine.pprof` — `pprof.Lookup("goroutine").WriteTo(f, 0)`。
  - `<timestamp>` 取 `time.Now().UTC().Format("20060102T150405")`;`<role>` 为 `host` 或 `plugin`。
  - 任一文件写失败立即返回错误(已成功写出的文件保留)。
- `func Install(dir, role string, onDump func())` — `signal.Notify(ch, syscall.SIGUSR1)` 并启动 goroutine:每收到一次信号,调用 `Write` 并通过 logrus 记录写出的文件路径(失败记录错误);随后调用 `onDump`(可为 nil)。信号处理串行,处理期间到达的重复信号由 channel 缓冲(容量 1)自然合并。

仅支持 Unix(`syscall.SIGUSR1`),不加 build tag,与项目的 Linux 部署目标一致。

## 主进程接入

- `pkg/configx/`:`Config` 新增 `HeapDumpDir string`(`mapstructure:"heap_dump_dir"`,即 `PICOTERA_HEAP_DUMP_DIR`),viper 默认值 `os.TempDir()`。
- `pkg/server/server.go`:`Server.Serve()` 在 `ListenAndServe` 之前调用 `heapdump.Install(s.config.HeapDumpDir, "host", onDump)`,其中 `onDump` 调用 `s.llmBridge.SignalPlugin(syscall.SIGUSR1)`,即:主进程先完成自身 dump,再转发信号给插件。

## Bridge 接口扩展

`pkg/llmbridge/client.go` 的 `Bridge` 接口新增一个方法:

```go
SignalPlugin(sig syscall.Signal) error
```

- `disabledBridge`:no-op,返回 nil。
- `pluginBridge`(`plugin_client.go`):持有 `b.mu`,若 `b.client == nil` 或 `b.client.Exited()` 则跳过(记日志后返回 nil)——**不会**为了 dump 而重启一个已死的插件,新进程的 heap 对排查泄漏没有意义;否则取 `b.client.ReattachConfig().Pid`(nil 或 0 同样跳过),`syscall.Kill(pid, sig)`。

## 插件子进程接入

- **输出目录传递**:`llmbridge.Config` 新增 `HeapDumpDir string`,`startPlugin` 在 `exec.Command` 上设置 `cmd.Env = append(os.Environ(), "PICOTERA_HEAP_DUMP_DIR="+cfg.HeapDumpDir)`,主进程把已解析(含默认值)的目录显式传给子进程,保证两端写入同一目录。插件重启(crash 后 `restartLocked`)走同一条 `startPlugin` 路径,环境变量与信号处理器自动恢复。
- `cmd/picotera-llmbridge-plugin/main.go`:`main()` 在 `plugin.Serve` 之前读取 `PICOTERA_HEAP_DUMP_DIR`(为空则用 `os.TempDir()`,覆盖手工单独运行插件的场景),调用 `heapdump.Install(dir, "plugin", nil)`。插件的 dump 日志写 stderr,经由现有 `pluginLogWriter` 汇入主进程日志。

## 信号流

```
operator: kill -USR1 <host-pid>
  └─ host: heapdump.Write(dir, "host")     → picotera-host-…-{heap,allocs,goroutine}.pprof
  └─ host: SignalPlugin(SIGUSR1)           → kill -USR1 <plugin-pid> (若存活)
       └─ plugin: heapdump.Write(dir, "plugin") → picotera-plugin-…-{heap,allocs,goroutine}.pprof
```

直接对插件 PID 发 SIGUSR1 也能单独 dump 插件,行为一致。

## 不做的事

- 不新增 REST API、不改 proto / ABI、不改 dashboard。
- 不处理 Windows(项目仅部署于 Linux)。
- 不在 dump 期间暂停请求处理;`runtime/pprof` 本身并发安全。
