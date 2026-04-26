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
