-- +goose Up
CREATE EXTENSION IF NOT EXISTS timescaledb CASCADE;

ALTER TABLE request DROP CONSTRAINT request_pkey;
ALTER TABLE request ALTER COLUMN created_at DROP DEFAULT;
ALTER TABLE request ADD PRIMARY KEY (id, created_at);

SELECT create_hypertable('request', by_range('created_at'), migrate_data => true, if_not_exists => true);

CREATE INDEX request_created_at_id_idx ON request (created_at DESC, id DESC);
CREATE INDEX request_parent_span_created_at_idx ON request (parent_span_id, created_at DESC, id DESC)
  WHERE parent_span_id IS NOT NULL AND parent_span_id <> '';
CREATE INDEX request_span_created_at_idx ON request (span_id, created_at ASC, id ASC)
  WHERE span_id IS NOT NULL;

-- +goose Down
CREATE TABLE request_plain (LIKE request INCLUDING DEFAULTS INCLUDING CONSTRAINTS INCLUDING INDEXES);

ALTER TABLE request_plain DROP CONSTRAINT request_plain_pkey;
ALTER TABLE request_plain ADD PRIMARY KEY (id);
ALTER TABLE request_plain ALTER COLUMN created_at SET DEFAULT CURRENT_TIMESTAMP;

INSERT INTO request_plain SELECT * FROM request;

DROP TABLE request;
ALTER TABLE request_plain RENAME TO request;
