package contract

import (
	"encoding/json"
	"net/http"

	"picotera/pkg/annotations"
	"picotera/pkg/db"

	"github.com/danielgtaylor/huma/v2"
	"github.com/jackc/pgx/v5/pgtype"
)

type GetModelRequest struct {
	Name string `path:"name" example:"gpt-4o"`
}

type ModelView struct {
	Name        string            `json:"name"`
	Title       string            `json:"title"`
	Developer   string            `json:"developer"`
	Series      string            `json:"series"`
	Disabled    bool              `json:"disabled"`
	Pricing     *Pricing          `json:"pricing,omitempty"`
	Annotations map[string]string `json:"annotations"`
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
	anno, err := annotations.Decode(model.Annotations)
	if err != nil {
		return nil, err
	}
	return &ModelView{
		Name:        model.Name,
		Title:       model.Title.String,
		Developer:   model.Developer.String,
		Series:      model.Series.String,
		Disabled:    model.Disabled,
		Pricing:     pricing,
		Annotations: anno,
	}, nil
}

// FromModelView converts a ModelView into the sqlc UpsertModel params, marshaling
// pricing and annotations to JSONB. A nil/empty annotations map is written as
// "{}" so the column never goes through the DEFAULT path on update.
func FromModelView(view *ModelView) (*db.UpsertModelParams, error) {
	pricingBytes, err := PricingToJSONB(view.Pricing)
	if err != nil {
		return nil, err
	}
	anno := view.Annotations
	if anno == nil {
		anno = map[string]string{}
	}
	annoBytes, err := json.Marshal(anno)
	if err != nil {
		return nil, err
	}
	return &db.UpsertModelParams{
		Name:        view.Name,
		Title:       pgtype.Text{String: view.Title, Valid: true},
		Developer:   pgtype.Text{String: view.Developer, Valid: true},
		Series:      pgtype.Text{String: view.Series, Valid: true},
		Disabled:    view.Disabled,
		Pricing:     pricingBytes,
		Annotations: annoBytes,
	}, nil
}
