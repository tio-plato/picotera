# 设计：JS 脚本错误栈追踪

## 目标

JS 脚本加载失败和 hook 运行失败时，错误信息必须包含用户脚本的稳定来源名与 QuickJS 计算出的行列号。来源名使用脚本 ID，格式为 `script:<id>`。PicoTera 内置 SDK、ctx 初始化、context patch、hook 包装表达式使用 `internal:<name>`，让内部错误和用户脚本错误可以直接区分。

## QuickJS 文件名能力

当前 `third_party/quickjs` 的 `Eval`、`EvalValue`、`Compile` 都把 QuickJS filename 参数硬编码为 `<eval>`，导致 `Error.stack` 中的 frame 不可定位。该本地 fork 已通过 `go.mod` replace 接入 PicoTera，因此在 fork 中新增显式命名 API：

```go
func (m *VM) EvalFile(javascript, filename string, flags int) (any, error)
func (m *VM) EvalValueFile(javascript, filename string, flags int) (Value, error)
func (m *VM) CompileFile(javascript, filename string, flags int) ([]byte, error)
```

现有 `Eval`、`EvalValue`、`Compile` 保持原签名，并委托到这些新方法，filename 仍为 `<eval>`。这是第三方 fork 的 API 扩展，不是 PicoTera 的运行时兼容层。

实现要求：

- filename 通过 `libc.CString` 转成 C 字符串并传给 `XJS_Eval` / `XJS_EvalThis`。
- filename 为空时拒绝并返回明确错误，不静默回退到 `<eval>`。
- `CompileFile` 走 compile-only 路径，返回的 syntax error 必须带 filename 和行列号。
- `errFromException` 读取异常对象的 `stack` 属性；存在非空 stack 时，Go error 使用
  `<exception string>\n<stack>`，保证直接抛出的 runtime error 也带来源定位。
- 新增 quickjs fork 单元测试，覆盖 `EvalFile` 和 `EvalValueFile` 的 runtime stack，以及 `CompileFile` 的语法错误定位。

## PicoTera 命名策略

在 `pkg/jsx` 增加一个小的命名求值封装，所有 JS 求值都走它：

```go
func scriptFilename(id string) (string, error)
func internalFilename(name string) string
```

`scriptFilename` 对脚本 ID 做严格校验，只接受非空 ID。脚本 ID 由数据库生成并在管理 API 中作为主键使用；遇到空 ID 说明数据或测试夹具无效，session 创建直接失败。不要对 ID 做 trim、大小写转换、替换字符、slug 化或容错猜测。

求值来源名分配：

| 代码来源 | filename |
| --- | --- |
| `pkg/jsx/sdk.js` | `internal:sdk.js` |
| `ctxInit` | `internal:ctx-init.js` |
| `PatchContext` 的 `Object.assign(...)` | `internal:patch-context.js` |
| `RunRewriteModel` 包装表达式 | `internal:hook-rewriteModel.js` |
| `RunSortProviders` 包装表达式 | `internal:hook-sortProviders.js` |
| `RunBeforeRequest` 包装表达式 | `internal:hook-beforeRequest.js` |
| `RunRewriteRequest` 包装表达式 | `internal:hook-rewriteRequest.js` |
| `RunBeforeTransform` 包装表达式 | `internal:hook-beforeTransform.js` |
| `RunRewriteProviderModels` 包装表达式 | `internal:hook-rewriteProviderModels.js` |
| 用户脚本 | `script:<script.ID>` |

## 错误传播

`sdk.js` 当前在 waterfall 中捕获 tap 异常并抛出新的 `Error`，内容包含 `hook name` 和原始 `error.stack`。保留这个行为，因为它能把 tap 名和用户脚本 stack 一起带回 Go。命名求值生效后，用户脚本中的 frame 会显示 `script:<id>:<line>:<column>`。

Go 侧不解析或重写 stack 文本。`newSession` 的加载错误继续返回 `jsx: eval script <id>: <quickjs error>`，QuickJS 错误本身必须含 `script:<id>` 定位信息。hook 运行错误继续由 `evalJSON` 返回 `jsx: <hook>: <quickjs error>`，QuickJS 错误正文中保留 SDK 包装出的 hook name 与原始 stack。

## 语法校验

`ValidateSyntax` 增加带来源名的内部实现：

```go
func ValidateSyntax(source string) error
func validateSyntaxFile(source, filename string) error
```

公开的 `ValidateSyntax` 继续用于管理 API 创建 / 更新脚本，使用 `script:<validation>` 作为 filename。session 加载真实脚本时不只依赖预校验，仍用真实 `script:<id>` 执行，这样线上加载错误能准确指向脚本 ID。

## 不改动项

不新增数据库字段、管理 API 字段、dashboard UI 字段或 OpenAPI 变更。该功能只改变错误文本中的来源名和行列号。
