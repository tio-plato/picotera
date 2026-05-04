-- +goose Up
ALTER TABLE provider_endpoint
  ADD COLUMN credentials_resolver INTEGER NOT NULL DEFAULT 0;

-- +goose Down
ALTER TABLE provider_endpoint DROP COLUMN credentials_resolver;
