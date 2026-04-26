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
