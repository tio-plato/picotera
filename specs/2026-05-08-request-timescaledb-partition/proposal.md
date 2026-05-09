# Request TimescaleDB Partitioning

用户希望把 `request` 表升级为分区表并使用 TimescaleDB：

- 使用 `request_id + created_at` 作为双重主键。
- 按 `created_at` 分区。
- 给所有 `request` 表查询增加 `created_at` 限制条件，用作分区裁剪信息。
- 在 Go 中将 xid 解析为时间，自动生成 `created_at`，避免改变用户侧请求。
