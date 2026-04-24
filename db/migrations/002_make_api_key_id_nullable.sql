-- +goose Up
-- v1 gateway skips API key auth; api_key_id will be NULL for gateway-routed requests
ALTER TABLE request ALTER COLUMN api_key_id DROP NOT NULL;

-- +goose Down
ALTER TABLE request ALTER COLUMN api_key_id SET NOT NULL;
