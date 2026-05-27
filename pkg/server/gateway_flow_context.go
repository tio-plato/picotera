package server

import (
	"context"
	"net/http"
	"time"

	"picotera/pkg/configx"
)

type gatewayContexts struct {
	Request       context.Context
	Persist       context.Context
	CancelPersist context.CancelFunc
}

func newGatewayContexts(r *http.Request, cfg *configx.Config) gatewayContexts {
	requestCtx := r.Context()
	persistBase := context.WithoutCancel(requestCtx)
	timeout := 5*time.Second + 2*time.Second
	if cfg != nil && cfg.GatewayReadTimeout > 5*time.Second {
		timeout = cfg.GatewayReadTimeout + 2*time.Second
	}
	persistCtx, cancel := context.WithTimeout(persistBase, timeout)
	return gatewayContexts{
		Request:       requestCtx,
		Persist:       persistCtx,
		CancelPersist: cancel,
	}
}
