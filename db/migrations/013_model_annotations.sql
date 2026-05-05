-- +goose Up
ALTER TABLE model ADD COLUMN annotations JSONB NOT NULL DEFAULT '{}'::jsonb;

-- +goose Down
ALTER TABLE model DROP COLUMN annotations;
