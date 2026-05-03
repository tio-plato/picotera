package server

import (
	"context"
	"errors"
	"picotera/pkg/contract"

	"github.com/danielgtaylor/huma/v2"
	"github.com/jackc/pgx/v5"
)

func (s *Server) handleListExchangeRates(ctx context.Context, _ *struct{}) (*contract.ListExchangeRatesResponse, error) {
	rates, err := s.queries.GetExchangeRates(ctx)
	if err != nil {
		return nil, huma.Error500InternalServerError("failed to list exchange rates", err)
	}
	views := make([]contract.ExchangeRateView, len(rates))
	for i := range rates {
		v, err := contract.ToExchangeRateView(&rates[i])
		if err != nil {
			return nil, huma.Error500InternalServerError("failed to convert exchange rate", err)
		}
		views[i] = *v
	}
	return &contract.ListExchangeRatesResponse{Body: views}, nil
}

func (s *Server) handleGetExchangeRate(ctx context.Context, input *contract.GetExchangeRateRequest) (*contract.GetExchangeRateResponse, error) {
	rate, err := s.queries.GetExchangeRateByCode(ctx, input.Code)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, huma.Error404NotFound("exchange rate not found")
		}
		return nil, huma.Error500InternalServerError("failed to get exchange rate", err)
	}
	v, err := contract.ToExchangeRateView(&rate)
	if err != nil {
		return nil, huma.Error500InternalServerError("failed to convert exchange rate", err)
	}
	return &contract.GetExchangeRateResponse{Body: *v}, nil
}

func (s *Server) handlePutExchangeRate(ctx context.Context, input *contract.PutExchangeRateRequest) (*contract.GetExchangeRateResponse, error) {
	if input.Body.Code == "" {
		return nil, huma.Error400BadRequest("code is required")
	}
	if input.Body.UnitsPerUsd <= 0 {
		return nil, huma.Error400BadRequest("unitsPerUsd must be greater than 0")
	}
	params, err := contract.FromExchangeRateView(&input.Body)
	if err != nil {
		return nil, huma.Error500InternalServerError("failed to encode exchange rate", err)
	}
	rate, err := s.queries.UpsertExchangeRate(ctx, params)
	if err != nil {
		return nil, huma.Error500InternalServerError("failed to upsert exchange rate", err)
	}
	v, err := contract.ToExchangeRateView(&rate)
	if err != nil {
		return nil, huma.Error500InternalServerError("failed to convert exchange rate", err)
	}
	return &contract.GetExchangeRateResponse{Body: *v}, nil
}

func (s *Server) handleDeleteExchangeRate(ctx context.Context, input *contract.DeleteExchangeRateRequest) (*struct{}, error) {
	if input.Body.Code == "USD" {
		return nil, huma.Error400BadRequest("cannot delete base currency")
	}
	if err := s.queries.DeleteExchangeRate(ctx, input.Body.Code); err != nil {
		return nil, huma.Error500InternalServerError("failed to delete exchange rate", err)
	}
	return &struct{}{}, nil
}
