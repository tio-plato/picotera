-- +goose Up
CREATE TABLE exchange_rate (
  code TEXT PRIMARY KEY,
  name TEXT NOT NULL,
  symbol TEXT NOT NULL,
  units_per_usd NUMERIC NOT NULL
);

INSERT INTO exchange_rate (code, name, symbol, units_per_usd) VALUES ('USD', 'US Dollar', '$', 1);

ALTER TABLE model ADD COLUMN pricing JSONB NOT NULL DEFAULT '{}'::jsonb;

ALTER TABLE request ADD COLUMN model_cost NUMERIC(20, 6);
ALTER TABLE request ADD COLUMN model_cost_currency TEXT;
ALTER TABLE request ADD COLUMN upstream_cost NUMERIC(20, 6);
ALTER TABLE request ADD COLUMN upstream_cost_currency TEXT;

-- +goose Down
ALTER TABLE request DROP COLUMN upstream_cost_currency;
ALTER TABLE request DROP COLUMN upstream_cost;
ALTER TABLE request DROP COLUMN model_cost_currency;
ALTER TABLE request DROP COLUMN model_cost;
ALTER TABLE model DROP COLUMN pricing;
DROP TABLE exchange_rate;
