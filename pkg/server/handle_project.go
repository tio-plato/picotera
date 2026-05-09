package server

import (
	"context"
	"encoding/json"
	"errors"

	"picotera/pkg/contract"
	"picotera/pkg/db"

	"github.com/danielgtaylor/huma/v2"
	"github.com/jackc/pgx/v5"
)

func (s *Server) handleListProjects(ctx context.Context, _ *struct{}) (*contract.ListProjectsResponse, error) {
	rows, err := s.queries.ListProjects(ctx)
	if err != nil {
		return nil, huma.Error500InternalServerError("failed to list projects", err)
	}
	out := make([]contract.ProjectView, len(rows))
	for i := range rows {
		v, err := contract.ToProjectView(&rows[i])
		if err != nil {
			return nil, huma.Error500InternalServerError("failed to decode project", err)
		}
		out[i] = *v
	}
	return &contract.ListProjectsResponse{Body: out}, nil
}

func (s *Server) handleGetProject(ctx context.Context, in *contract.GetProjectRequest) (*contract.GetProjectResponse, error) {
	r, err := s.queries.GetProject(ctx, in.ID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, huma.Error404NotFound("project not found")
		}
		return nil, huma.Error500InternalServerError("failed to get project", err)
	}
	v, err := contract.ToProjectView(&r)
	if err != nil {
		return nil, huma.Error500InternalServerError("failed to decode project", err)
	}
	return &contract.GetProjectResponse{Body: *v}, nil
}

func (s *Server) handleUpsertProject(ctx context.Context, in *contract.UpsertProjectRequest) (*contract.UpsertProjectResponse, error) {
	if in.Body.Name == "" {
		return nil, huma.Error400BadRequest("name is required")
	}
	paths := in.Body.Paths
	if paths == nil {
		paths = []string{}
	}
	for _, p := range paths {
		if p == "" {
			return nil, huma.Error400BadRequest("path entries must not be empty")
		}
	}
	pathsJSON, err := json.Marshal(paths)
	if err != nil {
		return nil, huma.Error500InternalServerError("failed to encode paths", err)
	}

	var row db.Project
	if in.Body.ID == 0 {
		row, err = s.queries.InsertProject(ctx, db.InsertProjectParams{
			Name:  in.Body.Name,
			Paths: pathsJSON,
		})
	} else {
		row, err = s.queries.UpdateProject(ctx, db.UpdateProjectParams{
			ID:    in.Body.ID,
			Name:  in.Body.Name,
			Paths: pathsJSON,
		})
	}
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, huma.Error404NotFound("project not found")
		}
		if isUniqueViolation(err) {
			return nil, huma.Error409Conflict("name already exists")
		}
		return nil, huma.Error500InternalServerError("failed to upsert project", err)
	}

	s.projectRouter.Invalidate()

	v, err := contract.ToProjectView(&row)
	if err != nil {
		return nil, huma.Error500InternalServerError("failed to decode project", err)
	}
	return &contract.UpsertProjectResponse{Body: *v}, nil
}

func (s *Server) handleDeleteProject(ctx context.Context, in *contract.DeleteProjectRequest) (*struct{}, error) {
	if err := s.queries.DeleteProject(ctx, in.Body.ID); err != nil {
		return nil, huma.Error500InternalServerError("failed to delete project", err)
	}
	s.projectRouter.Invalidate()
	return &struct{}{}, nil
}
