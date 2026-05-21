-- +goose Up
ALTER TABLE provider ADD COLUMN supports_native_web_search BOOLEAN NOT NULL DEFAULT FALSE;

-- +goose Down
ALTER TABLE provider DROP COLUMN supports_native_web_search;
