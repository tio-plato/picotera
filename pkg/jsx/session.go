package jsx

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"picotera/pkg/logx"
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
	registerHelpers(s)

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

// RunSortHook calls the sortProviders waterfall with the given input. Return
// semantics: if no tap mutated the value (passthrough), in.Providers is
// returned unchanged; if a tap returned an array of candidates, that array
// is returned; if a tap returned a `{providers: [...]}` shape, that array is
// returned. An empty array means "no providers" (gateway then fails 502).
func (s *Session) RunSortHook(in SortInput) ([]Candidate, error) {
	lit, err := marshalToJSLiteral(in)
	if err != nil {
		return nil, err
	}
	expr := fmt.Sprintf(`(async () => {
		const context = %s;
		const r = await picotera.hooks.sortProviders.runWaterfall(context, { providers: context.providers });
		if (r === context || typeof r === 'undefined') return null;
		return JSON.stringify(r);
	})()`, lit)
	jsonStr, err := s.runHookExpr("sortProviders.js", expr)
	if err != nil {
		return nil, err
	}
	if jsonStr == "" || jsonStr == "null" {
		return in.Providers, nil
	}
	// Try {providers: [...]} first.
	var obj struct {
		Providers []Candidate `json:"providers"`
	}
	if err := json.Unmarshal([]byte(jsonStr), &obj); err == nil && obj.Providers != nil {
		logx.WithContext(s.Context()).WithField("new_providers", obj.Providers).Debug("sortProviders hook returned new providers")
		return obj.Providers, nil
	}
	// Try direct array.
	var arr []Candidate
	if err := json.Unmarshal([]byte(jsonStr), &arr); err == nil {
		logx.WithContext(s.Context()).WithField("new_providers", arr).Debug("sortProviders hook returned new providers")
		return arr, nil
	}
	return in.Providers, nil
}

// RunBeforeRequestHook calls the beforeRequest waterfall and decodes the
// returned `{next, delay}` shape. Passthrough or a return that doesn't carry
// either key collapses to `Next=false, Delay=0`.
func (s *Session) RunBeforeRequestHook(in BeforeRequestInput) (BeforeRequestDecision, error) {
	var dec BeforeRequestDecision
	lit, err := marshalToJSLiteral(in)
	if err != nil {
		return dec, err
	}
	expr := fmt.Sprintf(`(async () => {
		const ctx = %s;
		const r = await picotera.hooks.beforeRequest.runWaterfall(ctx, { next: ctx.currentRetryCount > 0, delay: 0 });
		if (r === ctx || typeof r === 'undefined' || r === null) return null;
		return JSON.stringify({ next: !!r.next, delay: r.delay || 0 });
	})()`, lit)
	jsonStr, err := s.runHookExpr("beforeRequest.js", expr)
	if err != nil {
		return dec, err
	}
	if jsonStr == "" || jsonStr == "null" {
		if in.CurrentRetryCount > 0 {
			dec.Next = true
		}
		return dec, nil
	}
	if err := json.Unmarshal([]byte(jsonStr), &dec); err != nil {
		return dec, fmt.Errorf("jsx: beforeRequest decode: %w", err)
	}
	return dec, nil
}

// RunRewriteHook calls the rewriteRequest waterfall. Passthrough returns an
// empty RewriteOutput (all fields nil). If a tap returned `{body: <object>}`,
// the body is JSON-stringified at the JS boundary before reaching Go.
func (s *Session) RunRewriteHook(in RewriteInput) (RewriteOutput, error) {
	var out RewriteOutput
	lit, err := marshalToJSLiteral(in)
	if err != nil {
		return out, err
	}
	expr := fmt.Sprintf(`(async () => {
		const ctx = %s;
		const r = await picotera.hooks.rewriteRequest.runWaterfall(ctx, null);
		if (r === ctx || typeof r === 'undefined' || r === null) return null;
		if (r && typeof r.body === 'object' && r.body !== null) {
			return Object.assign({}, r, { body: JSON.stringify(r.body) });
		}
		return JSON.stringify(r);
	})()`, lit)
	jsonStr, err := s.runHookExpr("rewriteRequest.js", expr)
	if err != nil {
		return out, err
	}
	if jsonStr == "" || jsonStr == "null" {
		return out, nil
	}
	if err := json.Unmarshal([]byte(jsonStr), &out); err != nil {
		return out, fmt.Errorf("jsx: rewriteRequest decode: %w", err)
	}
	return out, nil
}

// runHookExpr evaluates the JS expression with HookTimeout, then
// JSON-serializes the resolved value (inside the same goroutine as the
// eval — fastschema/qjs values are not safe to use across goroutines).
// Returns the JSON string ready for json.Unmarshal.
func (s *Session) runHookExpr(name, expr string) (string, error) {
	if s.tainted {
		return "", ErrHookTimeout
	}
	type result struct {
		jsonStr string
		err     error
	}
	ch := make(chan result, 1)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				ch <- result{err: fmt.Errorf("jsx: eval panic: %v", r)}
			}
		}()
		v, err := s.rt.Context().Eval(name, qjs.Code(expr))
		if err != nil {
			ch <- result{err: err}
			return
		}
		if v.IsPromise() {
			v, err = v.Await()
			if err != nil {
				ch <- result{err: err}
				return
			}
		}
		defer v.Free()
		switch v.Type() {
		case "Undefined", "Null":
			ch <- result{jsonStr: "null"}
			return
		}

		jsonStr := v.String()
		ch <- result{jsonStr: jsonStr}
	}()

	timeout := s.engine.cfg.HookTimeout
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	select {
	case r := <-ch:
		return r.jsonStr, r.err
	case <-time.After(timeout):
		s.tainted = true
		s.cancel() // wakes the goroutine via panic; recover handles it
		<-ch       // drain
		return "", ErrHookTimeout
	}
}

// evalWithTimeout runs an Eval, racing against Engine.Config.HookTimeout.
// On timeout: cancels the runtime context (causing the in-flight Eval to
// panic via wazero's CloseOnContextDone), recovers, marks the session as
// tainted, returns ErrHookTimeout. Subsequent calls fail fast.
//
// NOTE: the returned *qjs.Value is bound to the goroutine that produced it.
// Use runHookExpr instead of touching the Value from another goroutine.
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
