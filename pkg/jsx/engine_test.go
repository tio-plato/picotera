package jsx

import (
	"context"
	"testing"
	"time"

	"picotera/pkg/db"

	"github.com/fastschema/qjs"
)

type fakeStore struct{ scripts []db.Script }

func (f *fakeStore) ListEnabledScripts(_ context.Context) ([]db.Script, error) {
	return f.scripts, nil
}

func TestEngine_LoadsScripts(t *testing.T) {
	store := &fakeStore{scripts: []db.Script{
		{ID: "a", Source: `picotera.hooks.sortProviders.tap("a", function (ctx) { return ctx; });`},
		{ID: "b", Source: `picotera.hooks.sortProviders.tap("b", function (ctx) { return ctx; });`},
	}}
	eng := NewEngine(Config{HookTimeout: time.Second, MemoryLimit: 64 * 1024 * 1024}, store)
	s, err := eng.NewSession(context.Background(), "")
	if err != nil {
		t.Fatalf("NewSession: %v", err)
	}
	defer s.Close()

	v, err := s.Context().Eval("probe.js", qjs.Code("picotera.hooks.sortProviders._taps.length"))
	if err != nil {
		t.Fatalf("eval: %v", err)
	}
	defer v.Free()
	if got := v.Int32(); got != 2 {
		t.Errorf("want 2 taps, got %d", got)
	}
}

func TestSession_CloseIdempotent(t *testing.T) {
	store := &fakeStore{}
	eng := NewEngine(Config{HookTimeout: time.Second, MemoryLimit: 64 * 1024 * 1024}, store)
	s, err := eng.NewSession(context.Background(), "")
	if err != nil {
		t.Fatalf("NewSession: %v", err)
	}
	s.Close()
	s.Close()
	s.Close()
}

func newTestSession(t *testing.T, scripts ...db.Script) *Session {
	t.Helper()
	eng := NewEngine(Config{HookTimeout: 500 * time.Millisecond, MemoryLimit: 64 * 1024 * 1024}, &fakeStore{scripts: scripts})
	s, err := eng.NewSession(context.Background(), "")
	if err != nil {
		t.Fatalf("NewSession: %v", err)
	}
	t.Cleanup(s.Close)
	return s
}

func TestSession_Promise_ResolvesValue(t *testing.T) {
	s := newTestSession(t, db.Script{ID: "a", Source: `globalThis.giveMe42 = function () { return Promise.resolve(42); }`})
	v, err := s.evalWithTimeout("call.js", "giveMe42()")
	if err != nil {
		t.Fatalf("evalWithTimeout: %v", err)
	}
	defer v.Free()
	if got := v.Int32(); got != 42 {
		t.Errorf("want 42, got %d", got)
	}
}

func TestSession_Promise_PropagatesRejection(t *testing.T) {
	s := newTestSession(t, db.Script{ID: "a", Source: `globalThis.boom = function () { return Promise.reject(new Error("boom message")); }`})
	_, err := s.evalWithTimeout("call.js", "boom()")
	if err == nil {
		t.Fatalf("want rejection, got nil")
	}
	if !contains(err.Error(), "boom message") {
		t.Errorf("error should mention `boom message`: %v", err)
	}
}

func TestSession_Promise_Timeout(t *testing.T) {
	// Tight loop ignoring the eval timeout — runaway script.
	s := newTestSession(t, db.Script{ID: "a", Source: `globalThis.loop = function () { for(;;){} }`})
	start := time.Now()
	_, err := s.evalWithTimeout("call.js", "loop()")
	elapsed := time.Since(start)
	if err == nil {
		t.Fatalf("want ErrHookTimeout, got nil")
	}
	if !errIs(err, ErrHookTimeout) {
		t.Errorf("want ErrHookTimeout, got %v", err)
	}
	if elapsed > 2*time.Second {
		t.Errorf("timeout took too long: %v", elapsed)
	}
}

