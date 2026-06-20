package contract

import (
	"net/http"

	"github.com/danielgtaylor/huma/v2"
)

// ConfigView is the runtime application configuration exposed to the dashboard.
type ConfigView struct {
	Title string `json:"title"`
}

// GetConfigResponse is the response for reading the application configuration.
type GetConfigResponse struct {
	Body ConfigView
}

var OperationGetConfig = huma.Operation{
	OperationID: "getConfig",
	Method:      http.MethodGet,
	Path:        "/config",
	Summary:     "Get runtime application configuration",
}
