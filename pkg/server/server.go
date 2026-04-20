package server

import (
	"context"
	"fmt"
	"net/http"
	"picotera/pkg/configx"
	"picotera/pkg/contract"
	"picotera/pkg/db"
	"picotera/pkg/logx"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humachi"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/sirupsen/logrus"

	_ "github.com/danielgtaylor/huma/v2/formats/cbor"
)

type Server struct {
	queries *db.Queries
	router  *chi.Mux
	api     huma.API
	config  *configx.Config
}

func NewServer(ctx context.Context) (*Server, error) {
	config, err := configx.Parse()
	if err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	conn, err := pgx.Connect(ctx, config.DatabaseURL)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}
	queries := db.New(conn)

	logx.WithContext(ctx).Info("connected to database")

	router := chi.NewMux()
	api := humachi.New(router, huma.DefaultConfig("PicoTera Management API", "1.0.0"))

	server := &Server{config: config, queries: queries, router: router, api: api}
	server.registerOperations()
	logx.WithContext(ctx).Info("registered operations")

	return server, nil
}

func NewHuma() huma.API {
	router := chi.NewMux()
	s := &Server{api: humachi.New(router, huma.DefaultConfig("PicoTera Management API", "1.0.0"))}
	s.registerOperations()
	return s.api
}

func (s *Server) registerOperations() {
	mgmt := huma.NewGroup(s.api, "/api/picotera")
	huma.Register(mgmt, contract.OperationGetProvider, s.handleGetProvider)
}

func (s *Server) Serve() error {
	logrus.WithField("host", s.config.Host).WithField("port", s.config.Port).Info("serving API")
	return http.ListenAndServe(fmt.Sprintf("%s:%d", s.config.Host, s.config.Port), s.router)
}
