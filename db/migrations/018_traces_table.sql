-- +goose Up
CREATE TABLE traces (
  id TEXT PRIMARY KEY,
  parent_span_id TEXT NOT NULL UNIQUE,
  first_request_at TIMESTAMP NOT NULL,
  last_request_at TIMESTAMP NOT NULL,
  updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX traces_last_request_at_id_idx ON traces (last_request_at DESC, id DESC);

-- +goose Down
DROP TABLE traces;
