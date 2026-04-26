package jsx

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/fastschema/qjs"
)

// ErrHookTimeout signals a hook ran past Engine.Config.HookTimeout. The
// session is then tainted and unusable for subsequent hooks.
var ErrHookTimeout = errors.New("jsx: hook timeout")

type Session struct {
	engine    *Engine
	rt        *qjs.Runtime
	cancel    context.CancelFunc
	requestID string
	closed    bool
	tainted   bool
}

func newSession(ctx context.Context, eng *Engine, requestID string) (*Session, error) {
	sessCtx, cancel := context.WithCancel(ctx)
	opt := qjs.Option{
		Context:            sessCtx,
		CloseOnContextDone: true,
		MemoryLimit:        int(eng.cfg.MemoryLimit),
	}
	rt, err := qjs.New(opt)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("jsx: qjs.New: %w", err)
	}
	s := &Session{engine: eng, rt: rt, cancel: cancel, requestID: requestID}

	c := rt.Context()
	if _, err := c.Eval("sdk.js", qjs.Code(sdkSource)); err != nil {
		s.Close()
		return nil, fmt.Errorf("jsx: eval sdk: %w", err)
	}

	scripts, err := eng.store.ListEnabledScripts(sessCtx)
	if err != nil {
		s.Close()
		return nil, fmt.Errorf("jsx: list scripts: %w", err)
	}
	for _, sc := range scripts {
		if _, err := c.Eval("script:"+sc.ID, qjs.Code(sc.Source)); err != nil {
			s.Close()
			return nil, fmt.Errorf("jsx: eval script %s: %w", sc.ID, err)
		}
	}
	return s, nil
}

// Close releases the underlying QuickJS runtime. Safe to call multiple times.
// If the session was tainted by a hook timeout, the runtime's wasm module is
// already gone — Close still tries rt.Close() but recovers any panic so the
// caller's `defer session.Close()` is always safe.
func (s *Session) Close() {
	if s.closed {
		return
	}
	s.closed = true
	if s.rt != nil {
		func() {
			defer func() { _ = recover() }()
			s.rt.Close()
		}()
		s.rt = nil
	}
	if s.cancel != nil {
		s.cancel()
	}
}

// Context returns the underlying qjs.Context for direct use within the package.
func (s *Session) Context() *qjs.Context {
	if s.rt == nil {
		return nil
	}
	return s.rt.Context()
}

// runHookExpr evaluates the JS expression with HookTimeout, then
// JSON-serializes the resolved value and returns the JSON string. This is
// the boundary used by all hook entrypoints to move data Go-ward.
func (s *Session) runHookExpr(name, expr string) (string, error) {
	v, err := s.evalWithTimeout(name, expr)
	if err != nil {
		return "", err
	}
	defer v.Free()
	if v.Type() == "Undefined" {
		return "null", nil
	}
	jsonStr, err := v.JSONStringify()
	if err != nil {
		return "", fmt.Errorf("jsx: stringify hook result: %w", err)
	}
	return jsonStr, nil
}

// evalWithTimeout runs an Eval, racing against Engine.Config.HookTimeout.
// On timeout: cancels the runtime context (causing the in-flight Eval to
// panic via wazero's CloseOnContextDone), recovers, marks the session as
// tainted, returns ErrHookTimeout. Subsequent calls fail fast.
func (s *Session) evalWithTimeout(name, src string) (*qjs.Value, error) {
	if s.tainted {
		return nil, ErrHookTimeout
	}
	type result struct {
		v   *qjs.Value
		err error
	}
	ch := make(chan result, 1)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				ch <- result{err: fmt.Errorf("jsx: eval panic: %v", r)}
			}
		}()
		v, err := s.rt.Context().Eval(name, qjs.Code(src))
		ch <- result{v: v, err: err}
	}()

	timeout := s.engine.cfg.HookTimeout
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	select {
	case r := <-ch:
		return r.v, r.err
	case <-time.After(timeout):
		s.tainted = true
		s.cancel() // wakes the goroutine via panic; recover handles it
		<-ch       // drain so we don't leak the channel send (goroutine will deliver shortly)
		return nil, ErrHookTimeout
	}
}