func TestSession_CtxRoundTrip(t *testing.T) {
	s := newTestSession(t, db.Script{ID: "rev", Source: `
		picotera.hooks.sortProviders.tap("rev", function (ctx) {
			return { providers: ctx.providers.slice().reverse() };
		});
	`})

	in := map[string]any{"providers": []map[string]any{
		{"id": 1}, {"id": 2}, {"id": 3},
	}}
	lit, err := marshalToJSLiteral(in)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	expr := "picotera.hooks.sortProviders.runWaterfall(" + lit + ")"
	jsonStr, err := s.runHookExpr("rev.js", expr)
	if err != nil {
		t.Fatalf("runHookExpr: %v", err)
	}
	var out struct {
		Providers []struct{ ID int } `json:"providers"`
	}
	if err := unmarshalJSON(jsonStr, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(out.Providers) != 3 || out.Providers[0].ID != 3 || out.Providers[2].ID != 1 {
		t.Errorf("want reversed order, got %+v", out.Providers)
	}
}

func TestSession_Hooks_Sort(t *testing.T) {
	s := newTestSession(t, db.Script{ID: "a", Source: `
		picotera.hooks.sortProviders.tap("a", function (ctx) {
			return ctx.providers.slice().reverse();
		});
	`})
	out, err := s.RunSortHook(SortInput{
		Providers: []Candidate{
			{Provider: map[string]any{"id": 1}, MPE: map[string]any{"providerId": 1}},
			{Provider: map[string]any{"id": 2}, MPE: map[string]any{"providerId": 2}},
		},
	})
	if err != nil {
		t.Fatalf("RunSortHook: %v", err)
	}
	if len(out) != 2 {
		t.Fatalf("want 2, got %d", len(out))
	}
	pm := out[0].Provider.(map[string]any)
	if int(pm["id"].(float64)) != 2 {
		t.Errorf("want first id=2 after reverse, got %v", pm["id"])
	}
}

func TestSession_Hooks_Sort_Passthrough(t *testing.T) {
	s := newTestSession(t, db.Script{ID: "a", Source: `
		picotera.hooks.sortProviders.tap("a", function () {});
	`})
	in := []Candidate{{Provider: map[string]any{"id": 1}}}
	out, err := s.RunSortHook(SortInput{Providers: in})
	if err != nil {
		t.Fatalf("RunSortHook: %v", err)
	}
	if len(out) != 1 {
		t.Fatalf("want passthrough preserves input, got %d entries", len(out))
	}
}

func TestSession_Hooks_BeforeRequest_Defaults(t *testing.T) {
	s := newTestSession(t, db.Script{ID: "a", Source: `
		picotera.hooks.beforeRequest.tap("a", function () {});
	`})
	dec, err := s.RunBeforeRequestHook(BeforeRequestInput{})
	if err != nil {
		t.Fatalf("RunBeforeRequestHook: %v", err)
	}
	if dec.Next || dec.Delay != 0 {
		t.Errorf("want defaults {Next:false, Delay:0}, got %+v", dec)
	}
}

func TestSession_Hooks_BeforeRequest_NextAndDelay(t *testing.T) {
	s := newTestSession(t, db.Script{ID: "a", Source: `
		picotera.hooks.beforeRequest.tap("a", function () { return { next: true, delay: 100 }; });
	`})
	dec, err := s.RunBeforeRequestHook(BeforeRequestInput{})
	if err != nil {
		t.Fatalf("RunBeforeRequestHook: %v", err)
	}
	if !dec.Next || dec.Delay != 100 {
		t.Errorf("want {Next:true, Delay:100}, got %+v", dec)
	}
}

func TestSession_Hooks_Rewrite_BodyObjectStringified(t *testing.T) {
	s := newTestSession(t, db.Script{ID: "a", Source: `
		picotera.hooks.rewriteRequest.tap("a", function () {
			return { body: { hello: "world" } };
		});
	`})
	out, err := s.RunRewriteHook(RewriteInput{
		UpstreamRequest: UpstreamRequestShape{URL: "https://x", Method: "POST"},
	})
	if err != nil {
		t.Fatalf("RunRewriteHook: %v", err)
	}
	want := `"{\"hello\":\"world\"}"`
	if string(out.Body) != want {
		t.Errorf("want body=%s, got %s", want, string(out.Body))
	}
}

func TestSession_Hooks_Rewrite_Passthrough(t *testing.T) {
	s := newTestSession(t, db.Script{ID: "a", Source: `
		picotera.hooks.rewriteRequest.tap("a", function () {});
	`})
	out, err := s.RunRewriteHook(RewriteInput{})
	if err != nil {
		t.Fatalf("RunRewriteHook: %v", err)
	}
	if out.URL != nil || out.Method != nil || out.Headers != nil || len(out.Body) > 0 {
		t.Errorf("want all-nil passthrough, got %+v", out)
	}
}

func contains(s, substr string) bool {
	for i := 0; i+len(substr) <= len(s); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func errIs(err, target error) bool {
	for e := err; e != nil; {
		if e == target {
			return true
		}
		u, ok := e.(interface{ Unwrap() error })
		if !ok {
			return false
		}
		e = u.Unwrap()
	}
	return false
}
