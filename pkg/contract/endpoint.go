package contract

import (
	"net/http"
	"picotera/pkg/db"

	"github.com/danielgtaylor/huma/v2"
)

const (
	CredentialsResolver_Unknown       int32 = 0
	CredentialsResolver_GeneralApiKey int32 = 1
	CredentialsResolver_BearerToken   int32 = 2
	CredentialsResolver_XApiKey       int32 = 3
)

func ToCredentialsResolver(s string) int32 {
	switch s {
	case "unknown":
		return CredentialsResolver_Unknown
	case "generalApiKey":
		return CredentialsResolver_GeneralApiKey
	case "bearerToken":
		return CredentialsResolver_BearerToken
	case "xApiKey":
		return CredentialsResolver_XApiKey
	default:
		return CredentialsResolver_Unknown
	}
}

func FromCredentialsResolver(cr int32) string {
	switch cr {
	case CredentialsResolver_Unknown:
		return "unknown"
	case CredentialsResolver_GeneralApiKey:
		return "generalApiKey"
	case CredentialsResolver_BearerToken:
		return "bearerToken"
	case CredentialsResolver_XApiKey:
		return "xApiKey"
	default:
		return "unknown"
	}
}

type EndpointView struct {
	Name                string `json:"name"`
	Path                string `json:"path"`
	ModelPath           string `json:"modelPath"`
	CredentialsResolver string `json:"credentialsResolver" enum:"generalApiKey,bearerToken,xApiKey,unknown"`
}

func ToEndpointView(endpoint *db.Endpoint) (*EndpointView, error) {
	return &EndpointView{
		Name:                endpoint.Name,
		Path:                endpoint.Path,
		ModelPath:           endpoint.ModelPath,
		CredentialsResolver: FromCredentialsResolver(endpoint.CredentialsResolver),
	}, nil
}

type ListEndpointsResponse struct {
	Body []EndpointView
}

type UpsertEndpointRequest struct {
	Body EndpointView
}

type UpsertEndpointResponse struct {
	Body EndpointView
}

type DeleteEndpointRequest struct {
	Body struct {
		Path string `json:"path"`
	}
}

var OperationListEndpoints = huma.Operation{
	OperationID: "listEndpoints",
	Method:      http.MethodGet,
	Path:        "/endpoints",
	Summary:     "List all endpoints",
}

var OperationUpsertEndpoint = huma.Operation{
	OperationID: "upsertEndpoint",
	Method:      http.MethodPut,
	Path:        "/endpoints",
	Summary:     "Upsert an endpoint",
}

var OperationDeleteEndpoint = huma.Operation{
	OperationID: "deleteEndpoint",
	Method:      http.MethodPost,
	Path:        "/endpoints/delete",
	Summary:     "Delete an endpoint",
}
