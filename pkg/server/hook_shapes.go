package server

import (
	"picotera/pkg/db"
	"picotera/pkg/jsx"
)

func endpointSummaryFromRow(row db.Endpoint) jsx.EndpointSummary {
	return jsx.EndpointSummary{
		Name:                row.Name,
		Path:                row.Path,
		ModelPath:           row.ModelPath,
		CredentialsResolver: row.CredentialsResolver,
		EndpointType:        row.EndpointType,
	}
}
