# Proposal — Traces Table

搞一个新的表叫做 traces 用来服务追踪功能，每当收到请求要插入 parent_span_id 的时候，往这个表里也 upsert 一下，记录一下这个 span id 所对应的最早、最晚时间，这样一来，当我们查询追踪的时候，就可以直接从这个表查；当我们要根据 parent span id 去查请求的时候，也可以知道 created at 的范围。同时给每个追踪也分配一个内部 id，追踪倒查请求的时候，用这个 id 去倒查。
