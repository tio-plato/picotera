-- +goose Up
ALTER TABLE model DROP COLUMN title;
ALTER TABLE model DROP COLUMN developer;
ALTER TABLE model DROP COLUMN series;

-- +goose Down
ALTER TABLE model ADD COLUMN series TEXT;
ALTER TABLE model ADD COLUMN developer TEXT;
ALTER TABLE model ADD COLUMN title TEXT;
