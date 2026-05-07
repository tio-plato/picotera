-- +goose Up
ALTER TABLE request ADD COLUMN cache_write_1h_tokens INTEGER;

-- +goose Down
ALTER TABLE request DROP COLUMN cache_write_1h_tokens;
