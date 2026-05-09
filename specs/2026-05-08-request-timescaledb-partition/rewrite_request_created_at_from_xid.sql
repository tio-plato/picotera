-- One-time pre-migration data rewrite for request.created_at.
--
-- Run this manually before applying the TimescaleDB hypertable migration:
--
--   psql "$PICOTERA_DATABASE_URL" \
--     -f specs/2026-05-08-request-timescaledb-partition/rewrite_request_created_at_from_xid.sql
--
-- The script rewrites every historical request.created_at value to the UTC
-- second encoded in request.id, which is an rs/xid string. It fails before
-- writing if any request ID is not a strict lowercase xid-shaped value.

BEGIN;

LOCK TABLE request IN SHARE ROW EXCLUSIVE MODE;

CREATE OR REPLACE FUNCTION pg_temp.request_created_at_from_xid(p_id text)
RETURNS timestamp
LANGUAGE plpgsql
IMMUTABLE
STRICT
AS $$
DECLARE
  alphabet constant text := '0123456789abcdefghijklmnopqrstuv';
  v0 int;
  v1 int;
  v2 int;
  v3 int;
  v4 int;
  v5 int;
  v6 int;
  b0 int;
  b1 int;
  b2 int;
  b3 int;
  unix_seconds bigint;
BEGIN
  IF p_id !~ '^[0-9a-v]{20}$' OR right(p_id, 1) NOT IN ('0', 'g') THEN
    RAISE EXCEPTION 'invalid xid: %', p_id;
  END IF;

  v0 := strpos(alphabet, substr(p_id, 1, 1)) - 1;
  v1 := strpos(alphabet, substr(p_id, 2, 1)) - 1;
  v2 := strpos(alphabet, substr(p_id, 3, 1)) - 1;
  v3 := strpos(alphabet, substr(p_id, 4, 1)) - 1;
  v4 := strpos(alphabet, substr(p_id, 5, 1)) - 1;
  v5 := strpos(alphabet, substr(p_id, 6, 1)) - 1;
  v6 := strpos(alphabet, substr(p_id, 7, 1)) - 1;

  b0 := ((v0 << 3) | (v1 >> 2)) & 255;
  b1 := ((v1 << 6) | (v2 << 1) | (v3 >> 4)) & 255;
  b2 := ((v3 << 4) | (v4 >> 1)) & 255;
  b3 := ((v4 << 7) | (v5 << 2) | (v6 >> 3)) & 255;

  unix_seconds := ((b0::bigint << 24) | (b1::bigint << 16) | (b2::bigint << 8) | b3::bigint);
  RETURN to_timestamp(unix_seconds) AT TIME ZONE 'UTC';
END;
$$;

DO $$
DECLARE
  invalid_count bigint;
  sample_id text;
BEGIN
  SELECT COUNT(*), MIN(id)
  INTO invalid_count, sample_id
  FROM request
  WHERE id !~ '^[0-9a-v]{20}$' OR right(id, 1) NOT IN ('0', 'g');

  IF invalid_count > 0 THEN
    RAISE EXCEPTION 'request.created_at rewrite aborted: found % invalid xid ids, first sample=%', invalid_count, sample_id;
  END IF;
END;
$$;

WITH rewritten AS (
  UPDATE request
  SET created_at = pg_temp.request_created_at_from_xid(id)
  WHERE created_at IS DISTINCT FROM pg_temp.request_created_at_from_xid(id)
  RETURNING 1
)
SELECT COUNT(*) AS rewritten_request_rows
FROM rewritten;

DO $$
DECLARE
  mismatch_count bigint;
BEGIN
  SELECT COUNT(*)
  INTO mismatch_count
  FROM request
  WHERE created_at IS DISTINCT FROM pg_temp.request_created_at_from_xid(id);

  IF mismatch_count > 0 THEN
    RAISE EXCEPTION 'request.created_at rewrite verification failed: % rows still mismatch xid time', mismatch_count;
  END IF;
END;
$$;

COMMIT;
