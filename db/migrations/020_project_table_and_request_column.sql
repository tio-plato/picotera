-- +goose Up
CREATE TABLE project (
  id            SERIAL PRIMARY KEY,
  name          TEXT NOT NULL UNIQUE,
  paths         JSONB NOT NULL DEFAULT '[]'::jsonb,
  first_seen_at TIMESTAMP,
  last_seen_at  TIMESTAMP,
  created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

ALTER TABLE request ADD COLUMN project_id INTEGER;
CREATE INDEX request_project_id_created_at_idx
  ON request (project_id, created_at DESC, id DESC)
  WHERE project_id IS NOT NULL;

-- +goose Down
DROP INDEX IF EXISTS request_project_id_created_at_idx;
ALTER TABLE request DROP COLUMN IF EXISTS project_id;
DROP TABLE IF EXISTS project;
