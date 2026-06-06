package jsx

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"picotera/pkg/db"
	"picotera/pkg/kv"
)

type fakeStore struct{ scripts []db.Script }

func (f *fakeStore) ListEnabledScripts(_ context.Context) ([]db.Script, error) {
	return f.scripts, nil
}

func newTestEngine(t *testing.T, scripts ...db.Script) Engine {
	t.Helper()
	return NewEngine(
		Config{HookTimeout: 500 * time.Millisecond, MemoryLimit: 64 * 1024 * 1024},
		&fakeStore{scripts: scripts},
		kv.NewMemoryStore(),
	)
}

func newTestSession(t *testing.T, scripts ...db.Script) Session {
	t.Helper()
	s, err := newTestEngine(t, scripts...).NewSession(context.Background(), "test-req")
	if err != nil {
		t.Fatalf("NewSession: %v", err)
	}
	t.Cleanup(s.Close)
	return s
}

func TestEngine_LoadsScripts(t *testing.T) {
	// Two taps registered across two scripts both fire in priority order.
	s := newTestSession(t,
		db.Script{ID: "a", Source: `picotera.hooks.rewriteModel.tap("a", function (ctx, m) { return m + "-a"; }, 1);`},
		db.Script{ID: "b", Source: `picotera.hooks.rewriteModel.tap("b", function (ctx, m) { return m + "-b"; }, 10);`},
	)
	out, err := s.RunRewriteModel("base")
	if err != nil {
		t.Fatalf("RunRewriteModel: %v", err)
	}
	if out != "base-a-b" {
		t.Errorf("want priority waterfall base-a-b, got %q", out)
	}
}

func TestSession_CloseIdempotent(t *testing.T) {
	s := newTestSession(t)
	s.Close()
	s.Close()
	s.Close()
}

func TestSession_BadScript_FailsSession(t *testing.T) {
	_, err := newTestEngine(t, db.Script{ID: "bad", Source: `this is not valid javascript @@@`}).
		NewSession(context.Background(), "")
	if err == nil {
		t.Fatalf("want error loading invalid script, got nil")
	}
	got := err.Error()
	if !strings.Contains(got, "jsx: eval script bad") || !strings.Contains(got, "script:bad") {
		t.Fatalf("error did not include script id filename: %v", got)
	}
}

func TestSession_LoadErrorIncludesScriptFilenameAndLine(t *testing.T) {
	_, err := newTestEngine(t, db.Script{ID: "script-load-fail", Source: "var ok = 1;\nvar bad = ;"}).
		NewSession(context.Background(), "")
	if err == nil {
		t.Fatalf("want error loading invalid script, got nil")
	}
	got := err.Error()
	for _, want := range []string{"eval script script-load-fail", "script:script-load-fail", ":2:"} {
		if !strings.Contains(got, want) {
			t.Fatalf("error missing %q: %v", want, got)
		}
	}
}

func TestSession_RuntimeErrorIncludesScriptStackAndTapName(t *testing.T) {
	s := newTestSession(t, db.Script{ID: "script-runtime-fail", Source: `
function failFromUserScript() {
  throw new Error("runtime boom");
}
picotera.hooks.rewriteModel.tap("runtime-tap", function (ctx, m) {
  failFromUserScript();
  return m;
});
`})
	_, err := s.RunRewriteModel("base")
	if err == nil {
		t.Fatalf("want runtime error, got nil")
	}
	got := err.Error()
	for _, want := range []string{"script:script-runtime-fail", ":3:"} {
		if !strings.Contains(got, want) {
			t.Fatalf("error missing %q: %v", want, got)
		}
	}
}

func TestSession_PatchContext_VisibleToHook(t *testing.T) {
	s := newTestSession(t, db.Script{ID: "a", Source: `
		picotera.hooks.rewriteModel.tap("a", function (ctx) {
			return ctx.requestModel + "/" + ctx.routedModel.name + "/" + ctx.provider.name;
		});
	`})
	rm := "req-model"
	if err := s.PatchContext(ContextPatch{
		RequestModel: &rm,
		RoutedModel:  &ModelSummary{Name: "routed", Annotations: map[string]string{}},
		Provider:     &ProviderSummary{ID: 7, Name: "prov"},
	}); err != nil {
		t.Fatalf("PatchContext: %v", err)
	}
	out, err := s.RunRewriteModel("ignored")
	if err != nil {
		t.Fatalf("RunRewriteModel: %v", err)
	}
	if out != "req-model/routed/prov" {
		t.Errorf("hook did not see patched ctx, got %q", out)
	}
}

