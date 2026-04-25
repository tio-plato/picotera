# Proposal — Request Artifacts

为请求增加 artifacts 功能。

1. 完整记录原始 request 和 response：对元请求，记录的是客户端发送和收到的 header 与 body；对上游请求，记录的是我们发送的和收到的 header 与 body。
2. 记录之后序列化为 json，然后用 zstd 压缩，用 minio sdk 按固定的文件名模板上传到对象存储，路径中要包含当前日期，方便后续整理；使用 `Content-Encoding: zstd` 上传，请求一个 json、响应一个 json，分别上传。
3. 在列请求的 API 中增加返回 presigned URL 供前端获取对应 artifacts。
4. 前端展示原始请求和响应。
