package contract

import (
	"encoding/json"
	"net/http"
	"picotera/pkg/db"

	"github.com/danielgtaylor/huma/v2"
)

type GetProviderRequest struct {
	ID int32 `path:"id" example:"1"`
}

type ProviderView struct {
	ID             int32             `json:"id"`
	Name           string            `json:"name"`
	Credentials    string            `json:"credentials"`
	Priority       int32             `json:"priority"`
	ProviderModels []string          `json:"providerModels"`
	Annotations    map[string]string `json:"annotations"`
}

type GetProviderResponse struct {
	Body ProviderView
}

type CreateProviderRequest struct {
	Body struct {
		Name           string            `json:"name"`
		Credentials    string            `json:"credentials"`
		Priority       int32             `json:"priority"`
		ProviderModels []string          `json:"providerModels"`
		Annotations    map[string]string `json:"annotations"`
	}
}

type CreateProviderResponse struct {
	Body ProviderView
}

func ToProviderView(provider *db.Provider) (*ProviderView, error) {
	var providerModels []string
	err := json.Unmarshal(provider.ProviderModels, &providerModels)
	if err != nil {
		return nil, err
	}

	var annotations map[string]string
	err = json.Unmarshal(provider.Annotations, &annotations)
	if err != nil {
		return nil, err
	}

	return &ProviderView{
		ID:             provider.ID,
		Name:           provider.Name,
		Credentials:    provider.Credentials,
		Priority:       provider.Priority,
		ProviderModels: providerModels,
		Annotations:    annotations,
	}, nil
}

func FromProviderView(providerView *ProviderView) (*db.Provider, error) {
	providerModels, err := json.Marshal(providerView.ProviderModels)
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
	}, nil
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
