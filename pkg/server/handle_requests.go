package server

import (
	"context"
	"errors"
	"picotera/pkg/artifacts"
	"picotera/pkg/contract"
	"picotera/pkg/db"
	"picotera/pkg/errorx"
	"picotera/pkg/logx"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

const artifactPresignTTL = time.Hour

// attachArtifactUrls fills in presigned URLs for the given view using id+createdAt.
// Errors are logged and silently dropped (URL fields stay empty).
func (s *Server) attachArtifactUrls(ctx context.Context, v *contract.RequestView, createdAt pgtype.Timestamp) {
	if s.artifacts == nil || !s.artifacts.Enabled() || !createdAt.Valid {
		return
	}
	ts := createdAt.Time
	reqURL, err := s.artifacts.PresignedGet(ctx, artifacts.RequestKey(v.ID, ts), artifactPresignTTL)
	if err != nil {
		logx.WithContext(ctx).WithError(err).WithField("id", v.ID).Warn("artifact: presign request failed")
	} else {
		v.RequestArtifactUrl = reqURL
	}
	respURL, err := s.artifacts.PresignedGet(ctx, artifacts.ResponseKey(v.ID, ts), artifactPresignTTL)
	if err != nil {
		logx.WithContext(ctx).WithError(err).WithField("id", v.ID).Warn("artifact: presign response failed")
	} else {
		v.ResponseArtifactUrl = respURL
	}
}

func (s *Server) handleListRequests(ctx context.Context, input *contract.ListRequestsRequest) (*contract.ListRequestsResponse, error) {
	limit := input.Limit
	if limit <= 0 {
		limit = 20
	}
	fetchLimit := limit + 1

	var cursorCreatedAt pgtype.Timestamp
	var cursorID pgtype.Text
	if input.Cursor != "" {
		var id string
		if err := contract.DecodeCursor(input.Cursor, "id", &id); err != nil {
			return nil, huma.Error400BadRequest("invalid cursor", err)
		}
		t, err := requestIDCreatedAt(id)
		if err != nil {
			return nil, huma.Error400BadRequest("invalid cursor", err)
		}
		cursorCreatedAt = pgtype.Timestamp{Time: t, Valid: true}
		cursorID = pgtype.Text{String: id, Valid: true}
	}

	var filterType pgtype.Int4
	if input.Type >= 0 {
		filterType = pgtype.Int4{Int32: input.Type, Valid: true}
	}
	var filterProviderID pgtype.Int4
	if input.ProviderID != 0 {
		filterProviderID = pgtype.Int4{Int32: input.ProviderID, Valid: true}
	}
	var filterEndpointPath pgtype.Text
	if input.EndpointPath != "" {
		filterEndpointPath = pgtype.Text{String: input.EndpointPath, Valid: true}
	}
	var filterModel pgtype.Text
	if input.Model != "" {
		filterModel = pgtype.Text{String: input.Model, Valid: true}
	}
	var filterUpstreamModel pgtype.Text
	if input.UpstreamModel != "" {
		filterUpstreamModel = pgtype.Text{String: input.UpstreamModel, Valid: true}
	}
	var filterParentSpanID pgtype.Text
	if input.ParentSpanID != "" {
		filterParentSpanID = pgtype.Text{String: input.ParentSpanID, Valid: true}
	}

	rows, err := s.queries.ListRequests(ctx, db.ListRequestsParams{
		Type:            filterType,
		ProviderID:      filterProviderID,
		EndpointPath:    filterEndpointPath,
		Model:           filterModel,
		UpstreamModel:   filterUpstreamModel,
		ParentSpanID:    filterParentSpanID,
		CursorCreatedAt: cursorCreatedAt,
		CursorID:        cursorID,
		Limit:           pgtype.Int4{Int32: fetchLimit, Valid: true},
	})
	if err != nil {
		return nil, huma.Error500InternalServerError("failed to list requests", err)
	}

	hasMore := int32(len(rows)) > limit
	if hasMore {
		rows = rows[:limit]
	}

	items := make([]contract.RequestView, len(rows))
	for i, row := range rows {
		items[i] = *contract.ToListRequestRowView(&row)
		s.attachArtifactUrls(ctx, &items[i], row.CreatedAt)
	}

	pagination := contract.PaginationInfo{HasMore: hasMore}
	if hasMore && len(rows) > 0 {
		last := rows[len(rows)-1]
		cursor, err := contract.EncodeCursor("id", last.ID)
		if err != nil {
			return nil, huma.Error500InternalServerError("failed to encode cursor", err)
		}
		pagination.NextCursor = cursor
	}

	return &contract.ListRequestsResponse{
		Body: contract.PaginatedBody[contract.RequestView]{
			Items:      items,
			Pagination: pagination,
		},
	}, nil
}

func (s *Server) handleListRequestTraces(ctx context.Context, input *contract.ListRequestTracesRequest) (*contract.ListRequestTracesResponse, error) {
	limit := input.Limit
	if limit <= 0 {
		limit = 20
	}
	fetchLimit := limit + 1

	var cursorLastRequestAt pgtype.Timestamp
	var cursorParentSpanID pgtype.Text
	if input.Cursor != "" {
		var lastRequestAt, parentSpanID string
		if err := contract.DecodeCursor(input.Cursor, "lastRequestAt", &lastRequestAt, "parentSpanId", &parentSpanID); err != nil {
			return nil, huma.Error400BadRequest("invalid cursor", err)
		}
		t, err := time.Parse(time.RFC3339Nano, lastRequestAt)
		if err != nil {
			return nil, huma.Error400BadRequest("invalid cursor", err)
		}
		cursorLastRequestAt = pgtype.Timestamp{Time: t.UTC(), Valid: true}
		cursorParentSpanID = pgtype.Text{String: parentSpanID, Valid: true}
	}

	rows, err := s.queries.ListRequestTraces(ctx, db.ListRequestTracesParams{
		CursorLastRequestAt: cursorLastRequestAt,
		CursorParentSpanID:  cursorParentSpanID,
		Limit:               pgtype.Int4{Int32: fetchLimit, Valid: true},
	})
	if err != nil {
		return nil, huma.Error500InternalServerError("failed to list request traces", err)
	}

	hasMore := int32(len(rows)) > limit
	if hasMore {
		rows = rows[:limit]
	}

	items := make([]contract.RequestTraceView, len(rows))
	for i, row := range rows {
		view, err := contract.ToRequestTraceView(&row)
		if err != nil {
			return nil, huma.Error500InternalServerError("failed to parse request trace costs", err)
		}
		items[i] = *view
	}

	pagination := contract.PaginationInfo{HasMore: hasMore}
	if hasMore && len(rows) > 0 {
		last := rows[len(rows)-1]
		lastRequestAt := ""
		if last.LastRequestAt.Valid {
			lastRequestAt = last.LastRequestAt.Time.UTC().Format(time.RFC3339Nano)
		}
		parentSpanID := ""
		if last.ParentSpanID.Valid {
			parentSpanID = last.ParentSpanID.String
		}
		cursor, err := contract.EncodeCursor("lastRequestAt", lastRequestAt, "parentSpanId", parentSpanID)
		if err != nil {
			return nil, huma.Error500InternalServerError("failed to encode cursor", err)
		}
		pagination.NextCursor = cursor
	}

	return &contract.ListRequestTracesResponse{
		Body: contract.PaginatedBody[contract.RequestTraceView]{
			Items:      items,
			Pagination: pagination,
		},
	}, nil
}

func (s *Server) handleGetRequest(ctx context.Context, input *contract.GetRequestRequest) (*contract.GetRequestResponse, error) {
	idCreatedAt, err := requestIDCreatedAt(input.ID)
	if err != nil {
		return nil, err
	}
	req, err := s.queries.GetRequest(ctx, db.GetRequestParams{
		ID:          input.ID,
		IDCreatedAt: pgtype.Timestamp{Time: idCreatedAt, Valid: true},
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, huma.Error404NotFound("request not found", errorx.RequestNotFound)
		}
		return nil, huma.Error500InternalServerError("failed to get request", err)
	}
	view := contract.ToRequestView(&req)
	s.attachArtifactUrls(ctx, view, req.CreatedAt)
	return &contract.GetRequestResponse{Body: *view}, nil
}

func (s *Server) handleListRequestSpans(ctx context.Context, input *contract.ListRequestSpansRequest) (*contract.ListRequestSpansResponse, error) {
	idCreatedAt, err := requestIDCreatedAt(input.ID)
	if err != nil {
		return nil, err
	}
	req, err := s.queries.GetRequest(ctx, db.GetRequestParams{
		ID:          input.ID,
		IDCreatedAt: pgtype.Timestamp{Time: idCreatedAt, Valid: true},
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, huma.Error404NotFound("request not found", errorx.RequestNotFound)
		}
		return nil, huma.Error500InternalServerError("failed to get request", err)
	}
	rows, err := s.queries.ListRequestsBySpan(ctx, db.ListRequestsBySpanParams{
		ID:          input.ID,
		IDCreatedAt: pgtype.Timestamp{Time: idCreatedAt, Valid: true},
	})
	if err != nil {
		return nil, huma.Error500InternalServerError("failed to list request spans", err)
	}
	if len(rows) == 0 {
		view := contract.ToRequestView(&req)
		s.attachArtifactUrls(ctx, view, req.CreatedAt)
		return &contract.ListRequestSpansResponse{Body: []contract.RequestView{*view}}, nil
	}
	items := make([]contract.RequestView, len(rows))
	for i, row := range rows {
		items[i] = *contract.ToListRequestsBySpanRowView(&row)
		s.attachArtifactUrls(ctx, &items[i], row.CreatedAt)
	}
	return &contract.ListRequestSpansResponse{Body: items}, nil
}
