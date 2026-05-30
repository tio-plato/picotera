package server

import (
	"context"
	"net/http"
	"time"
)

const persistTimeout = 30 * time.Second

type gatewayContexts struct {
	Request       context.Context
	cancelRequest context.CancelFunc
	persistBase   context.Context
	cancelBase    context.CancelFunc
}

func newGatewayContexts(r *http.Request) gatewayContexts {
	requestCtx := r.Context()
	// Request is cancellable so the dashboard can interrupt the whole flow.
	// persistBase derives from WithoutCancel so persistence and artifact
	// uploads still complete after an interrupt.
	cancellableRequest, cancelRequest := context.WithCancel(requestCtx)
	persistBase, cancelBase := context.WithCancel(context.WithoutCancel(requestCtx))
	return gatewayContexts{
		Request:       cancellableRequest,
		cancelRequest: cancelRequest,
		persistBase:   persistBase,
		cancelBase:    cancelBase,
	}
}

// Persist returns a fresh persistence context bounded by persistTimeout,
// starting now. Callers MUST defer the returned cancel.
func (c gatewayContexts) Persist() (context.Context, context.CancelFunc) {
	return context.WithTimeout(c.persistBase, persistTimeout)
}
