-- +goose Up
-- Meta requests start without a provider, endpoint, status, or time_spent
ALTER TABLE request ALTER COLUMN provider_id DROP NOT NULL;
ALTER TABLE request ALTER COLUMN endpoint_path DROP NOT NULL;
ALTER TABLE request ALTER COLUMN status_code DROP NOT NULL;
ALTER TABLE request ALTER COLUMN time_spent_ms DROP NOT NULL;
-- Type: 0=meta (client request), 1=upstream (provider request). Default 1 for backward compat.
ALTER TABLE request ADD COLUMN type INTEGER NOT NULL DEFAULT 1;
-- Status: 0=pending, 1=header_received, 2=completed, 3=failed
ALTER TABLE request ADD COLUMN status INTEGER NOT NULL DEFAULT 0;

-- +goose Down
ALTER TABLE request DROP COLUMN status;
ALTER TABLE request DROP COLUMN type;
ALTER TABLE request ALTER COLUMN time_spent_ms SET NOT NULL;
ALTER TABLE request ALTER COLUMN status_code SET NOT NULL;
ALTER TABLE request ALTER COLUMN endpoint_path SET NOT NULL;
ALTER TABLE request ALTER COLUMN provider_id SET NOT NULL;
