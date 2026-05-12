-- +goose Up
ALTER TABLE provider ADD COLUMN proxy_url TEXT;

-- +goose Down
ALTER TABLE provider DROP COLUMN proxy_url;
