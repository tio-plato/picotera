package jsx

import (
	"context"
	"time"

	"picotera/pkg/kv"
)

type Config struct {
	HookTimeout      time.Duration
	MemoryLimit      int64
	MaxTotalAttempts int
	MaxDelay         time.Duration
}

// qjsEngine is the in-process QuickJS implementation of Engine.
type qjsEngine struct {
	cfg     Config
	store   ScriptStore
	kvStore kv.Store
}

// NewEngine returns the in-process QuickJS Engine.
func NewEngine(cfg Config, store ScriptStore, kvStore kv.Store) Engine {
	return &qjsEngine{cfg: cfg, store: store, kvStore: kvStore}
}

func (e *qjsEngine) Config() Config { return e.cfg }

func (e *qjsEngine) NewSession(ctx context.Context, requestID string) (Session, error) {
	return newSession(ctx, e, requestID)
}
