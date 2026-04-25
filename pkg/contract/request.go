package contract

import (
	"net/http"
	"picotera/pkg/db"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/jackc/pgx/v5/pgtype"
)

type RequestView struct {
	ID               string  `json:"id"`
	SpanID           string  `json:"spanId,omitempty"`
	ParentSpanID     string  `json:"parentSpanId,omitempty"`
	Type             int32   `json:"type"`
	Status           int32   `json:"status"`
	ProviderID       *int32  `json:"providerId,omitempty"`
	EndpointPath     string  `json:"endpointPath,omitempty"`
	ApiKeyID         *int32  `json:"apiKeyId,omitempty"`
	Model            string  `json:"model,omitempty"`
	InputTokens      *int32  `json:"inputTokens,omitempty"`
	CacheReadTokens  *int32  `json:"cacheReadTokens,omitempty"`
	OutputTokens     *int32  `json:"outputTokens,omitempty"`
	CacheWriteTokens *int32  `json:"cacheWriteTokens,omitempty"`
	StatusCode       *int32  `json:"statusCode,omitempty"`
	ErrorMessage     string  `json:"errorMessage,omitempty"`
	TtftMs           *int32  `json:"ttftMs,omitempty"`
	TimeSpentMs      *int32  `json:"timeSpentMs,omitempty"`
	CreatedAt        string  `json:"createdAt,omitempty"`
}

type requestLike struct {
	ID               string
	SpanID           pgtype.Text
	ParentSpanID     pgtype.Text
	Type             int32
	Status           int32
	ProviderID       pgtype.Int4
	EndpointPath     pgtype.Text
	ApiKeyID         pgtype.Int4
	Model            pgtype.Text
	InputTokens      pgtype.Int4
	CacheReadTokens  pgtype.Int4
	OutputTokens     pgtype.Int4
	CacheWriteTokens pgtype.Int4
	StatusCode       pgtype.Int4
	ErrorMessage     pgtype.Text
	TtftMs           pgtype.Int4
	TimeSpentMs      pgtype.Int4
	CreatedAt        pgtype.Timestamp
}

func toRequestView(r requestLike) *RequestView {
	view := &RequestView{
		ID:     r.ID,
		Type:   r.Type,
		Status: r.Status,
	}
	if r.SpanID.Valid {
		view.SpanID = r.SpanID.String
	}
	if r.ParentSpanID.Valid {
		view.ParentSpanID = r.ParentSpanID.String
	}
	if r.ProviderID.Valid {
		v := r.ProviderID.Int32
		view.ProviderID = &v
	}
	if r.EndpointPath.Valid {
		view.EndpointPath = r.EndpointPath.String
	}
	if r.ApiKeyID.Valid {
		v := r.ApiKeyID.Int32
		view.ApiKeyID = &v
	}
	if r.Model.Valid {
		view.Model = r.Model.String
	}
	if r.InputTokens.Valid {
		v := r.InputTokens.Int32
		view.InputTokens = &v
	}
	if r.CacheReadTokens.Valid {
		v := r.CacheReadTokens.Int32
		view.CacheReadTokens = &v
	}
	if r.OutputTokens.Valid {
		v := r.OutputTokens.Int32
		view.OutputTokens = &v
	}
	if r.CacheWriteTokens.Valid {
		v := r.CacheWriteTokens.Int32
		view.CacheWriteTokens = &v
	}
	if r.StatusCode.Valid {
		v := r.StatusCode.Int32
		view.StatusCode = &v
	}
	if r.ErrorMessage.Valid {
		view.ErrorMessage = r.ErrorMessage.String
	}
	if r.TtftMs.Valid {
		v := r.TtftMs.Int32
		view.TtftMs = &v
	}
	if r.TimeSpentMs.Valid {
		v := r.TimeSpentMs.Int32
		view.TimeSpentMs = &v
	}
	if r.CreatedAt.Valid {
		view.CreatedAt = r.CreatedAt.Time.UTC().Format(time.RFC3339Nano)
	}
	return view
}

