package contract

import (
	"encoding/json"
	"net/http"

	"picotera/pkg/db"

	"github.com/danielgtaylor/huma/v2"
)

// GlobalSettingView represents a key-value global setting.
type GlobalSettingView struct {
	Key   string          `json:"key"`
	Value json.RawMessage `json:"value"`
}

// ToGlobalSettingView converts a db.GlobalSetting row to the API view.
func ToGlobalSettingView(r *db.GlobalSetting) GlobalSettingView {
	v := make(json.RawMessage, len(r.Value))
	copy(v, r.Value)
	return GlobalSettingView{Key: r.Key, Value: v}
}

// ListGlobalSettingsResponse is the response for listing all settings.
type ListGlobalSettingsResponse struct {
	Body []GlobalSettingView
}

// GetGlobalSettingRequest is the request for getting a single setting.
type GetGlobalSettingRequest struct {
	Key string `path:"key"`
}

// GetGlobalSettingResponse is the response for getting a single setting.
type GetGlobalSettingResponse struct {
	Body GlobalSettingView
}

// UpsertGlobalSettingRequestBody is the body for creating or updating a setting.
type UpsertGlobalSettingRequestBody struct {
	Key   string          `json:"key"`
	Value json.RawMessage `json:"value"`
}

// UpsertGlobalSettingRequest is the request for creating or updating a setting.
type UpsertGlobalSettingRequest struct {
	Body UpsertGlobalSettingRequestBody
}

// UpsertGlobalSettingResponse is the response for creating or updating a setting.
type UpsertGlobalSettingResponse struct {
	Body GlobalSettingView
}

// DeleteGlobalSettingRequest is the request for deleting a setting.
type DeleteGlobalSettingRequest struct {
	Key string `path:"key"`
}

// Operation declarations.

var OperationListGlobalSettings = huma.Operation{
	OperationID: "listGlobalSettings",
	Method:      http.MethodGet,
	Path:        "/settings",
	Summary:     "List all global settings",
}

var OperationGetGlobalSetting = huma.Operation{
	OperationID: "getGlobalSetting",
	Method:      http.MethodGet,
	Path:        "/settings/{key}",
	Summary:     "Get a global setting by key",
}

var OperationUpsertGlobalSetting = huma.Operation{
	OperationID: "upsertGlobalSetting",
	Method:      http.MethodPut,
	Path:        "/settings",
	Summary:     "Create or update a global setting",
}

var OperationDeleteGlobalSetting = huma.Operation{
	OperationID: "deleteGlobalSetting",
	Method:      http.MethodDelete,
	Path:        "/settings/{key}",
	Summary:     "Delete a global setting",
}
