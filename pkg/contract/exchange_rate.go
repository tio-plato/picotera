package contract

import (
	"fmt"
	"math/big"
	"net/http"
	"picotera/pkg/db"

	"github.com/danielgtaylor/huma/v2"
	"github.com/jackc/pgx/v5/pgtype"
)

type ExchangeRateView struct {
	Code        string  `json:"code"`
	Name        string  `json:"name"`
	Symbol      string  `json:"symbol"`
	UnitsPerUsd float64 `json:"unitsPerUsd"`
}

type GetExchangeRateRequest struct {
	Code string `path:"code" example:"USD"`
}

type GetExchangeRateResponse struct {
	Body ExchangeRateView
}

type PutExchangeRateRequest struct {
	Body ExchangeRateView
}

type DeleteExchangeRateRequest struct {
	Body struct {
		Code string `json:"code"`
	}
}

type ListExchangeRatesResponse struct {
	Body []ExchangeRateView
}

var OperationListExchangeRates = huma.Operation{
	OperationID: "listExchangeRates",
	Method:      http.MethodGet,
	Path:        "/exchange-rates",
	Summary:     "List all exchange rates",
}

var OperationGetExchangeRate = huma.Operation{
	OperationID: "getExchangeRate",
	Method:      http.MethodGet,
	Path:        "/exchange-rates/{code}",
	Summary:     "Get an exchange rate by code",
}

var OperationPutExchangeRate = huma.Operation{
	OperationID: "putExchangeRate",
	Method:      http.MethodPut,
	Path:        "/exchange-rates",
	Summary:     "Upsert an exchange rate",
}

var OperationDeleteExchangeRate = huma.Operation{
	OperationID: "deleteExchangeRate",
	Method:      http.MethodPost,
	Path:        "/exchange-rates/delete",
	Summary:     "Delete an exchange rate",
}

func ToExchangeRateView(r *db.ExchangeRate) (*ExchangeRateView, error) {
	units, err := numericToFloat(r.UnitsPerUsd)
	if err != nil {
		return nil, fmt.Errorf("exchange_rate %s: %w", r.Code, err)
	}
	return &ExchangeRateView{
		Code:        r.Code,
		Name:        r.Name,
		Symbol:      r.Symbol,
		UnitsPerUsd: units,
	}, nil
}

func FromExchangeRateView(v *ExchangeRateView) (db.UpsertExchangeRateParams, error) {
	num, err := floatToNumeric(v.UnitsPerUsd)
	if err != nil {
		return db.UpsertExchangeRateParams{}, err
	}
	return db.UpsertExchangeRateParams{
		Code:        v.Code,
		Name:        v.Name,
		Symbol:      v.Symbol,
		UnitsPerUsd: num,
	}, nil
}

func numericToFloat(n pgtype.Numeric) (float64, error) {
	if !n.Valid {
		return 0, nil
	}
	f, err := n.Float64Value()
	if err != nil {
		return 0, err
	}
	if !f.Valid {
		return 0, nil
	}
	return f.Float64, nil
}

func floatToNumeric(f float64) (pgtype.Numeric, error) {
	if f == 0 {
		return pgtype.Numeric{Int: big.NewInt(0), Exp: 0, Valid: true}, nil
	}
	var n pgtype.Numeric
	if err := n.Scan(fmt.Sprintf("%v", f)); err != nil {
		return pgtype.Numeric{}, err
	}
	return n, nil
}
