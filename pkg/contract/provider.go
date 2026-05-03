package contract

import (
	"encoding/json"
	"net/http"
	"picotera/pkg/db"

	"github.com/danielgtaylor/huma/v2"
)

type ProviderModelEntry struct {
	Model             string            `json:"model"`
	UpstreamModelName string            `json:"upstreamModelName,omitempty"`
	Endpoints         []string          `json:"endpoints,omitempty"`
	Priority          int32             `json:"priority,omitempty"`
	Annotations       map[string]string `json:"annotations,omitempty"`
	Disabled          bool              `json:"disabled,omitempty"`
	Pricing           *Pricing          `json:"pricing,omitempty"`
}

type GetProviderRequest struct {
	ID int32 `path:"id" example:"1"`
}

type ProviderView struct {
	ID             int32                         `json:"id"`
	Name           string                        `json:"name"`
	Credentials    string                        `json:"credentials"`
	Priority       int32                         `json:"priority"`
	ProviderModels []ProviderModelEntry          `json:"providerModels"`
	Annotations    map[string]string             `json:"annotations"`
	Disabled       bool                          `json:"disabled"`
}

type GetProviderResponse struct {
	Body ProviderView
}

type CreateProviderRequest struct {
	Body struct {
		Name           string                        `json:"name"`
		Credentials    string                        `json:"credentials"`
		Priority       int32                         `json:"priority"`
		ProviderModels []ProviderModelEntry          `json:"providerModels"`
		Annotations    map[string]string             `json:"annotations"`
		Disabled       bool                          `json:"disabled"`
	}
}

type CreateProviderResponse struct {
	Body ProviderView
}

type UpsertProviderRequest struct {
	Body struct {
		ID             int32                         `json:"id,omitempty"`
		Name           string                        `json:"name"`
		Credentials    string                        `json:"credentials"`
		Priority       int32                         `json:"priority"`
		ProviderModels []ProviderModelEntry          `json:"providerModels"`
		Annotations    map[string]string             `json:"annotations"`
		Disabled       bool                          `json:"disabled"`
	}
}

type UpsertProviderResponse struct {
	Body ProviderView
}

type DeleteProviderRequest struct {
	Body struct {
		ID int32 `json:"id"`
	}
}

func ToProviderView(provider *db.Provider) (*ProviderView, error) {
	providerModels := []ProviderModelEntry{}
	if len(provider.ProviderModels) > 0 {
		if err := json.Unmarshal(provider.ProviderModels, &providerModels); err != nil {
			return nil, err
		}
	}
	if providerModels == nil {
		providerModels = []ProviderModelEntry{}
	}

	annotations := map[string]string{}
	if len(provider.Annotations) > 0 {
		if err := json.Unmarshal(provider.Annotations, &annotations); err != nil {
			return nil, err
		}
	}

	return &ProviderView{
		ID:             provider.ID,
		Name:           provider.Name,
		Credentials:    provider.Credentials,
		Priority:       provider.Priority,
		ProviderModels: providerModels,
		Annotations:    annotations,
		Disabled:       provider.Disabled,
	}, nil
}

func FromProviderView(providerView *ProviderView) (*db.Provider, error) {
	models := providerView.ProviderModels
	if models == nil {
		models = []ProviderModelEntry{}
	}
	providerModels, err := json.Marshal(models)
	if err != nil {
		return nil, err
	}

	annotations, err := json.Marshal(providerView.Annotations)
	if err != nil {
		return nil, err
	}

	return &db.Provider{
		ID:             providerView.ID,
		Name:           providerView.Name,
		Credentials:    providerView.Credentials,
		Priority:       providerView.Priority,
		ProviderModels: providerModels,
		Annotations:    annotations,
		Disabled:       providerView.Disabled,
	}, nil
}

type ListProvidersResponse struct {
	Body []ProviderView
}

var OperationListProviders = huma.Operation{
	OperationID: "listProviders",
	Method:      http.MethodGet,
	Path:        "/providers",
	Summary:     "List all providers",
}

var OperationGetProvider = huma.Operation{
	OperationID: "getProvider",
	Method:      http.MethodGet,
	Path:        "/providers/{id}",
	Summary:     "Get a provider by ID",
}

var OperationCreateProvider = huma.Operation{
	OperationID: "createProvider",
	Method:      http.MethodPost,
	Path:        "/providers",
	Summary:     "Create a new provider",
}

var OperationUpsertProvider = huma.Operation{
	OperationID: "upsertProvider",
	Method:      http.MethodPut,
	Path:        "/providers",
	Summary:     "Upsert a provider",
}

var OperationDeleteProvider = huma.Operation{
	OperationID: "deleteProvider",
	Method:      http.MethodPost,
	Path:        "/providers/delete",
	Summary:     "Delete a provider",
}
