# Proposal: ProviderEndpoint CRUD API

为 `provider_endpoint` 表增加 CRUD 接口，具体包括：

1. **根据渠道查询所有 endpoint** — 给定 `provider_id`，返回该渠道关联的所有 endpoint 列表
2. **Upsert 渠道的某个 endpoint** — 给定 `provider_id` + `endpoint_id`，插入或更新关联记录（含 `upstream_url`）
3. **删除某个渠道的某个 endpoint** — 给定 `provider_id` + `endpoint_id`，删除关联记录

不需要分页功能。
