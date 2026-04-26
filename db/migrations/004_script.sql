-- +goose Up
CREATE TABLE script (
  id TEXT PRIMARY KEY,
  name TEXT NOT NULL,
  source TEXT NOT NULL,
  enabled BOOLEAN NOT NULL DEFAULT TRUE,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX script_enabled_idx ON script (id) WHERE enabled = TRUE;

-- +goose Down
DROP INDEX IF EXISTS script_enabled_idx;
DROP TABLE script;
