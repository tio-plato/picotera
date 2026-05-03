# Proposal — Trace Parent Span

增加追踪功能。提取请求中的 `X-Claude-Code-Session-Id`、`Conversation_id` 这两个头作为 parent_span_id 保存到请求中。
