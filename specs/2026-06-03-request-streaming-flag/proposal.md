# 需求：请求 Streaming 标识与非流式请求的 header 超时豁免

给网关请求（path-based gateway）和 unified 请求的上下文都增加一个 `Streaming bool` 字段，用来标识这个请求是否是流式的。

识别方法（按顺序）：

1. gemini stream 端点是流式的；
2. 请求 body 里 `stream = true` 是流式的；
3. `Accept` 里有 `text/event-stream` 是流式的；
4. `Accept` 里有 `application/x-ndjson` 是流式的；
5. 其它的都是非流式的。

然后，对非流式的请求，不应用 read response headers timeout（`GatewayResponseHeaderTimeout`）。