func TestSession_PatchContext_PreservesCustomFields(t *testing.T) {
	// A hook stashes a value on ctx; a later PatchContext (Object.assign) must
	// not wipe it, and a later hook can read it back.
	s := newTestSession(t, db.Script{ID: "a", Source: `
		picotera.hooks.rewriteModel.tap("a", function (ctx, m) { ctx.mine = 99; return m; });
		picotera.hooks.beforeRequest.tap("a", function (ctx, d) { return { delay: ctx.mine }; });
	`})
	if _, err := s.RunRewriteModel("m"); err != nil {
		t.Fatalf("RunRewriteModel: %v", err)
	}
	// Patch an unrelated field; must preserve ctx.mine.
	if err := s.PatchContext(ContextPatch{Provider: &ProviderSummary{ID: 1}}); err != nil {
		t.Fatalf("PatchContext: %v", err)
	}
	dec, err := s.RunBeforeRequest(BeforeRequestDecision{})
	if err != nil {
		t.Fatalf("RunBeforeRequest: %v", err)
	}
	if dec.Delay != 99 {
		t.Errorf("custom ctx field not preserved across patches, got delay=%d", dec.Delay)
	}
}

func TestSession_RewriteModel_Passthrough(t *testing.T) {
	s := newTestSession(t, db.Script{ID: "a", Source: `picotera.hooks.rewriteModel.tap("a", function () {});`})
	out, err := s.RunRewriteModel("claude-3-haiku")
	if err != nil {
		t.Fatalf("RunRewriteModel: %v", err)
	}
	if out != "claude-3-haiku" {
		t.Errorf("want passthrough, got %q", out)
	}
}

func TestSession_RewriteModel_NonStringKeepsInitial(t *testing.T) {
	s := newTestSession(t, db.Script{ID: "a", Source: `picotera.hooks.rewriteModel.tap("a", function () { return 42; });`})
	out, err := s.RunRewriteModel("orig")
	if err != nil {
		t.Fatalf("RunRewriteModel: %v", err)
	}
	if out != "orig" {
		t.Errorf("non-string result should keep initial, got %q", out)
	}
}

func TestSession_SortProviders_Reverse(t *testing.T) {
	s := newTestSession(t, db.Script{ID: "a", Source: `
		picotera.hooks.sortProviders.tap("a", function (ctx, list) { return list.slice().reverse(); });
	`})
	out, err := s.RunSortProviders([]CandidateView{
		{Provider: ProviderSummary{ID: 1}},
		{Provider: ProviderSummary{ID: 2}},
	})
	if err != nil {
		t.Fatalf("RunSortProviders: %v", err)
	}
	if len(out) != 2 || out[0].Provider.ID != 2 {
		t.Errorf("want reversed, got %+v", out)
	}
}

func TestSession_SortProviders_Passthrough(t *testing.T) {
	s := newTestSession(t, db.Script{ID: "a", Source: `picotera.hooks.sortProviders.tap("a", function () {});`})
	in := []CandidateView{{Provider: ProviderSummary{ID: 1}}}
	out, err := s.RunSortProviders(in)
	if err != nil {
		t.Fatalf("RunSortProviders: %v", err)
	}
	if len(out) != 1 || out[0].Provider.ID != 1 {
		t.Errorf("want passthrough preserves input, got %+v", out)
	}
}

func TestSession_SortProviders_ProvidersWrapper(t *testing.T) {
	s := newTestSession(t, db.Script{ID: "a", Source: `
		picotera.hooks.sortProviders.tap("a", function (ctx, list) { return { providers: list.slice(0, 1) }; });
	`})
	out, err := s.RunSortProviders([]CandidateView{
		{Provider: ProviderSummary{ID: 1}},
		{Provider: ProviderSummary{ID: 2}},
	})
	if err != nil {
		t.Fatalf("RunSortProviders: %v", err)
	}
	if len(out) != 1 || out[0].Provider.ID != 1 {
		t.Errorf("want {providers:[...]} shape accepted, got %+v", out)
	}
}

func TestSession_SortProviders_EmptyArrayMeansNoProviders(t *testing.T) {
	s := newTestSession(t, db.Script{ID: "a", Source: `picotera.hooks.sortProviders.tap("a", function () { return []; });`})
	out, err := s.RunSortProviders([]CandidateView{{Provider: ProviderSummary{ID: 1}}})
	if err != nil {
		t.Fatalf("RunSortProviders: %v", err)
	}
	if len(out) != 0 {
		t.Errorf("empty array should mean no providers, got %+v", out)
	}
}

