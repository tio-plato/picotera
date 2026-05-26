package server

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"picotera/db/migrations"
	"picotera/pkg/artifacts"
	"picotera/pkg/configx"
	"picotera/pkg/contract"
	"picotera/pkg/db"
	"picotera/pkg/jsx"
	"picotera/pkg/kv"
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
	queries          *db.Queries
	router           *chi.Mux
	api              huma.API
	config           *configx.Config
	httpClient       *http.Client
	proxyCache       *proxyTransportCache
	artifacts        artifacts.Sink
	jsxEngine        *jsx.Engine
	kvStore          kv.Store
	staticHandler    http.Handler
	endpointRouter   *endpointRouter
	projectRouter    *projectRouter
	projectExtractor *projectExtractor
	llmBridge        llmbridge.Bridge
}

func NewServer(ctx context.Context) (*Server, error) {
	config, err := configx.Parse()
	if err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	logx.WithContext(ctx).Info("running migrations")
	migrationResult, err := migrations.UpWithResult(config.DatabaseURL)
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
	if migrationResult.PreviousVersion < traceBackfillMigrationVersion && migrationResult.CurrentVersion >= traceBackfillMigrationVersion {
		logx.WithContext(ctx).Info("backfilling historical traces")
		if err := backfillTraces(ctx, queries); err != nil {
			conn.Close()
			return nil, fmt.Errorf("failed to backfill traces: %w", err)
		}
	} else {
		logx.WithContext(ctx).WithFields(logrus.Fields{
			"previousVersion": migrationResult.PreviousVersion,
			"currentVersion":  migrationResult.CurrentVersion,
		}).Debug("skipping historical trace backfill")
	}

	logx.WithContext(ctx).Info("connected to database")

	baseTransport := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   config.GatewayDialTimeout,
			KeepAlive: config.GatewayDialKeepAlive,
		}).DialContext,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          100,
		IdleConnTimeout:       config.GatewayIdleConnTimeout,
		TLSHandshakeTimeout:   config.GatewayTLSHandshakeTimeout,
		ExpectContinueTimeout: config.GatewayExpectContinueTimeout,
		ResponseHeaderTimeout: config.GatewayResponseHeaderTimeout,
	}
	httpClient := &http.Client{Transport: baseTransport}
	proxyCache := newProxyTransportCache(baseTransport)

	sink, err := artifacts.NewSink(config.S3, logx.WithContext(ctx))
	if err != nil {
		logx.WithContext(ctx).WithError(err).Warn("failed to init artifact sink, continuing without artifacts")
		sink, _ = artifacts.NewSink(configx.S3Config{}, logx.WithContext(ctx))
	}

	router := chi.NewMux()
	api := humachi.New(router, huma.DefaultConfig("PicoTera Management API", "1.0.0"))

	kvStore, err := kv.New(config.KV.Driver, kv.WithRedisURL(config.KV.RedisURL))
	if err != nil {
		return nil, fmt.Errorf("failed to create kv store: %w", err)
	}

	jsxEngine := jsx.NewEngine(jsx.Config{
		HookTimeout:      config.JSHookTimeout,
		MemoryLimit:      config.JSMemoryLimit,
		MaxTotalAttempts: config.JSMaxTotalAttempts,
		MaxDelay:         config.JSMaxDelay,
	}, queries, kvStore)

	projectRouter := newProjectRouter(queries)
	if config.LLMBridgeWASMPath != "" {
		cacheDir := config.LLMBridgeWASMCacheDir
		if cacheDir == "" {
			cacheDir = llmbridge.DefaultCacheDir(config.LLMBridgeWASMPath)
		}
		logx.WithContext(ctx).WithFields(logrus.Fields{
			"path":      config.LLMBridgeWASMPath,
			"cache_dir": cacheDir,
			"runtime":   config.LLMBridgeWASMRuntime,
			"pool":      config.LLMBridgeWASMPoolSize,
		}).Info("prewarming llmbridge wasm")
	}
	llmBridge, err := llmbridge.New(ctx, llmbridge.Config{
		PoolSize:    config.LLMBridgeWASMPoolSize,
		WASMPath:    config.LLMBridgeWASMPath,
		CacheDir:    config.LLMBridgeWASMCacheDir,
		RuntimeMode: config.LLMBridgeWASMRuntime,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to initialize llmbridge: %w", err)
	}
	server := &Server{
		config:           config,
		queries:          queries,
		router:           router,
		api:              api,
		httpClient:       httpClient,
		proxyCache:       proxyCache,
		artifacts:        sink,
		jsxEngine:        jsxEngine,
		kvStore:          kvStore,
		staticHandler:    static.Handler(),
		endpointRouter:   newEndpointRouter(queries),
		projectRouter:    projectRouter,
		projectExtractor: newProjectExtractor(projectRouter),
		llmBridge:        llmBridge,
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
	huma.Register(mgmt, contract.OperationUpdateProviderModels, s.handleUpdateProviderModels)
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
	huma.Register(mgmt, contract.OperationGetOverviewSummary, s.handleGetOverviewSummary)
	huma.Register(mgmt, contract.OperationGetOverviewDistribution, s.handleGetOverviewDistribution)
	huma.Register(mgmt, contract.OperationGetOverviewSeries, s.handleGetOverviewSeries)
	huma.Register(mgmt, contract.OperationListProjects, s.handleListProjects)
	huma.Register(mgmt, contract.OperationGetProject, s.handleGetProject)
	huma.Register(mgmt, contract.OperationUpsertProject, s.handleUpsertProject)
	huma.Register(mgmt, contract.OperationDeleteProject, s.handleDeleteProject)
	huma.Register(mgmt, contract.OperationListScripts, s.handleListScripts)
	huma.Register(mgmt, contract.OperationGetScript, s.handleGetScript)
	huma.Register(mgmt, contract.OperationCreateScript, s.handleCreateScript)
	huma.Register(mgmt, contract.OperationUpdateScript, s.handleUpdateScript)
	huma.Register(mgmt, contract.OperationDeleteScript, s.handleDeleteScript)
	huma.Register(mgmt, contract.OperationSimulateDispatch, s.handleSimulateDispatch)
	huma.Register(mgmt, contract.OperationListKvEntries, s.handleListKvEntries)
	huma.Register(mgmt, contract.OperationGetKvEntry, s.handleGetKvEntry)
	huma.Register(mgmt, contract.OperationUpsertKvEntry, s.handleUpsertKvEntry)
	huma.Register(mgmt, contract.OperationDeleteKvEntry, s.handleDeleteKvEntry)
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
