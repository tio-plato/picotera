-- name: GetExchangeRates :many
SELECT * FROM exchange_rate ORDER BY code;

-- name: GetExchangeRateByCode :one
SELECT * FROM exchange_rate WHERE code = $1 LIMIT 1;

-- name: UpsertExchangeRate :one
INSERT INTO exchange_rate (code, name, symbol, units_per_usd)
VALUES ($1, $2, $3, $4)
ON CONFLICT (code) DO UPDATE
  SET name = $2, symbol = $3, units_per_usd = $4
RETURNING *;

-- name: DeleteExchangeRate :exec
DELETE FROM exchange_rate WHERE code = $1;