func TestSession_BeforeRequest_Defaults(t *testing.T) {
	s := newTestSession(t, db.Script{ID: "a", Source: `picotera.hooks.beforeRequest.tap("a", function () {});`})
	dec, err := s.RunBeforeRequest(BeforeRequestDecision{})
	if err != nil {
		t.Fatalf("RunBeforeRequest: %v", err)
	}
	if dec.Next || dec.Delay != 0 || dec.UpstreamModel != "" {
		t.Errorf("want defaults, got %+v", dec)
	}
}

func TestSession_BeforeRequest_InitialNextPreserved(t *testing.T) {
	s := newTestSession(t, db.Script{ID: "a", Source: `picotera.hooks.beforeRequest.tap("a", function () {});`})
	dec, err := s.RunBeforeRequest(BeforeRequestDecision{Next: true})
	if err != nil {
		t.Fatalf("RunBeforeRequest: %v", err)
	}
	if !dec.Next {
		t.Errorf("passthrough should preserve initial next=true, got %+v", dec)
	}
}

func TestSession_BeforeRequest_NextAndDelay(t *testing.T) {
	s := newTestSession(t, db.Script{ID: "a", Source: `picotera.hooks.beforeRequest.tap("a", function () { return { next: true, delay: 100 }; });`})
	dec, err := s.RunBeforeRequest(BeforeRequestDecision{})
	if err != nil {
		t.Fatalf("RunBeforeRequest: %v", err)
	}
	if !dec.Next || dec.Delay != 100 {
		t.Errorf("want {next:true, delay:100}, got %+v", dec)
	}
}

func TestSession_BeforeRequest_UpstreamModelNonStringDropped(t *testing.T) {
	s := newTestSession(t, db.Script{ID: "a", Source: `picotera.hooks.beforeRequest.tap("a", function () { return { upstreamModel: 42 }; });`})
	dec, err := s.RunBeforeRequest(BeforeRequestDecision{})
	if err != nil {
		t.Fatalf("RunBeforeRequest: %v", err)
	}
	if dec.UpstreamModel != "" {
		t.Errorf("non-string upstreamModel should be dropped, got %q", dec.UpstreamModel)
	}
}

func TestSession_RewriteRequest_Passthrough(t *testing.T) {
	s := newTestSession(t, db.Script{ID: "a", Source: `picotera.hooks.rewriteRequest.tap("a", function () {});`})
	in := PendingRequestShape{
		URL:     "https://x/v1/chat",
		Method:  "POST",
		Headers: map[string][]string{"content-type": {"application/json"}},
		Body:    json.RawMessage(`{"a":1}`),
	}
	out, err := s.RunRewriteRequest(in)
	if err != nil {
		t.Fatalf("RunRewriteRequest: %v", err)
	}
	if out.URL != in.URL || out.Method != in.Method {
		t.Errorf("passthrough should preserve URL/Method, got %+v", out)
	}
	var inner string
	if err := json.Unmarshal(out.Body, &inner); err != nil {
		t.Fatalf("body should be a JSON string token, got %s: %v", string(out.Body), err)
	}
	if inner != `{"a":1}` {
		t.Errorf("body content mismatch: got %q", inner)
	}
}

func TestSession_RewriteRequest_BodyJSONRoundtrip(t *testing.T) {
	s := newTestSession(t, db.Script{ID: "a", Source: `
		picotera.hooks.rewriteRequest.tap("a", function (ctx, pending) { pending.body.b = 2; return pending; });
	`})
	out, err := s.RunRewriteRequest(PendingRequestShape{
		URL:     "https://x",
		Method:  "POST",
		Headers: map[string][]string{"content-type": {"application/json"}},
		Body:    json.RawMessage(`{"a":1}`),
	})
	if err != nil {
		t.Fatalf("RunRewriteRequest: %v", err)
	}
	var inner string
	if err := json.Unmarshal(out.Body, &inner); err != nil {
		t.Fatalf("body should be JSON string token: %v", err)
	}
	var got map[string]int
	if err := json.Unmarshal([]byte(inner), &got); err != nil {
		t.Fatalf("inner body should be JSON object: %v (raw=%q)", err, inner)
	}
	if got["a"] != 1 || got["b"] != 2 {
		t.Errorf("want {a:1,b:2}, got %+v", got)
	}
}

func TestSession_BeforeTransform_Passthrough(t *testing.T) {
	s := newTestSession(t, db.Script{ID: "a", Source: `picotera.hooks.beforeTransform.tap("a", function () {});`})
	out, err := s.RunBeforeTransform(OutboundProfile{Type: "openai", Config: map[string]any{}})
	if err != nil {
		t.Fatalf("RunBeforeTransform: %v", err)
	}
	if out.Type != "openai" || len(out.Config) != 0 {
		t.Errorf("want passthrough, got %+v", out)
	}
}

