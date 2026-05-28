-- +goose NO TRANSACTION
-- +goose Up
SELECT remove_continuous_aggregate_policy('request_speed_hourly', if_exists => true);
DROP MATERIALIZED VIEW IF EXISTS request_speed_hourly;

CREATE MATERIALIZED VIEW request_speed_hourly
WITH (timescaledb.continuous) AS
SELECT
  time_bucket(INTERVAL '1 hour', created_at) AS bucket_at,
  model,
  upstream_model,
  provider_id,
  api_key_id,
  project_id,
  SUM(CASE
    WHEN input_tokens >= 50 AND ttft_ms >= 500
    THEN input_tokens::float8
  END) AS prefill_token_sum,
  SUM(CASE
    WHEN input_tokens >= 50 AND ttft_ms >= 500
    THEN ttft_ms::float8
  END) AS prefill_time_sum,
  COUNT(CASE
    WHEN input_tokens >= 50 AND ttft_ms >= 500
    THEN 1
  END) AS prefill_request_count,
  SUM(CASE
    WHEN output_tokens >= 50
      AND ttft_ms IS NOT NULL
      AND time_spent_ms IS NOT NULL
      AND (time_spent_ms - ttft_ms) >= 500
    THEN output_tokens::float8
  END) AS decode_token_sum,
  SUM(CASE
    WHEN output_tokens >= 50
      AND ttft_ms IS NOT NULL
      AND time_spent_ms IS NOT NULL
      AND (time_spent_ms - ttft_ms) >= 500
    THEN (time_spent_ms - ttft_ms)::float8
  END) AS decode_time_sum
FROM request
WHERE type = 1
GROUP BY bucket_at, model, upstream_model, provider_id, api_key_id, project_id
WITH NO DATA;

ALTER MATERIALIZED VIEW request_speed_hourly
  SET (timescaledb.materialized_only = false);

SELECT add_continuous_aggregate_policy(
  'request_speed_hourly',
  start_offset      => INTERVAL '35 days',
  end_offset        => INTERVAL '5 minutes',
  schedule_interval => INTERVAL '5 minutes'
);

-- +goose Down
SELECT remove_continuous_aggregate_policy('request_speed_hourly', if_exists => true);
DROP MATERIALIZED VIEW IF EXISTS request_speed_hourly;

CREATE MATERIALIZED VIEW request_speed_hourly
WITH (timescaledb.continuous) AS
SELECT
  time_bucket(INTERVAL '1 hour', created_at) AS bucket_at,
  model,
  upstream_model,
  provider_id,
  api_key_id,
  project_id,
  SUM(CASE
    WHEN input_tokens >= 50 AND ttft_ms >= 500
    THEN input_tokens::float8
  END) AS prefill_token_sum,
  SUM(CASE
    WHEN input_tokens >= 50 AND ttft_ms >= 500
    THEN ttft_ms::float8
  END) AS prefill_time_sum,
  SUM(CASE
    WHEN output_tokens >= 50
      AND ttft_ms IS NOT NULL
      AND time_spent_ms IS NOT NULL
      AND (time_spent_ms - ttft_ms) >= 500
    THEN output_tokens::float8
  END) AS decode_token_sum,
  SUM(CASE
    WHEN output_tokens >= 50
      AND ttft_ms IS NOT NULL
      AND time_spent_ms IS NOT NULL
      AND (time_spent_ms - ttft_ms) >= 500
    THEN (time_spent_ms - ttft_ms)::float8
  END) AS decode_time_sum
FROM request
WHERE type = 1
GROUP BY bucket_at, model, upstream_model, provider_id, api_key_id, project_id
WITH NO DATA;

ALTER MATERIALIZED VIEW request_speed_hourly
  SET (timescaledb.materialized_only = false);

SELECT add_continuous_aggregate_policy(
  'request_speed_hourly',
  start_offset      => INTERVAL '35 days',
  end_offset        => INTERVAL '5 minutes',
  schedule_interval => INTERVAL '5 minutes'
);
