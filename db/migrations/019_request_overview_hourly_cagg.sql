-- +goose Up
CREATE MATERIALIZED VIEW request_overview_hourly
WITH (timescaledb.continuous) AS
SELECT
  time_bucket('1 hour', created_at) AS bucket_at,
  api_key_id,
  model,
  upstream_model,
  provider_id,
  upstream_cost_currency,
  COUNT(*)::bigint AS request_count,
  SUM(
    COALESCE(input_tokens, 0)
    + COALESCE(cache_read_tokens, 0)
    + COALESCE(output_tokens, 0)
    + COALESCE(cache_write_tokens, 0)
    + COALESCE(cache_write_1h_tokens, 0)
  )::bigint AS total_tokens,
  SUM(COALESCE(input_tokens, 0))::bigint AS input_tokens,
  SUM(COALESCE(cache_read_tokens, 0))::bigint AS cache_read_tokens,
  SUM(COALESCE(output_tokens, 0))::bigint AS output_tokens,
  SUM(COALESCE(cache_write_tokens, 0))::bigint AS cache_write_tokens,
  SUM(COALESCE(cache_write_1h_tokens, 0))::bigint AS cache_write_1h_tokens,
  SUM(upstream_cost)::numeric(20, 6) AS upstream_cost
FROM request
WHERE type = 1
GROUP BY bucket_at, api_key_id, model, upstream_model, provider_id, upstream_cost_currency
WITH NO DATA;

ALTER MATERIALIZED VIEW request_overview_hourly
SET (timescaledb.materialized_only = false);

SELECT add_continuous_aggregate_policy(
  'request_overview_hourly',
  start_offset => INTERVAL '35 days',
  end_offset => INTERVAL '5 minutes',
  schedule_interval => INTERVAL '5 minutes'
);

-- +goose Down
SELECT remove_continuous_aggregate_policy('request_overview_hourly', if_exists => true);
DROP MATERIALIZED VIEW request_overview_hourly;
