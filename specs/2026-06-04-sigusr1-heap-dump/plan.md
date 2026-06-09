# 执行计划:SIGUSR1 触发 Heap Dump

## Step 1: 新增 `pkg/heapdump/` 包

1. 创建 `pkg/heapdump/heapdump.go`:
   - `Write(dir, role string) ([]string, error)`:
     - 生成时间戳 `time.Now().UTC().Format("20060102T150405")`;
     - 写 `picotera-<role>-<ts>-heap.pprof`:先 `runtime.GC()`,再 `pprof.Lookup("heap").WriteTo(f, 0)`;
     - 写 `picotera-<role>-<ts>-allocs.pprof`:`pprof.Lookup("allocs").WriteTo(f, 0)`;
     - 写 `picotera-<role>-<ts>-goroutine.pprof`:`pprof.Lookup("goroutine").WriteTo(f, 0)`;
     - 每个文件 `os.Create` → `WriteTo` → `Close`,任一步失败返回 error(附文件路径上下文),成功返回三个路径。
   - `Install(dir, role string, onDump func())`:
     - `ch := make(chan os.Signal, 1)`、`signal.Notify(ch, syscall.SIGUSR1)`;
     - goroutine 循环:收到信号 → 用 `pkg/logx` 记录开始 → `Write` → 记录文件路径或错误 → `onDump != nil` 时调用。
2. 添加 `pkg/heapdump/heapdump_test.go`:对 `Write` 写临时目录做单测——返回三个存在且非空的文件,文件名匹配 `picotera-<role>-<ts>-{heap,allocs,goroutine}.pprof` 模式。

## Step 2: 配置项

1. `pkg/configx/`:`Config` 结构新增 `HeapDumpDir string \`mapstructure:"heap_dump_dir"\``。
2. `Parse()` 中 `viper.SetDefault("heap_dump_dir", os.TempDir())`。

## Step 3: Bridge 接口新增 `SignalPlugin`

1. `pkg/llmbridge/client.go`:`Bridge` 接口新增 `SignalPlugin(sig syscall.Signal) error`;`disabledBridge` 实现为返回 nil。
2. `pkg/llmbridge/plugin_client.go`:`pluginBridge.SignalPlugin`:
   - 持 `b.mu`;
   - `b.client == nil || b.client.Exited()` → 记 debug 日志并返回 nil(不重启插件);
   - `rc := b.client.ReattachConfig()`;`rc == nil || rc.Pid == 0` → 同样跳过;
   - `syscall.Kill(rc.Pid, sig)`,错误原样返回。
3. `llmbridge.Config` 新增 `HeapDumpDir string`;`startPlugin` 中对 `exec.Command` 设置 `cmd.Env = append(os.Environ(), "PICOTERA_HEAP_DUMP_DIR="+cfg.HeapDumpDir)`。

## Step 4: 主进程接线

1. `pkg/server/server.go`:
   - `NewServer` 构造 `llmbridge.Config` 时填入 `HeapDumpDir: config.HeapDumpDir`;
   - `Serve()` 在 `ListenAndServe` 之前调用:
     ```go
     heapdump.Install(s.config.HeapDumpDir, "host", func() {
         if err := s.llmBridge.SignalPlugin(syscall.SIGUSR1); err != nil {
             logrus.WithError(err).Warn("failed to forward SIGUSR1 to llmbridge plugin")
         }
     })
     ```

## Step 5: 插件子进程接线

1. `cmd/picotera-llmbridge-plugin/main.go`:`main()` 开头:
   ```go
   dumpDir := os.Getenv("PICOTERA_HEAP_DUMP_DIR")
   if dumpDir == "" {
       dumpDir = os.TempDir()
   }
   heapdump.Install(dumpDir, "plugin", nil)
   ```
   插件内不写额外日志代码:`heapdump.Install` 内部用 logx(logrus)记录,logrus 默认输出 stderr,在插件进程中会被主进程的 `pluginLogWriter` 收集汇入网关日志。

## Step 6: 验证

1. `go build ./...`、`go test ./pkg/heapdump/ ./pkg/llmbridge/ ./pkg/server/`。
2. 手工验证:`mise run server` 启动后:
   - `kill -USR1 $(pgrep -f 'cmd/picotera/main.go' | head -1)`(或编译后的 picotera 进程 PID);
   - 确认输出目录出现 `picotera-host-*-{heap,allocs,goroutine}.pprof` 与 `picotera-plugin-*-{heap,allocs,goroutine}.pprof` 共 6 个文件;
   - `go tool pprof -top <heap 文件>` 能正常解析;
   - 杀掉插件子进程后再次发信号,确认主进程仍正常 dump 且日志记录插件被跳过。
