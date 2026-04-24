-- +goose Up
ALTER TABLE request ALTER COLUMN api_key_id DROP NOT NULL;

-- +goose Down
ALTER TABLE request ALTER COLUMN api_key_id SET NOT NULL;