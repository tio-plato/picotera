package server

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"

	"picotera/pkg/contract"
	"picotera/pkg/db"

	"github.com/danielgtaylor/huma/v2"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

func generateApiKey() (string, error) {
	var buf [16]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return "", err
	}
	return "sk_pt_" + hex.EncodeToString(buf[:]), nil
}

func marshalAnnotations(a map[string]string) ([]byte, error) {
	if a == nil {
		a = map[string]string{}
	}
	return json.Marshal(a)
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return true
	}
	// Defensive fallback for adapters that wrap the SQLSTATE differently.
	return strings.Contains(err.Error(), "23505")
}

func (s *Server) handleListApiKeys(ctx context.Context, _ *struct{}) (*contract.ListApiKeysResponse, error) {
	rows, err := s.queries.ListApiKeys(ctx)
	if err != nil {
		return nil, huma.Error500InternalServerError("failed to list api keys", err)
	}
	out := make([]contract.ApiKeyView, len(rows))
	for i := range rows {
		v, err := contract.ToApiKeyView(&rows[i])
		if err != nil {
			return nil, huma.Error500InternalServerError("failed to decode api key", err)
		}
		out[i] = *v
	}
	return &contract.ListApiKeysResponse{Body: out}, nil
}

func (s *Server) handleGetApiKey(ctx context.Context, in *contract.GetApiKeyRequest) (*contract.GetApiKeyResponse, error) {
	r, err := s.queries.GetApiKey(ctx, in.ID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, huma.Error404NotFound("api key not found")
		}
		return nil, huma.Error500InternalServerError("failed to get api key", err)
	}
	v, err := contract.ToApiKeyView(&r)
	if err != nil {
		return nil, huma.Error500InternalServerError("failed to decode api key", err)
	}
	return &contract.GetApiKeyResponse{Body: *v}, nil
}

func (s *Server) handleCreateApiKey(ctx context.Context, in *contract.CreateApiKeyRequest) (*contract.CreateApiKeyResponse, error) {
	key := strings.TrimSpace(in.Body.Key)
	if key == "" {
		gen, err := generateApiKey()
		if err != nil {
			return nil, huma.Error500InternalServerError("failed to generate api key", err)
		}
		key = gen
	}
	annotations, err := marshalAnnotations(in.Body.Annotations)
	if err != nil {
		return nil, huma.Error500InternalServerError("failed to encode annotations", err)
	}
	r, err := s.queries.InsertApiKey(ctx, db.InsertApiKeyParams{
		Name:        in.Body.Name,
		Key:         key,
		Disabled:    in.Body.Disabled,
		Annotations: annotations,
	})
	if err != nil {
		if isUniqueViolation(err) {
			return nil, huma.Error409Conflict("key already exists")
		}
		return nil, huma.Error500InternalServerError("failed to create api key", err)
	}
	v, err := contract.ToApiKeyView(&r)
	if err != nil {
		return nil, huma.Error500InternalServerError("failed to decode api key", err)
	}
	return &contract.CreateApiKeyResponse{Body: *v}, nil
}

func (s *Server) handleUpdateApiKey(ctx context.Context, in *contract.UpdateApiKeyRequest) (*contract.UpdateApiKeyResponse, error) {
	key := strings.TrimSpace(in.Body.Key)
	if key == "" {
		gen, err := generateApiKey()
		if err != nil {
			return nil, huma.Error500InternalServerError("failed to generate api key", err)
		}
		key = gen
	}
	annotations, err := marshalAnnotations(in.Body.Annotations)
	if err != nil {
		return nil, huma.Error500InternalServerError("failed to encode annotations", err)
	}
	r, err := s.queries.UpdateApiKey(ctx, db.UpdateApiKeyParams{
		ID:          in.ID,
		Name:        in.Body.Name,
		Key:         key,
		Disabled:    in.Body.Disabled,
		Annotations: annotations,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, huma.Error404NotFound("api key not found")
		}
		if isUniqueViolation(err) {
			return nil, huma.Error409Conflict("key already exists")
		}
		return nil, huma.Error500InternalServerError("failed to update api key", err)
	}
	v, err := contract.ToApiKeyView(&r)
	if err != nil {
		return nil, huma.Error500InternalServerError("failed to decode api key", err)
	}
	return &contract.UpdateApiKeyResponse{Body: *v}, nil
}

func (s *Server) handleDeleteApiKey(ctx context.Context, in *contract.DeleteApiKeyRequest) (*struct{}, error) {
	if err := s.queries.DeleteApiKey(ctx, in.Body.ID); err != nil {
		return nil, huma.Error500InternalServerError("failed to delete api key", err)
	}
	return &struct{}{}, nil
}
