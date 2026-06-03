package contract

import (
	"encoding/json"
	"net/http"
	"picotera/pkg/db"
	"time"

	"github.com/danielgtaylor/huma/v2"
)

type ProjectView struct {
	ID          int32    `json:"id"`
	Name        string   `json:"name"`
	Paths       []string `json:"paths"`
	FirstSeenAt string   `json:"firstSeenAt,omitempty"`
	LastSeenAt  string   `json:"lastSeenAt,omitempty"`
	CreatedAt   string   `json:"createdAt"`
	UpdatedAt   string   `json:"updatedAt"`
	AutoCreated bool     `json:"autoCreated"`
}

func ToProjectView(p *db.Project) (*ProjectView, error) {
	paths := []string{}
	if len(p.Paths) > 0 {
		if err := json.Unmarshal(p.Paths, &paths); err != nil {
			return nil, err
		}
	}
	if paths == nil {
		paths = []string{}
	}
	v := &ProjectView{
		ID:          p.ID,
		Name:        p.Name,
		Paths:       paths,
		AutoCreated: p.AutoCreated,
	}
	if p.FirstSeenAt.Valid {
		v.FirstSeenAt = p.FirstSeenAt.Time.UTC().Format(time.RFC3339)
	}
	if p.LastSeenAt.Valid {
		v.LastSeenAt = p.LastSeenAt.Time.UTC().Format(time.RFC3339)
	}
	if p.CreatedAt.Valid {
		v.CreatedAt = p.CreatedAt.Time.UTC().Format(time.RFC3339)
	}
	if p.UpdatedAt.Valid {
		v.UpdatedAt = p.UpdatedAt.Time.UTC().Format(time.RFC3339)
	}
	return v, nil
}

type ListProjectsResponse struct {
	Body []ProjectView
}

type GetProjectRequest struct {
	ID int32 `path:"id"`
}
type GetProjectResponse struct{ Body ProjectView }

type UpsertProjectRequest struct {
	Body struct {
		ID    int32    `json:"id,omitempty"`
		Name  string   `json:"name"`
		Paths []string `json:"paths"`
	}
}

type UpsertProjectResponse struct {
	Body ProjectView
}

type DeleteProjectRequest struct {
	Body struct {
		ID int32 `json:"id"`
	}
}

var OperationListProjects = huma.Operation{
	OperationID: "listProjects",
	Method:      http.MethodGet,
	Path:        "/projects",
	Summary:     "List all projects",
}

var OperationGetProject = huma.Operation{
	OperationID: "getProject",
	Method:      http.MethodGet,
	Path:        "/projects/{id}",
	Summary:     "Get a project by ID",
}

var OperationUpsertProject = huma.Operation{
	OperationID: "upsertProject",
	Method:      http.MethodPut,
	Path:        "/projects",
	Summary:     "Upsert a project",
}

var OperationDeleteProject = huma.Operation{
	OperationID: "deleteProject",
	Method:      http.MethodPost,
	Path:        "/projects/delete",
	Summary:     "Delete a project",
}

type MergeProjectRequest struct {
	Body struct {
		SourceID int32 `json:"sourceId"`
		TargetID int32 `json:"targetId"`
	}
}

type MergeProjectResponse struct {
	Body ProjectView
}

var OperationMergeProject = huma.Operation{
	OperationID: "mergeProject",
	Method:      http.MethodPost,
	Path:        "/projects/merge",
	Summary:     "Merge one project into another",
}
