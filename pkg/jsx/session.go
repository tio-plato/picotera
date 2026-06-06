package jsx

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"picotera/pkg/logx"

	"modernc.org/quickjs"
)

// ErrHookTimeout signals a hook ran past Engine.Config.HookTimeout. The
// session is then tainted and unusable for subsequent hooks.
var ErrHookTimeout = errors.New("jsx: hook timeout")

// LogEntry is a single console.{log,info,warn,error,debug} call captured
// during a session. Times are UTC RFC3339Nano. debug is normalized to info.
type LogEntry struct {
	Level   string    `json:"level"`
	Message string    `json:"message"`
	Ts      time.Time `json:"ts"`
}

const (
	maxLogEntries    = 1000
	maxLogBytes      = 256 * 1024
	maxLogMessageLen = 8 * 1024
	logTruncSuffix   = "... [truncated]"
	logSentinelMsg   = "[picotera] log buffer truncated"
)

// qjsSession is the in-process QuickJS implementation of Session. The
// underlying *quickjs.VM is not concurrency-safe; a session is used
// sequentially within a single request flow.
type qjsSession struct {
	engine    *qjsEngine
	vm        *quickjs.VM
	ctx       context.Context
	requestID string
	closed    bool
	tainted   bool

	logsMu    sync.Mutex
	logs      []LogEntry
	logsBytes int
	logsTrunc bool
}

func (s *qjsSession) appendLog(level, message string) {
	switch level {
	case "info", "warn", "error":
	default:
		level = "info"
	}
	if len(message) > maxLogMessageLen {
		message = message[:maxLogMessageLen-len(logTruncSuffix)] + logTruncSuffix
	}
	s.logsMu.Lock()
	defer s.logsMu.Unlock()
	if s.logsTrunc {
		return
	}
	if len(s.logs) >= maxLogEntries || s.logsBytes+len(message) > maxLogBytes {
		s.logs = append(s.logs, LogEntry{
			Level:   "warn",
			Message: logSentinelMsg,
			Ts:      time.Now().UTC(),
		})
		s.logsTrunc = true
		return
	}
	s.logs = append(s.logs, LogEntry{
		Level:   level,
		Message: message,
		Ts:      time.Now().UTC(),
	})
	s.logsBytes += len(message)
}

// Logs returns a snapshot copy of the captured log entries. Must be called
// before Close.
func (s *qjsSession) Logs() []LogEntry {
	s.logsMu.Lock()
	defer s.logsMu.Unlock()
	if len(s.logs) == 0 {
		return nil
	}
	out := make([]LogEntry, len(s.logs))
	copy(out, s.logs)
	return out
}

// ctxInit is the JS that installs the persistent globalThis.ctx with all
// fields at their zero/null state. Fields are filled in later via PatchContext.
const ctxInit = `globalThis.ctx = {
	endpointType: null,
	endpoint: null,
	requestModel: null,
	routedModel: null,
	request: null,
	apiKey: null,
	provider: null,
	providerModel: null,
	attempt: null,
	annotations: {},
	stream: false,
	sourceFormat: ""
};`

func newSession(ctx context.Context, eng *qjsEngine, requestID string) (*qjsSession, error) {
	vm, err := quickjs.NewVM()
	if err != nil {
		return nil, fmt.Errorf("jsx: quickjs.NewVM: %w", err)
	}
	if eng.cfg.MemoryLimit > 0 {
		vm.SetMemoryLimit(uintptr(eng.cfg.MemoryLimit))
	}
	s := &qjsSession{engine: eng, vm: vm, ctx: ctx, requestID: requestID}
	registerHelpers(s)

	if _, err := vm.Eval(sdkSource, quickjs.EvalGlobal); err != nil {
		s.Close()
		return nil, fmt.Errorf("jsx: eval sdk: %w", err)
	}
	if _, err := vm.Eval(ctxInit, quickjs.EvalGlobal); err != nil {
		s.Close()
		return nil, fmt.Errorf("jsx: init ctx: %w", err)
	}

	scripts, err := eng.store.ListEnabledScripts(ctx)
	if err != nil {
		s.Close()
		return nil, fmt.Errorf("jsx: list scripts: %w", err)
	}
	for _, sc := range scripts {
		if _, err := vm.Eval(sc.Source, quickjs.EvalGlobal); err != nil {
			s.Close()
			return nil, fmt.Errorf("jsx: eval script %s: %w", sc.ID, err)
		}
	}
	return s, nil
}

// Close releases the underlying QuickJS VM. Safe to call multiple times.
func (s *qjsSession) Close() {
	if s.closed {
		return
	}
	s.closed = true
	if s.vm != nil {
		func() {
			defer func() { _ = recover() }()
			_ = s.vm.Close()
		}()
		s.vm = nil
	}
}