func TestSession_BeforeTransform_ReturnsNewProfile(t *testing.T) {
	s := newTestSession(t, db.Script{ID: "a", Source: `
		picotera.hooks.beforeTransform.tap("a", function () { return { type: "openrouter", config: { provider: "x" } }; });
	`})
	out, err := s.RunBeforeTransform(OutboundProfile{Type: "openai", Config: map[string]any{"keep": true}})
	if err != nil {
		t.Fatalf("RunBeforeTransform: %v", err)
	}
	if out.Type != "openrouter" || out.Config["provider"] != "x" {
		t.Errorf("want openrouter profile, got %+v", out)
	}
	if _, ok := out.Config["keep"]; ok {
		t.Errorf("new profile should replace config, got %+v", out.Config)
	}
}

func TestSession_BeforeTransform_TypeMustBeString(t *testing.T) {
	s := newTestSession(t, db.Script{ID: "a", Source: `picotera.hooks.beforeTransform.tap("a", function () { return { type: 42 }; });`})
	_, err := s.RunBeforeTransform(OutboundProfile{Type: "openai", Config: map[string]any{}})
	if err == nil || !strings.Contains(err.Error(), "jsx: beforeTransform type must be string") {
		t.Fatalf("err = %v, want beforeTransform type error", err)
	}
}

func TestSession_BeforeTransform_ConfigMustBeObject(t *testing.T) {
	for _, tt := range []struct{ name, source string }{
		{"number", `picotera.hooks.beforeTransform.tap("a", function () { return { config: 42 }; });`},
		{"array", `picotera.hooks.beforeTransform.tap("a", function () { return { config: [] }; });`},
	} {
		t.Run(tt.name, func(t *testing.T) {
			s := newTestSession(t, db.Script{ID: "a", Source: tt.source})
			_, err := s.RunBeforeTransform(OutboundProfile{Type: "openai", Config: map[string]any{}})
			if err == nil || !strings.Contains(err.Error(), "jsx: beforeTransform config must be object") {
				t.Fatalf("err = %v, want beforeTransform config error", err)
			}
		})
	}
}

func TestSession_RewriteProviderModels_Passthrough(t *testing.T) {
	s := newTestSession(t, db.Script{ID: "a", Source: `picotera.hooks.rewriteProviderModels.tap("a", function () {});`})
	in := []ProviderModelEntry{{Model: "gpt-4o"}, {Model: "my-mini", UpstreamModelName: "gpt-4o-mini"}}
	out, err := s.RunRewriteProviderModels(in)
	if err != nil {
		t.Fatalf("RunRewriteProviderModels: %v", err)
	}
	if len(out) != 2 || out[0].Model != "gpt-4o" || out[1].Model != "my-mini" {
		t.Errorf("want passthrough, got %+v", out)
	}
}

func TestSession_RewriteProviderModels_Replace(t *testing.T) {
	s := newTestSession(t, db.Script{ID: "a", Source: `
		picotera.hooks.rewriteProviderModels.tap("a", function () {
			return [{model: 'a'}, {model: 'b', upstreamModelName: 'B', priority: 7, annotations: {x: 'y'}}];
		});
	`})
	out, err := s.RunRewriteProviderModels(nil)
	if err != nil {
		t.Fatalf("RunRewriteProviderModels: %v", err)
	}
	if len(out) != 2 || out[1].Model != "b" || out[1].UpstreamModelName != "B" || out[1].Priority != 7 || out[1].Annotations["x"] != "y" {
		t.Errorf("unexpected result: %+v", out)
	}
}

func TestSession_RewriteProviderModels_NonArrayKeepsInput(t *testing.T) {
	s := newTestSession(t, db.Script{ID: "a", Source: `picotera.hooks.rewriteProviderModels.tap("a", function () { return 42; });`})
	in := []ProviderModelEntry{{Model: "keep-me"}}
	out, err := s.RunRewriteProviderModels(in)
	if err != nil {
		t.Fatalf("RunRewriteProviderModels: %v", err)
	}
	if len(out) != 1 || out[0].Model != "keep-me" {
		t.Errorf("want fallback to input, got %+v", out)
	}
}

