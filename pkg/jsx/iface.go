package jsx

import "context"

// Engine constructs per-request JS sessions. The in-process implementation is
// backed by QuickJS; the interface leaves room for a future go-plugin (gRPC)
// implementation that the callers can hold without code changes.
type Engine interface {
	// NewSession creates a per-request JS session. The caller MUST call Close().
	NewSession(ctx context.Context, requestID string) (Session, error)
	Config() Config
}

// Session is a single meta-request's JS context. All method parameters and
// return values are JSON-serializable so a gRPC plugin can pass JSON strings.
//
// The persistent globalThis.ctx is shared for the whole session; PatchContext
// shallow-merges determined fields onto it as the request flow progresses.
// Each Run* method evaluates the corresponding waterfall against globalThis.ctx
// and the supplied initial value, returning the (possibly rewritten) value.
type Session interface {
	// PatchContext shallow-merges patch's non-nil fields onto globalThis.ctx,
	// preserving custom fields the scripts attached.
	PatchContext(patch ContextPatch) error

	RunRewriteModel(initial string) (string, error)
	RunSortProviders(initial []CandidateView) ([]CandidateView, error)
	RunBeforeRequest(initial BeforeRequestDecision) (BeforeRequestDecision, error)
	RunRewriteRequest(initial PendingRequestShape) (PendingRequestShape, error)
	RunBeforeTransform(initial OutboundProfile) (OutboundProfile, error)
	RunRewriteProviderModels(initial []ProviderModelEntry) ([]ProviderModelEntry, error)

	Logs() []LogEntry
	Close()
}
