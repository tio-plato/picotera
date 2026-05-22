-- +goose NO TRANSACTION
-- +goose Up
CREATE MATERIALIZED VIEW request_speed_hourly
WITH (timescaledb.continuous) AS
SELECT
  time_bucket(INTERVAL '1 hour', created_at) AS bucket_at,
  model,
  upstream_model,
  provider_id,
  api_key_id,
  project_id,
  SUM(input_tokens::float8) AS prefill_token_sum,
  SUM(ttft_ms::float8) AS prefill_time_sum,
  SUM(output_tokens::float8) AS decode_token_sum,
  SUM((time_spent_ms - ttft_ms)::float8) AS decode_time_sum
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
