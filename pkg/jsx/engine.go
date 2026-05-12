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

type Engine struct {
	cfg     Config
	store   ScriptStore
	kvStore kv.Store
}

func NewEngine(cfg Config, store ScriptStore, kvStore kv.Store) *Engine {
	return &Engine{cfg: cfg, store: store, kvStore: kvStore}
}

func (e *Engine) Config() Config { return e.cfg }

// NewSession creates a per-request JS session. The caller MUST call Close().
func (e *Engine) NewSession(ctx context.Context, requestID string) (*Session, error) {
	return newSession(ctx, e, requestID)
}