func (s *qjsSession) timeout() time.Duration {
	if s.engine.cfg.HookTimeout > 0 {
		return s.engine.cfg.HookTimeout
	}
	return 5 * time.Second
}

// PatchContext shallow-merges patch's non-nil fields onto globalThis.ctx.
func (s *qjsSession) PatchContext(patch ContextPatch) error {
	if s.tainted {
		return ErrHookTimeout
	}
	b, err := json.Marshal(patch)
	if err != nil {
		return fmt.Errorf("jsx: marshal context patch: %w", err)
	}
	if string(b) == "{}" {
		return nil
	}
	if _, err := s.vm.Eval("Object.assign(globalThis.ctx, "+string(b)+")", quickjs.EvalGlobal); err != nil {
		return fmt.Errorf("jsx: patch context: %w", err)
	}
	return nil
}

// evalJSON evaluates a hook IIFE and returns the result as JSON bytes.
// isUndefined is true when the IIFE returned `undefined` (passthrough). On a
// timeout/interrupt the session is tainted and ErrHookTimeout is returned.
func (s *qjsSession) evalJSON(name, expr string) (data json.RawMessage, isUndefined bool, err error) {
	if s.tainted {
		return nil, false, ErrHookTimeout
	}
	if terr := s.vm.SetEvalTimeout(s.timeout()); terr != nil {
		return nil, false, fmt.Errorf("jsx: %s set timeout: %w", name, terr)
	}
	v, err := s.vm.EvalValue(expr, quickjs.EvalGlobal)
	if err != nil {
		if isInterrupt(err) {
			s.tainted = true
			return nil, false, ErrHookTimeout
		}
		return nil, false, fmt.Errorf("jsx: %s: %w", name, err)
	}
	defer v.Free()
	if v.IsUndefined() {
		return nil, true, nil
	}
	b, merr := v.MarshalJSON()
	if merr != nil {
		return nil, false, fmt.Errorf("jsx: %s marshal: %w", name, merr)
	}
	// marshalJSON relies on the underlying JS value's GC lifetime, so we need to clone the bytes to avoid use-after-free.
	b = bytes.Clone(b)
	return json.RawMessage(b), false, nil
}

func isInterrupt(err error) bool {
	return err != nil && strings.Contains(err.Error(), "interrupted")
}

func mustJSON(v any) (string, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return "", fmt.Errorf("jsx: marshal initial: %w", err)
	}
	return string(b), nil
}

// RunRewriteModel runs the rewriteModel waterfall. A non-string result keeps
// the initial value.
func (s *qjsSession) RunRewriteModel(initial string) (string, error) {
	init, err := mustJSON(initial)
	if err != nil {
		return initial, err
	}
	expr := `(function () {
		var r = picotera.hooks.rewriteModel.runWaterfall(globalThis.ctx, ` + init + `);
		if (typeof r !== 'string') return undefined;
		return r;
	})()`
	data, undef, err := s.evalJSON("rewriteModel", expr)
	if err != nil || undef {
		return initial, err
	}
	var out string
	if err := json.Unmarshal(data, &out); err != nil {
		return initial, fmt.Errorf("jsx: rewriteModel decode: %w", err)
	}
	return out, nil
}

// RunSortProviders runs the sortProviders waterfall. Passthrough keeps the
// initial list. A returned array (or {providers: [...]}) replaces it; an empty
// array means "no providers".
func (s *qjsSession) RunSortProviders(initial []CandidateView) ([]CandidateView, error) {
	init, err := mustJSON(initial)
	if err != nil {
		return initial, err
	}
	expr := `(function () {
		var initial = ` + init + `;
		var r = picotera.hooks.sortProviders.runWaterfall(globalThis.ctx, initial);
		if (r === globalThis.ctx || typeof r === 'undefined' || r === null) return undefined;
		if (Array.isArray(r)) return r;
		if (r && Array.isArray(r.providers)) return r.providers;
		return undefined;
	})()`
	data, undef, err := s.evalJSON("sortProviders", expr)
	if err != nil || undef {
		return initial, err
	}
	var out []CandidateView
	if err := json.Unmarshal(data, &out); err != nil {
		logx.WithContext(s.ctx).WithError(err).Debug("sortProviders hook returned undecodable value; keeping input")
		return initial, nil
	}
	return out, nil
}