func ToRequestView(r *db.Request) *RequestView {
	return toRequestView(requestLike{
		ID:               r.ID,
		SpanID:           r.SpanID,
		ParentSpanID:     r.ParentSpanID,
		Type:             r.Type,
		Status:           r.Status,
		ProviderID:       r.ProviderID,
		EndpointPath:     r.EndpointPath,
		ApiKeyID:         r.ApiKeyID,
		Model:            r.Model,
		InputTokens:      r.InputTokens,
		CacheReadTokens:  r.CacheReadTokens,
		OutputTokens:     r.OutputTokens,
		CacheWriteTokens: r.CacheWriteTokens,
		StatusCode:       r.StatusCode,
		ErrorMessage:     r.ErrorMessage,
		TtftMs:           r.TtftMs,
		TimeSpentMs:      r.TimeSpentMs,
		CreatedAt:        r.CreatedAt,
	})
}

func ToListRequestRowView(r *db.ListRequestsRow) *RequestView {
	return toRequestView(requestLike{
		ID:               r.ID,
		SpanID:           r.SpanID,
		ParentSpanID:     r.ParentSpanID,
		Type:             r.Type,
		Status:           r.Status,
		ProviderID:       r.ProviderID,
		EndpointPath:     r.EndpointPath,
		ApiKeyID:         r.ApiKeyID,
		Model:            r.Model,
		InputTokens:      r.InputTokens,
		CacheReadTokens:  r.CacheReadTokens,
		OutputTokens:     r.OutputTokens,
		CacheWriteTokens: r.CacheWriteTokens,
		StatusCode:       r.StatusCode,
		ErrorMessage:     r.ErrorMessage,
		TtftMs:           r.TtftMs,
		TimeSpentMs:      r.TimeSpentMs,
		CreatedAt:        r.CreatedAt,
	})
}

func ToListRequestsBySpanRowView(r *db.ListRequestsBySpanRow) *RequestView {
	return toRequestView(requestLike{
		ID:               r.ID,
		SpanID:           r.SpanID,
		ParentSpanID:     r.ParentSpanID,
		Type:             r.Type,
		Status:           r.Status,
		ProviderID:       r.ProviderID,
		EndpointPath:     r.EndpointPath,
		ApiKeyID:         r.ApiKeyID,
		Model:            r.Model,
		InputTokens:      r.InputTokens,
		CacheReadTokens:  r.CacheReadTokens,
		OutputTokens:     r.OutputTokens,
		CacheWriteTokens: r.CacheWriteTokens,
		StatusCode:       r.StatusCode,
		ErrorMessage:     r.ErrorMessage,
		TtftMs:           r.TtftMs,
		TimeSpentMs:      r.TimeSpentMs,
		CreatedAt:        r.CreatedAt,
	})
}

type ListRequestsRequest struct {
	PaginationRequest
	Type         int32  `query:"type,omitempty" default:"-1"`
	ProviderID   int32  `query:"providerId,omitempty"`
	EndpointPath string `query:"endpointPath,omitempty"`
	Model        string `query:"model,omitempty"`
}

type ListRequestsResponse = PaginatedResponse[RequestView]

type GetRequestRequest struct {
	ID string `path:"id"`
}

type GetRequestResponse struct {
	Body RequestView
}

type ListRequestSpansRequest struct {
	ID string `path:"id"`
}

type ListRequestSpansResponse struct {
	Body []RequestView
}

var OperationListRequests = huma.Operation{
	OperationID: "listRequests",
	Method:      http.MethodGet,
	Path:        "/requests",
	Summary:     "List requests",
}

var OperationGetRequest = huma.Operation{
	OperationID: "getRequest",
	Method:      http.MethodGet,
	Path:        "/requests/{id}",
	Summary:     "Get a request by ID",
}

var OperationListRequestSpans = huma.Operation{
	OperationID: "listRequestSpans",
	Method:      http.MethodGet,
	Path:        "/requests/{id}/spans",
	Summary:     "List spans (meta + upstream) related to a request",
}
