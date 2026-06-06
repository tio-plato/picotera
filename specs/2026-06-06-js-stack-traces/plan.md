# 执行计划：JS 脚本错误栈追踪

## 1. 扩展本地 QuickJS fork

1. 在 `third_party/quickjs/quickjs.go` 中新增 filename 参数辅助函数，负责把非空 filename 转为 C 字符串并释放。
2. 新增 `EvalFile`，实现与现有 `Eval` 相同的返回语义，但把 filename 传给 `XJS_Eval`。
3. 新增 `EvalValueFile`，实现与现有 `EvalValue` 相同的 `Value` 生命周期和异常处理，但把 filename 传给 `XJS_Eval`。
4. 新增 `CompileFile`，复用 compile-only bytecode 写出逻辑，把 filename 传给 `XJS_Eval`。
5. 修改 `errFromException`：读取异常对象的 `stack` 属性；stack 非空时把它追加到 Go error。
6. 保留 `Eval`、`EvalValue`、`Compile` 的公开签名，让它们委托到新方法并继续使用 `<eval>`。
7. 在 quickjs fork 测试中新增用例：
   - `EvalFile` 执行 `throw new Error("boom")`，验证错误包含自定义 filename 和行列。
   - `EvalValueFile` 执行会抛错的 IIFE，验证错误包含自定义 filename 和行列。
   - `CompileFile` 编译非法 JS，验证语法错误包含自定义 filename 和行列。

## 2. 接入 `pkg/jsx`

1. 在 `pkg/jsx` 新增来源名 helper：
   - `scriptFilename(id string) (string, error)`：空 ID 返回错误。
   - `internalFilename(name string) string`：内部调用点使用固定常量名。
2. `newSession` 改为：
   - SDK 用 `EvalFile(sdkSource, "internal:sdk.js", EvalGlobal)`。
   - ctx 初始化用 `EvalFile(ctxInit, "internal:ctx-init.js", EvalGlobal)`。
   - 每个用户脚本先校验非空 ID，再用 `EvalFile(sc.Source, "script:"+sc.ID, EvalGlobal)`。
3. `PatchContext` 改用 `EvalFile(..., "internal:patch-context.js", EvalGlobal)`。
4. `evalJSON` 增加 filename 参数并用 `EvalValueFile`。
5. 所有 `Run*` 方法调用 `evalJSON` 时传入对应内部 hook filename。
6. `ValidateSyntax` 改为通过 `CompileFile` 设置 validation filename；新增未导出的 `validateSyntaxFile` 供测试用真实来源名覆盖。

## 3. 锁定 PicoTera 行为

1. 在 `pkg/jsx/engine_test.go` 增加加载期错误测试：
   - 构造脚本 ID `script-load-fail`，source 中第 2 行语法错误。
   - `NewSession` 返回错误。
   - 断言错误字符串包含 `eval script script-load-fail`、`script:script-load-fail` 和行号信息。
2. 在 `pkg/jsx/engine_test.go` 增加运行期错误测试：
   - 构造脚本 ID `script-runtime-fail`，tap 内调用第 3 行定义的函数并抛错。
   - 调用 `RunRewriteModel`。
   - 断言错误字符串包含 `hook name: <tap name>`、`script:script-runtime-fail` 和对应行号。
3. 在 `pkg/jsx/validate_test.go` 增加 validation filename 测试，验证语法错误包含 `script:<validation>` 和行号信息。
4. 现有 `TestSession_BadScript_FailsSession` 保留，并加强断言。

## 4. 验证

1. 运行 `go test ./third_party/quickjs`。
2. 运行 `go test ./pkg/jsx`。
3. 运行 `go test ./pkg/server`，确认 gateway 调用 JS hook 的错误传播未被破坏。
4. 运行 `go test ./...`；如果外部依赖或环境导致全量测试失败，记录失败包和原因。

## 5. 完成标准

1. 用户脚本加载失败时，错误文本包含 `script:<script id>` 与 QuickJS 行列号。
2. hook 运行失败时，错误文本同时包含 hook tap 名、`script:<script id>` 与 QuickJS 行列号。
3. PicoTera 内部 JS 片段错误显示 `internal:<name>`，不会伪装成用户脚本错误。
4. 没有新增兼容层、输入容错、数据库迁移、管理 API 变更或 dashboard 变更。
