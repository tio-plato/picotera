-- +goose Up
ALTER TABLE project ADD COLUMN auto_created BOOLEAN NOT NULL DEFAULT false;

-- +goose Down
ALTER TABLE project DROP COLUMN IF EXISTS auto_created;
