-- +goose Up
ALTER TABLE request ADD COLUMN upstream_model TEXT;

-- +goose Down
ALTER TABLE request DROP COLUMN upstream_model;
