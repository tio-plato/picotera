package contract

import (
	"encoding/json"
	"net/http"
	"picotera/pkg/db"

	"github.com/danielgtaylor/huma/v2"
	"github.com/jackc/pgx/v5/pgtype"
)

type ModelProviderEndpointView struct {
	ModelName         string            `json:"modelName"`
	ProviderID        int32             `json:"providerId"`
	EndpointPath      string            `json:"endpointPath"`
	UpstreamModelName string            `json:"upstreamModelName,omitempty"`
	Priority          int32             `json:"priority"`
	Annotations       map[string]string `json:"annotations"`
}

func ToModelProviderEndpointView(mpe *db.ModelProviderEndpoint) (*ModelProviderEndpointView, error) {
	var annotations map[string]string
	if err := json.Unmarshal(mpe.Annotations, &annotations); err != nil {
		return nil, err
	}

	return &ModelProviderEndpointView{
		ModelName:         mpe.ModelName,
		ProviderID:        mpe.ProviderID,
		EndpointPath:      mpe.EndpointPath,
		UpstreamModelName: mpe.UpstreamModelName.String,
		Priority:          mpe.Priority,
		Annotations:       annotations,
	}, nil
}

func FromModelProviderEndpointView(view *ModelProviderEndpointView) (*db.UpsertModelProviderEndpointParams, error) {
	annotations, err := json.Marshal(view.Annotations)
	if err != nil {
		return nil, err
	}

	return &db.UpsertModelProviderEndpointParams{
		ModelName:         view.ModelName,
		ProviderID:        view.ProviderID,
		EndpointPath:      view.EndpointPath,
		UpstreamModelName: pgtype.Text{String: view.UpstreamModelName, Valid: view.UpstreamModelName != ""},
		Priority:          view.Priority,
		Annotations:       annotations,
	}, nil
}

type ListModelProviderEndpointsRequest struct {
	PaginationRequest
	ModelName    string `query:"modelName,omitempty"`
	ProviderID   int32  `query:"providerId,omitempty"`
	EndpointPath string `query:"endpointPath,omitempty"`
}

type ListModelProviderEndpointsResponse = PaginatedResponse[ModelProviderEndpointView]

type GetModelProviderEndpointRequest struct {
	ModelName    string `query:"modelName"`
	ProviderID   int32  `query:"providerId"`
	EndpointPath string `query:"endpointPath"`
}

type GetModelProviderEndpointResponse struct {
	Body ModelProviderEndpointView
}

type UpsertModelProviderEndpointRequest struct {
	Body ModelProviderEndpointView
}

type UpsertModelProviderEndpointResponse struct {
	Body ModelProviderEndpointView
}

type DeleteModelProviderEndpointRequest struct {
	Body struct {
		ModelName    string `json:"modelName"`
		ProviderID   int32  `json:"providerId"`
		EndpointPath string `json:"endpointPath"`
	}
}

var OperationListModelProviderEndpoints = huma.Operation{
	OperationID: "listModelProviderEndpoints",
	Method:      http.MethodGet,
	Path:        "/model-provider-endpoints",
	Summary:     "List model provider endpoints",
}

var OperationGetModelProviderEndpoint = huma.Operation{
	OperationID: "getModelProviderEndpoint",
	Method:      http.MethodGet,
	Path:        "/model-provider-endpoints/get",
	Summary:     "Get a model provider endpoint",
}

var OperationUpsertModelProviderEndpoint = huma.Operation{
	OperationID: "upsertModelProviderEndpoint",
	Method:      http.MethodPut,
	Path:        "/model-provider-endpoints",
	Summary:     "Upsert a model provider endpoint",
}

var OperationDeleteModelProviderEndpoint = huma.Operation{
	OperationID: "deleteModelProviderEndpoint",
	Method:      http.MethodPost,
	Path:        "/model-provider-endpoints/delete",
	Summary:     "Delete a model provider endpoint",
}