// RunBeforeRequest runs the beforeRequest waterfall. Passthrough keeps the
// initial decision. A non-string upstreamModel is dropped at the JS boundary.
func (s *qjsSession) RunBeforeRequest(initial BeforeRequestDecision) (BeforeRequestDecision, error) {
	init, err := mustJSON(initial)
	if err != nil {
		return initial, err
	}
	expr := `(function () {
		var r = picotera.hooks.beforeRequest.runWaterfall(globalThis.ctx, ` + init + `);
		if (r === globalThis.ctx || typeof r === 'undefined' || r === null) return undefined;
		var um = (typeof r.upstreamModel === 'string') ? r.upstreamModel : '';
		return { next: !!r.next, delay: r.delay || 0, upstreamModel: um };
	})()`
	data, undef, err := s.evalJSON("beforeRequest", expr)
	if err != nil || undef {
		return initial, err
	}
	var out BeforeRequestDecision
	if err := json.Unmarshal(data, &out); err != nil {
		return initial, fmt.Errorf("jsx: beforeRequest decode: %w", err)
	}
	return out, nil
}

// RunRewriteRequest runs the rewriteRequest waterfall. The hook must return a
// complete pending request; passthrough keeps the initial. A non-string body
// is JSON.stringify'd at the JS boundary so Go always receives a JSON string
// token in Body when the field is present.
func (s *qjsSession) RunRewriteRequest(initial PendingRequestShape) (PendingRequestShape, error) {
	init, err := mustJSON(initial)
	if err != nil {
		return initial, err
	}
	expr := `(function () {
		var initial = ` + init + `;
		var r = picotera.hooks.rewriteRequest.runWaterfall(globalThis.ctx, initial);
		var v = (typeof r === 'undefined' || r === null) ? initial : r;
		if (v && typeof v.body !== 'undefined' && typeof v.body !== 'string') {
			v.body = JSON.stringify(v.body);
		}
		return v;
	})()`
	data, undef, err := s.evalJSON("rewriteRequest", expr)
	if err != nil || undef {
		return initial, err
	}
	var out PendingRequestShape
	if err := json.Unmarshal(data, &out); err != nil {
		return initial, fmt.Errorf("jsx: rewriteRequest decode: %w", err)
	}
	return out, nil
}

// RunBeforeTransform runs the beforeTransform waterfall. The contract is strict
// because its result drives bridge construction: taps must return an object,
// with string type and object config when present.
func (s *qjsSession) RunBeforeTransform(initial OutboundProfile) (OutboundProfile, error) {
	if initial.Config == nil {
		initial.Config = map[string]any{}
	}
	init, err := mustJSON(initial)
	if err != nil {
		return initial, err
	}
	expr := `(function () {
		var initial = ` + init + `;
		var r = picotera.hooks.beforeTransform.runWaterfall(globalThis.ctx, initial);
		if (typeof r === 'undefined' || r === null) return undefined;
		if (typeof r !== 'object' || Array.isArray(r)) {
			throw new Error("jsx: beforeTransform result must be object");
		}
		if (Object.prototype.hasOwnProperty.call(r, "type") && typeof r.type !== 'string') {
			throw new Error("jsx: beforeTransform type must be string");
		}
		if (Object.prototype.hasOwnProperty.call(r, "config")) {
			if (r.config === null || typeof r.config !== 'object' || Array.isArray(r.config)) {
				throw new Error("jsx: beforeTransform config must be object");
			}
		}
		return {
			type: Object.prototype.hasOwnProperty.call(r, "type") ? r.type : initial.type,
			config: Object.prototype.hasOwnProperty.call(r, "config") ? r.config : {}
		};
	})()`
	data, undef, err := s.evalJSON("beforeTransform", expr)
	if err != nil || undef {
		return initial, err
	}
	var out OutboundProfile
	if err := json.Unmarshal(data, &out); err != nil {
		return initial, fmt.Errorf("jsx: beforeTransform decode: %w", err)
	}
	if out.Config == nil {
		out.Config = map[string]any{}
	}
	return out, nil
}

// RunRewriteProviderModels runs the rewriteProviderModels waterfall. A
// non-array / undefined result, or an undecodable array, keeps the input.
func (s *qjsSession) RunRewriteProviderModels(initial []ProviderModelEntry) ([]ProviderModelEntry, error) {
	init, err := mustJSON(initial)
	if err != nil {
		return initial, err
	}
	expr := `(function () {
		var initial = ` + init + `;
		var r = picotera.hooks.rewriteProviderModels.runWaterfall(globalThis.ctx, initial);
		if (typeof r === 'undefined' || r === null || !Array.isArray(r)) return undefined;
		return r;
	})()`
	data, undef, err := s.evalJSON("rewriteProviderModels", expr)
	if err != nil || undef {
		return initial, err
	}
	var out []ProviderModelEntry
	if err := json.Unmarshal(data, &out); err != nil {
		return initial, nil
	}
	return out, nil
}
