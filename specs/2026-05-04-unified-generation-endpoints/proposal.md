# Unified Generation Endpoints

新增几个接口：

- `/api/picotera/v1/messages`
- `/api/picotera/v1/responses`
- `/api/picotera/v1/chat/completions`

这几个接口接受对应格式的请求，然后检索所有关于生成的三类 endpoints (gemini, openai,
anthropic)，并统一进行排序。当请求的上游与请求路径格式不一样时，使用
<https://github.com/looplj/axonhub/tree/unstable/llm> 这个仓库里的代码，进行转换之后再请求
上游。比如请求的是 `/api/picotera/v1/messages` 然后上游端点是 anthropic messages 类型，那
就和以前一样走 1:1 请求；如果上游端点是别的类型比如 openaiChat 类型，则转换之后再请求，响
应也转换。

转换在原本脚本的各类 hook 包括 `rewriteRequest` 都进行完成之后再执行。
