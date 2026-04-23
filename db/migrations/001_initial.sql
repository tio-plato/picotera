-- +goose Up
-- provider is a inference provider of the API
CREATE TABLE provider (
  id SERIAL PRIMARY KEY,
  name TEXT NOT NULL,
  credentials TEXT NOT NULL,
  priority INTEGER NOT NULL,
  provider_models JSONB NOT NULL, -- models supported by the provider
  annotations JSONB NOT NULL
);

-- endpoint is a user-facing endpoint that can be used to access the API
CREATE TABLE endpoint (
  path TEXT PRIMARY KEY, -- path component of the accessing URL. It should be a relative path, like /api/v1/chat/completions
  name TEXT NOT NULL,
  model_path TEXT NOT NULL, -- model field path in the request body
  credentials_resolver INTEGER NOT NULL
);

-- provider_endpoint indicates that a provider's endpoint is available for a given endpoint
CREATE TABLE provider_endpoint (
  provider_id INTEGER NOT NULL,
  endpoint_path TEXT NOT NULL,
  upstream_url TEXT NOT NULL,
  PRIMARY KEY (provider_id, endpoint_path)
);

-- model is a model that can be used to access and route requests to the API
CREATE TABLE model (
  name TEXT PRIMARY KEY,
  title TEXT, -- human readable model name
  developer TEXT,
  series TEXT
);

-- model_provider_endpoint indicates that a model is available on a provider's endpoint
CREATE TABLE model_provider_endpoint (
  model_name TEXT NOT NULL,
  provider_id INTEGER NOT NULL,
  endpoint_path TEXT NOT NULL,
  upstream_model_name TEXT, -- model name in the upstream API, if different from the model name
  priority INTEGER NOT NULL,
  annotations JSONB NOT NULL,
  PRIMARY KEY (model_name, provider_id, endpoint_path)
);

-- api_key is a downstream API key for a given provider
CREATE TABLE api_key (
  id SERIAL PRIMARY KEY,
  name TEXT NOT NULL,
  api_key_hash BYTEA NOT NULL UNIQUE,
  api_key_masked TEXT NOT NULL,
  annotations JSONB NOT NULL
);

-- request records a request to the API
CREATE TABLE request (
  id TEXT PRIMARY KEY,
  span_id TEXT,
  parent_span_id TEXT,
  provider_id INTEGER NOT NULL,
  -- request metadata
  endpoint_path TEXT NOT NULL,
  api_key_id INTEGER NOT NULL,
  model TEXT,
  -- response metadata
  input_tokens INTEGER,
  cache_read_tokens INTEGER,
  output_tokens INTEGER,
  cache_write_tokens INTEGER,
  status_code INTEGER NOT NULL,
  error_message TEXT,
  -- performance metrics
  ttft_ms INTEGER,
  time_spent_ms INTEGER NOT NULL,
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- +goose Down
DROP TABLE request;
DROP TABLE api_key;
DROP TABLE model_provider_endpoint;
DROP TABLE model;
DROP TABLE provider_endpoint;
DROP TABLE endpoint;
DROP TABLE provider;
