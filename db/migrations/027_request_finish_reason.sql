-- +goose Up
ALTER TABLE request ADD COLUMN finish_reason INTEGER;

-- +goose Down
ALTER TABLE request DROP COLUMN IF EXISTS finish_reason;
