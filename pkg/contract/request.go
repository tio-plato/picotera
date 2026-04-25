package contract

import (
	"net/http"
	"picotera/pkg/db"
	"time"

	"github.com/danielgtaylor/huma/v2"
)

type RequestView struct {
	ID               string  `json:"id"`
	SpanID           string  `json:"spanId,omitempty"`
	ParentSpanID     string  `json:"parentSpanId,omitempty"`
	ProviderID       int32   `json:"providerId"`
	EndpointPath     string  `json:"endpointPath"`
	ApiKeyID         *int32  `json:"apiKeyId,omitempty"`
	Model            string  `json:"model,omitempty"`
	InputTokens      *int32  `json:"inputTokens,omitempty"`
	CacheReadTokens  *int32  `json:"cacheReadTokens,omitempty"`
	OutputTokens     *int32  `json:"outputTokens,omitempty"`
	CacheWriteTokens *int32  `json:"cacheWriteTokens,omitempty"`
	StatusCode       int32   `json:"statusCode"`
	ErrorMessage     string  `json:"errorMessage,omitempty"`
	TtftMs           *int32  `json:"ttftMs,omitempty"`
	TimeSpentMs      int32   `json:"timeSpentMs"`
	CreatedAt        string  `json:"createdAt"`
}

func ToRequestView(r *db.Request) *RequestView {
	view := &RequestView{
		ID:           r.ID,
		ProviderID:   r.ProviderID,
		EndpointPath: r.EndpointPath,
		StatusCode:   r.StatusCode,
		TimeSpentMs:  r.TimeSpentMs,
	}
	if r.SpanID.Valid {
		view.SpanID = r.SpanID.String
	}
	if r.ParentSpanID.Valid {
		view.ParentSpanID = r.ParentSpanID.String
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
	if r.ErrorMessage.Valid {
		view.ErrorMessage = r.ErrorMessage.String
	}
	if r.TtftMs.Valid {
		v := r.TtftMs.Int32
		view.TtftMs = &v
	}
	if r.CreatedAt.Valid {
		view.CreatedAt = r.CreatedAt.Time.UTC().Format(time.RFC3339Nano)
	}
	return view
}

type ListRequestsRequest struct {
	PaginationRequest
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
