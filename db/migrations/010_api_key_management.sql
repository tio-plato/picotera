-- +goose Up
ALTER TABLE api_key DROP COLUMN api_key_hash;
ALTER TABLE api_key DROP COLUMN api_key_masked;
ALTER TABLE api_key ADD COLUMN key TEXT NOT NULL;
ALTER TABLE api_key ADD COLUMN disabled BOOLEAN NOT NULL DEFAULT FALSE;
ALTER TABLE api_key ADD COLUMN created_at TIMESTAMPTZ NOT NULL DEFAULT now();
ALTER TABLE api_key ADD COLUMN updated_at TIMESTAMPTZ NOT NULL DEFAULT now();
CREATE UNIQUE INDEX api_key_key_idx ON api_key (key);

-- +goose Down
DROP INDEX IF EXISTS api_key_key_idx;
ALTER TABLE api_key DROP COLUMN updated_at;
ALTER TABLE api_key DROP COLUMN created_at;
ALTER TABLE api_key DROP COLUMN disabled;
ALTER TABLE api_key DROP COLUMN key;
ALTER TABLE api_key ADD COLUMN api_key_masked TEXT NOT NULL DEFAULT '';
ALTER TABLE api_key ADD COLUMN api_key_hash BYTEA NOT NULL DEFAULT '';
ALTER TABLE api_key ALTER COLUMN api_key_masked DROP DEFAULT;
ALTER TABLE api_key ALTER COLUMN api_key_hash DROP DEFAULT;
