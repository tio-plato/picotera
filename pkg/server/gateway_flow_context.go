package server

import (
	"context"
	"net/http"
	"time"
)

const persistTimeout = 30 * time.Second

type gatewayContexts struct {
	Request     context.Context
	persistBase context.Context
	cancelBase  context.CancelFunc
}

func newGatewayContexts(r *http.Request) gatewayContexts {
	requestCtx := r.Context()
	persistBase, cancelBase := context.WithCancel(context.WithoutCancel(requestCtx))
	return gatewayContexts{
		Request:     requestCtx,
		persistBase: persistBase,
		cancelBase:  cancelBase,
	}
}

// Persist returns a fresh persistence context bounded by persistTimeout,
// starting now. Callers MUST defer the returned cancel.
func (c gatewayContexts) Persist() (context.Context, context.CancelFunc) {
	return context.WithTimeout(c.persistBase, persistTimeout)
}
