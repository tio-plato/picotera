package server

import (
	"context"
	"fmt"
	"net/http"
	"picotera/db/migrations"
	"picotera/pkg/artifacts"
	"picotera/pkg/configx"
	"picotera/pkg/contract"
	"picotera/pkg/db"
	"picotera/pkg/jsx"
	"picotera/pkg/logx"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humachi"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/sirupsen/logrus"

	_ "github.com/danielgtaylor/huma/v2/formats/cbor"
)

type Server struct {
	queries    *db.Queries
	router     *chi.Mux
	api        huma.API
	config     *configx.Config
	httpClient *http.Client
	artifacts  artifacts.Sink
	jsxEngine  *jsx.Engine
}

func NewServer(ctx context.Context) (*Server, error) {
	config, err := configx.Parse()
	if err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	logx.WithContext(ctx).Info("running migrations")
	err = migrations.Up(config.DatabaseURL)
	if err != nil {
		logx.WithContext(ctx).WithError(err).Error("failed to run migrations")
		return nil, fmt.Errorf("failed to run migrations: %w", err)
	}
	logx.WithContext(ctx).Info("migrations completed")

	conn, err := pgxpool.New(ctx, config.DatabaseURL)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	queries := db.New(conn)

	logx.WithContext(ctx).Info("connected to database")

	httpClient := &http.Client{
		Transport: &http.Transport{
			ResponseHeaderTimeout: config.GatewayReadTimeout,
		},
	}

	sink, err := artifacts.NewSink(config.S3, logx.WithContext(ctx))
	if err != nil {
		logx.WithContext(ctx).WithError(err).Warn("failed to init artifact sink, continuing without artifacts")
		sink, _ = artifacts.NewSink(configx.S3Config{}, logx.WithContext(ctx))
	}

	router := chi.NewMux()
	api := humachi.New(router, huma.DefaultConfig("PicoTera Management API", "1.0.0"))

	jsxEngine := jsx.NewEngine(jsx.Config{
		HookTimeout:      config.JSHookTimeout,
		MemoryLimit:      config.JSMemoryLimit,
		MaxTotalAttempts: config.JSMaxTotalAttempts,
		MaxDelay:         config.JSMaxDelay,
	}, queries)

	server := &Server{config: config, queries: queries, router: router, api: api, httpClient: httpClient, artifacts: sink, jsxEngine: jsxEngine}
	server.registerOperations()
	server.registerEndpoints()
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
	huma.Register(mgmt, contract.OperationListProviders, s.handleListProviders)
	huma.Register(mgmt, contract.OperationGetProvider, s.handleGetProvider)
	huma.Register(mgmt, contract.OperationCreateProvider, s.handleCreateProvider)
	huma.Register(mgmt, contract.OperationUpsertProvider, s.handleUpsertProvider)
	huma.Register(mgmt, contract.OperationDeleteProvider, s.handleDeleteProvider)
	huma.Register(mgmt, contract.OperationListModels, s.handleListModels)
	huma.Register(mgmt, contract.OperationGetModel, s.handleGetModel)
	huma.Register(mgmt, contract.OperationPutModel, s.handlePutModel)
	huma.Register(mgmt, contract.OperationDeleteModel, s.handleDeleteModel)
	huma.Register(mgmt, contract.OperationListEndpoints, s.handleListEndpoints)
	huma.Register(mgmt, contract.OperationUpsertEndpoint, s.handleUpsertEndpoint)
	huma.Register(mgmt, contract.OperationDeleteEndpoint, s.handleDeleteEndpoint)
	huma.Register(mgmt, contract.OperationListModelProviderEndpoints, s.handleListModelProviderEndpoints)
	huma.Register(mgmt, contract.OperationGetModelProviderEndpoint, s.handleGetModelProviderEndpoint)
	huma.Register(mgmt, contract.OperationUpsertModelProviderEndpoint, s.handleUpsertModelProviderEndpoint)
	huma.Register(mgmt, contract.OperationDeleteModelProviderEndpoint, s.handleDeleteModelProviderEndpoint)
	huma.Register(mgmt, contract.OperationListProviderEndpoints, s.handleListProviderEndpoints)
	huma.Register(mgmt, contract.OperationUpsertProviderEndpoint, s.handleUpsertProviderEndpoint)
	huma.Register(mgmt, contract.OperationDeleteProviderEndpoint, s.handleDeleteProviderEndpoint)
	huma.Register(mgmt, contract.OperationListRequests, s.handleListRequests)
	huma.Register(mgmt, contract.OperationGetRequest, s.handleGetRequest)
	huma.Register(mgmt, contract.OperationListRequestSpans, s.handleListRequestSpans)
	huma.Register(mgmt, contract.OperationListScripts, s.handleListScripts)
	huma.Register(mgmt, contract.OperationGetScript, s.handleGetScript)
	huma.Register(mgmt, contract.OperationCreateScript, s.handleCreateScript)
	huma.Register(mgmt, contract.OperationUpdateScript, s.handleUpdateScript)
	huma.Register(mgmt, contract.OperationDeleteScript, s.handleDeleteScript)
}

func (s *Server) registerEndpoints() {
	s.router.Mount("/", &gatewayHandler{s})
}

func (s *Server) Serve() error {
	logrus.WithField("host", s.config.Host).WithField("port", s.config.Port).Info("serving API")
	return http.ListenAndServe(fmt.Sprintf("%s:%d", s.config.Host, s.config.Port), s.router)
}
