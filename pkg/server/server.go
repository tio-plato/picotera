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
	"picotera/pkg/llmbridge"
	"picotera/pkg/logx"
	"picotera/pkg/server/static"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humachi"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/sirupsen/logrus"

	_ "github.com/danielgtaylor/huma/v2/formats/cbor"
)

type Server struct {
	queries        *db.Queries
	router         *chi.Mux
	api            huma.API
	config         *configx.Config
	httpClient     *http.Client
	artifacts      artifacts.Sink
	jsxEngine      *jsx.Engine
	staticHandler  http.Handler
	endpointRouter *endpointRouter
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

	server := &Server{
		config:         config,
		queries:        queries,
		router:         router,
		api:            api,
		httpClient:     httpClient,
		artifacts:      sink,
		jsxEngine:      jsxEngine,
		staticHandler:  static.Handler(),
		endpointRouter: newEndpointRouter(queries),
	}
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
	huma.Register(mgmt, contract.OperationListProviderEndpoints, s.handleListProviderEndpoints)
	huma.Register(mgmt, contract.OperationUpsertProviderEndpoint, s.handleUpsertProviderEndpoint)
	huma.Register(mgmt, contract.OperationDeleteProviderEndpoint, s.handleDeleteProviderEndpoint)
	huma.Register(mgmt, contract.OperationFetchModels, s.handleFetchModels)
	huma.Register(mgmt, contract.OperationListRequests, s.handleListRequests)
	huma.Register(mgmt, contract.OperationListRequestTraces, s.handleListRequestTraces)
	huma.Register(mgmt, contract.OperationGetRequest, s.handleGetRequest)
	huma.Register(mgmt, contract.OperationListRequestSpans, s.handleListRequestSpans)
	huma.Register(mgmt, contract.OperationListExchangeRates, s.handleListExchangeRates)
	huma.Register(mgmt, contract.OperationGetExchangeRate, s.handleGetExchangeRate)
	huma.Register(mgmt, contract.OperationPutExchangeRate, s.handlePutExchangeRate)
	huma.Register(mgmt, contract.OperationDeleteExchangeRate, s.handleDeleteExchangeRate)
	huma.Register(mgmt, contract.OperationMatchPricing, s.handleMatchPricing)
	huma.Register(mgmt, contract.OperationListApiKeys, s.handleListApiKeys)
	huma.Register(mgmt, contract.OperationGetApiKey, s.handleGetApiKey)
	huma.Register(mgmt, contract.OperationCreateApiKey, s.handleCreateApiKey)
	huma.Register(mgmt, contract.OperationUpdateApiKey, s.handleUpdateApiKey)
	huma.Register(mgmt, contract.OperationDeleteApiKey, s.handleDeleteApiKey)
	huma.Register(mgmt, contract.OperationListScripts, s.handleListScripts)
	huma.Register(mgmt, contract.OperationGetScript, s.handleGetScript)
	huma.Register(mgmt, contract.OperationCreateScript, s.handleCreateScript)
	huma.Register(mgmt, contract.OperationUpdateScript, s.handleUpdateScript)
	huma.Register(mgmt, contract.OperationDeleteScript, s.handleDeleteScript)
}

func (s *Server) registerEndpoints() {
	// Unified generation routes. Registered BEFORE the catch-all gateway
	// mount so chi resolves them as exact-match handlers, never reaching
	// endpointRouter.Match. They route to handle_unified_gateway.go.
	s.router.Post("/api/picotera/v1/messages", s.handleUnifiedGenerate(llmbridge.FormatAnthropicMessages))
	s.router.Post("/api/picotera/v1/responses", s.handleUnifiedGenerate(llmbridge.FormatOpenAIResponses))
	s.router.Post("/api/picotera/v1/chat/completions", s.handleUnifiedGenerate(llmbridge.FormatOpenAIChatCompletions))
	s.router.Post("/api/picotera/v1beta/models/{model}:generateContent", s.handleUnifiedGenerate(llmbridge.FormatGeminiGenerateContent))
	s.router.Post("/api/picotera/v1beta/models/{model}:streamGenerateContent", s.handleUnifiedGenerate(llmbridge.FormatGeminiStreamGenerateContent))

	s.router.Mount("/", &gatewayHandler{s})
}

func (s *Server) Serve() error {
	logrus.WithField("host", s.config.Host).WithField("port", s.config.Port).Info("serving API")
	return http.ListenAndServe(fmt.Sprintf("%s:%d", s.config.Host, s.config.Port), s.router)
}