func TestSession_RewriteProviderModels_ReadsUpstreamResponse(t *testing.T) {
	s := newTestSession(t, db.Script{ID: "a", Source: `
		picotera.hooks.rewriteProviderModels.tap("a", function (ctx) {
			return [{model: ctx.upstreamResponse.data[0].id}];
		});
	`})
	if err := s.PatchContext(ContextPatch{
		UpstreamResponse: json.RawMessage(`{"data":[{"id":"up-model"}]}`),
	}); err != nil {
		t.Fatalf("PatchContext: %v", err)
	}
	out, err := s.RunRewriteProviderModels(nil)
	if err != nil {
		t.Fatalf("RunRewriteProviderModels: %v", err)
	}
	if len(out) != 1 || out[0].Model != "up-model" {
		t.Errorf("want model from upstreamResponse, got %+v", out)
	}
}

func TestSession_Timeout_TaintsSession(t *testing.T) {
	s := newTestSession(t, db.Script{ID: "a", Source: `picotera.hooks.sortProviders.tap("a", function () { for(;;){} });`})
	start := time.Now()
	_, err := s.RunSortProviders([]CandidateView{{Provider: ProviderSummary{ID: 1}}})
	if err != ErrHookTimeout {
		t.Fatalf("want ErrHookTimeout, got %v", err)
	}
	if elapsed := time.Since(start); elapsed > 2*time.Second {
		t.Errorf("timeout took too long: %v", elapsed)
	}
	// Tainted: subsequent hooks fast-fail.
	if _, err := s.RunBeforeRequest(BeforeRequestDecision{}); err != ErrHookTimeout {
		t.Errorf("tainted session should fast-fail, got %v", err)
	}
	if err := s.PatchContext(ContextPatch{Provider: &ProviderSummary{ID: 1}}); err != ErrHookTimeout {
		t.Errorf("tainted session PatchContext should fast-fail, got %v", err)
	}
}

func TestSession_MemoryLimit(t *testing.T) {
	eng := NewEngine(
		Config{HookTimeout: 2 * time.Second, MemoryLimit: 2 * 1024 * 1024},
		&fakeStore{scripts: []db.Script{{ID: "a", Source: `
			picotera.hooks.sortProviders.tap("a", function () { var x = new Array(5000000).fill(0); return [{provider:{id:x.length}}]; });
		`}}},
		kv.NewMemoryStore(),
	)
	s, err := eng.NewSession(context.Background(), "")
	if err != nil {
		t.Fatalf("NewSession: %v", err)
	}
	defer s.Close()
	_, err = s.RunSortProviders([]CandidateView{{Provider: ProviderSummary{ID: 1}}})
	if err == nil {
		t.Fatalf("want memory-limit error, got nil")
	}
	if err == ErrHookTimeout {
		t.Errorf("memory exhaustion should not surface as a timeout")
	}
}

func TestSession_Fetch_Sync(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"hello":"world"}`))
	}))
	defer srv.Close()

	s := newTestSession(t, db.Script{ID: "a", Source: `
		picotera.hooks.rewriteModel.tap("a", function () {
			var resp = picotera.fetch("` + srv.URL + `");
			var parsed = JSON.parse(resp.body);
			return String(resp.status) + ":" + parsed.hello;
		});
	`})
	out, err := s.RunRewriteModel("m")
	if err != nil {
		t.Fatalf("RunRewriteModel: %v", err)
	}
	if out != "200:world" {
		t.Errorf("fetch result mismatch, got %q", out)
	}
}

func TestSession_KV_RoundTrip(t *testing.T) {
	s := newTestSession(t, db.Script{ID: "a", Source: `
		picotera.hooks.rewriteModel.tap("a", function () {
			picotera.kv.set("k", { n: 5 });
			var v = picotera.kv.get("k");
			var missing = picotera.kv.get("nope");
			return v.n + ":" + (missing === null);
		});
	`})
	out, err := s.RunRewriteModel("m")
	if err != nil {
		t.Fatalf("RunRewriteModel: %v", err)
	}
	if out != "5:true" {
		t.Errorf("kv round-trip mismatch, got %q", out)
	}
}

func TestSession_Console_Captured(t *testing.T) {
	s := newTestSession(t, db.Script{ID: "a", Source: `
		picotera.hooks.rewriteModel.tap("a", function () {
			console.log("hello", { nested: true });
			console.warn("careful");
			console.error("boom");
			return "m";
		});
	`})
	if _, err := s.RunRewriteModel("m"); err != nil {
		t.Fatalf("RunRewriteModel: %v", err)
	}
	logs := s.Logs()
	if len(logs) != 3 {
		t.Fatalf("want 3 log entries, got %d (%+v)", len(logs), logs)
	}
	if logs[0].Level != "info" || !strings.Contains(logs[0].Message, "hello") {
		t.Errorf("unexpected first log: %+v", logs[0])
	}
	if logs[1].Level != "warn" || logs[2].Level != "error" {
		t.Errorf("unexpected log levels: %+v", logs)
	}
}
