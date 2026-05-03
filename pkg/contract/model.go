package contract

import (
	"net/http"
	"picotera/pkg/db"

	"github.com/danielgtaylor/huma/v2"
)

type GetModelRequest struct {
	Name string `path:"name" example:"gpt-4o"`
}

type ModelView struct {
	Name      string   `json:"name"`
	Title     string   `json:"title"`
	Developer string   `json:"developer"`
	Series    string   `json:"series"`
	Disabled  bool     `json:"disabled"`
	Pricing   *Pricing `json:"pricing,omitempty"`
}

type GetModelResponse struct {
	Body ModelView
}

type PutModelRequest struct {
	Body ModelView
}

type DeleteModelRequest struct {
	Body struct {
		Name string `json:"name"`
	}
}

type ListModelsResponse struct {
	Body []ModelView
}

var OperationListModels = huma.Operation{
	OperationID: "listModels",
	Method:      http.MethodGet,
	Path:        "/models",
	Summary:     "List all models",
}

var OperationGetModel = huma.Operation{
	OperationID: "getModel",
	Method:      http.MethodGet,
	Path:        "/models/{name}",
	Summary:     "Get a model by name",
}

var OperationPutModel = huma.Operation{
	OperationID: "putModel",
	Method:      http.MethodPut,
	Path:        "/models",
	Summary:     "Upsert a model",
}

var OperationDeleteModel = huma.Operation{
	OperationID: "deleteModel",
	Method:      http.MethodPost,
	Path:        "/models/delete",
	Summary:     "Delete a model",
}

func ToModelView(model *db.Model) (*ModelView, error) {
	pricing, err := PricingFromJSONB(model.Pricing)
	if err != nil {
		return nil, err
	}
	return &ModelView{
		Name:      model.Name,
		Title:     model.Title.String,
		Developer: model.Developer.String,
		Series:    model.Series.String,
		Disabled:  model.Disabled,
		Pricing:   pricing,
	}, nil
}
