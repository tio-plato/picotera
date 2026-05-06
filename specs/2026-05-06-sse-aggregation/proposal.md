# SSE 聚合方案研究

前端那个聚合 SSE 的功能写得太一般了，很多情况根本不兼容。不要继续自己手写维护这套聚合逻辑，研究换成第三方方案，例如 Vercel AI SDK 或 axonhub。

目标是聚合后的 JSON 尽量和上游 non-streaming 的格式一样；如果做不到，也可以接受类似 Vercel AI SDK 的 ModelMessage / UI message 格式。也可以用 Go 在后端响应完成后做聚合，再给前端展示。
