-- +goose Up
UPDATE provider SET provider_models = '{}'::jsonb;
DROP TABLE model_provider_endpoint;
CREATE INDEX idx_provider_models_gin
  ON provider USING GIN (provider_models jsonb_path_ops);

-- +goose Down
DROP INDEX idx_provider_models_gin;
CREATE TABLE model_provider_endpoint (
  model_name TEXT NOT NULL,
  provider_id INTEGER NOT NULL,
  endpoint_path TEXT NOT NULL,
  upstream_model_name TEXT,
  priority INTEGER NOT NULL,
  annotations JSONB NOT NULL,
  PRIMARY KEY (model_name, provider_id, endpoint_path)
);
UPDATE provider SET provider_models = '[]'::jsonb;
