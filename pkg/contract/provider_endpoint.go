package contract

import (
	"net/http"
	"picotera/pkg/db"

	"github.com/danielgtaylor/huma/v2"
)

type ProviderEndpointView struct {
	ProviderID          int32  `json:"providerId"`
	EndpointPath        string `json:"endpointPath"`
	UpstreamUrl         string `json:"upstreamUrl"`
	CredentialsResolver string `json:"credentialsResolver,omitempty" enum:"unknown,generalApiKey,bearerToken,xApiKey,searchKey,googApiKey"`
}

func ToProviderEndpointView(pe *db.ProviderEndpoint) *ProviderEndpointView {
	return &ProviderEndpointView{
		ProviderID:          pe.ProviderID,
		EndpointPath:        pe.EndpointPath,
		UpstreamUrl:         pe.UpstreamUrl,
		CredentialsResolver: FromCredentialsResolver(pe.CredentialsResolver),
	}
}

func FromProviderEndpointView(view *ProviderEndpointView) *db.UpsertProviderEndpointParams {
	return &db.UpsertProviderEndpointParams{
		ProviderID:          view.ProviderID,
		EndpointPath:        view.EndpointPath,
		UpstreamUrl:         view.UpstreamUrl,
		CredentialsResolver: ToCredentialsResolver(view.CredentialsResolver),
	}
}

type ListProviderEndpointsRequest struct {
	ProviderID int32 `query:"providerId"`
}

type ListProviderEndpointsResponse struct {
	Body []ProviderEndpointView
}

type UpsertProviderEndpointRequest struct {
	Body ProviderEndpointView
}

type UpsertProviderEndpointResponse struct {
	Body ProviderEndpointView
}

type DeleteProviderEndpointRequest struct {
	Body struct {
		ProviderID   int32  `json:"providerId"`
		EndpointPath string `json:"endpointPath"`
	}
}

var OperationListProviderEndpoints = huma.Operation{
	OperationID: "listProviderEndpoints",
	Method:      http.MethodGet,
	Path:        "/provider-endpoints",
	Summary:     "List provider endpoints",
}

var OperationUpsertProviderEndpoint = huma.Operation{
	OperationID: "upsertProviderEndpoint",
	Method:      http.MethodPut,
	Path:        "/provider-endpoints",
	Summary:     "Upsert a provider endpoint",
}

var OperationDeleteProviderEndpoint = huma.Operation{
	OperationID: "deleteProviderEndpoint",
	Method:      http.MethodPost,
	Path:        "/provider-endpoints/delete",
	Summary:     "Delete a provider endpoint",
}

type FetchModelsRequest struct {
	Body struct {
		ProviderID   int32  `json:"providerId"`
		EndpointPath string `json:"endpointPath"`
	}
}

type FetchModelsResponse struct {
	Body struct {
		ProviderID     int32                `json:"providerId"`
		ProviderModels []ProviderModelEntry `json:"providerModels"`
		RemovedModels  []string             `json:"removedModels"`
	}
}

var OperationFetchModels = huma.Operation{
	OperationID: "fetchModels",
	Method:      http.MethodPost,
	Path:        "/provider-endpoints/fetch-models",
	Summary:     "Fetch model list from upstream provider",
}
