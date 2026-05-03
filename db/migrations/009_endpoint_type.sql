-- +goose Up
ALTER TABLE endpoint ADD COLUMN endpoint_type INTEGER NOT NULL DEFAULT 1;

-- +goose Down
ALTER TABLE endpoint DROP COLUMN endpoint_type;
