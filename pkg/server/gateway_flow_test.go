package server

import (
	"context"
	"errors"
	"net/http/httptest"
	"testing"
	"time"

	"picotera/pkg/configx"
	"picotera/pkg/contract"
	"picotera/pkg/db"
	"picotera/pkg/jsx"
	"picotera/pkg/llmbridge"

	"github.com/jackc/pgx/v5/pgtype"
)

func TestGatewayCandidateSidecarLookupPath(t *testing.T) {
	set := candidateSet{Items: []gatewayCandidate{{
		Candidate: jsx.Candidate{Provider: jsx.ProviderSummary{ID: 7}},
		Sidecar:   gatewayCandidateSidecar{Key: "7", ProviderID: 7, UpstreamURL: "https://upstream.test"},
	}}}
	side, ok := lookupCandidateSidecar(gatewayRoutePath, candidateSidecarMap(set), set.Items[0].Candidate)
	if !ok {
		t.Fatal("expected path sidecar lookup to succeed")
	}
	if side.ProviderID != 7 || side.UpstreamURL != "https://upstream.test" {
		t.Fatalf("unexpected sidecar: %+v", side)
	}
}

func TestGatewayCandidateSidecarLookupUnified(t *testing.T) {
	cand := jsx.Candidate{Provider: jsx.ProviderSummary{ID: 9}, MPE: jsx.CandidateMPE{EndpointPath: "/v1/messages"}}
	set := candidateSet{Items: []gatewayCandidate{{Candidate: cand, Sidecar: gatewayCandidateSidecar{Key: "9|/v1/messages", ProviderID: 9}}}}
	if _, ok := lookupCandidateSidecar(gatewayRouteUnified, candidateSidecarMap(set), cand); !ok {
		t.Fatal("expected unified sidecar lookup to include endpoint path")
	}
	wrongPath := cand
	wrongPath.MPE.EndpointPath = "/v1/chat/completions"
	if _, ok := lookupCandidateSidecar(gatewayRouteUnified, candidateSidecarMap(set), wrongPath); ok {
		t.Fatal("expected wrong provider/path pair to miss")
	}
}

func TestGatewayUnknownCandidateSkipped(t *testing.T) {
	f := &gatewayFlow{h: &gatewayHandler{Server: &Server{config: &configx.Config{JSMaxTotalAttempts: 1}}}, config: gatewayFlowConfig{Kind: gatewayRoutePath}}
	result := f.runAttempts([]jsx.Candidate{{Provider: jsx.ProviderSummary{ID: 404}}}, map[string]gatewayCandidateSidecar{}, gatewayJSContext{})
	if result.Handled {
		t.Fatal("unknown candidate should not be handled")
	}
	if result.LastErr != nil {
		t.Fatalf("unknown candidate should skip without last error, got %v", result.LastErr)
	}
}

func TestGatewayDelayRespectsContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	f := &gatewayFlow{
		h:    &gatewayHandler{Server: &Server{config: &configx.Config{JSMaxDelay: time.Second}}},
		ctxs: gatewayContexts{Request: ctx},
	}
	if f.waitHookDelay(1000) {
		t.Fatal("expected canceled request context to interrupt delay")
	}
}

func TestGatewayHookErrorStatusMapping(t *testing.T) {
	if got := gatewayHookStatus(errors.New("boom")); got != 502 {
		t.Fatalf("plain hook error status = %d, want 502", got)
	}
	if got := gatewayHookStatus(jsx.ErrHookTimeout); got != 503 {
		t.Fatalf("timeout hook error status = %d, want 503", got)
	}
}

func TestBuildPathCandidateSetAnnotations(t *testing.T) {
	rows := []providerCandidateRow{{
		ProviderID:              1,
		ProviderName:            "provider",
		ProviderCredentials:     "secret",
		ProviderPriority:        10,
		UpstreamURL:             "https://upstream.test",
		SendCredentialsResolver: contract.CredentialsResolver_BearerToken,
		ProxyURL:                pgtype.Text{String: "direct", Valid: true},
		ProviderAnnotations:     []byte(`{"k":"provider","providerOnly":"1"}`),
		ModelAnnotations:        []byte(`{"k":"model","modelOnly":"1"}`),
		EntryAnnotations:        []byte(`{"k":"entry","entryOnly":"1"}`),
		EndpointPath:            "/v1/messages",
	}}
	set, err := buildPathCandidateSet(rows, map[string]string{"k": "api", "apiOnly": "1"}, nil, db.Endpoint{Path: "/v1/messages"})
	if err != nil {
		t.Fatal(err)
	}
	anno := set.Items[0].Candidate.Annotations
	if anno["k"] != "api" || anno["modelOnly"] != "1" || anno["providerOnly"] != "1" || anno["entryOnly"] != "1" || anno["apiOnly"] != "1" {
		t.Fatalf("unexpected merged annotations: %+v", anno)
	}
}

func TestBuildUnifiedCandidateSetAnnotationsAndFormat(t *testing.T) {
	rows := []db.GetProvidersByEndpointTypesAndModelRow{{
		ProviderID:              2,
		ProviderName:            "provider",
		ProviderCredentials:     "secret",
		ProviderPriority:        10,
		UpstreamUrl:             "https://upstream.test",
		SendCredentialsResolver: contract.CredentialsResolver_BearerToken,
		EndpointPath:            "/v1/chat/completions",
		EndpointType:            contract.EndpointType_OpenAIChatCompletions,
		ProviderAnnotations:     []byte(`{"k":"provider"}`),
		ModelAnnotations:        []byte(`{"k":"model"}`),
		Annotations:             []byte(`{"k":"entry"}`),
		SupportsNativeWebSearch: true,
	}}
	set, err := buildUnifiedCandidateSet(rows, map[string]string{"k": "api"}, nil, db.Endpoint{})
	if err != nil {
		t.Fatal(err)
	}
	side := set.Items[0].Sidecar
	if side.UpstreamFormat != llmbridge.FormatOpenAIChatCompletions || !side.SupportsNativeWebSearch {
		t.Fatalf("unexpected unified sidecar: %+v", side)
	}
	if set.Items[0].Candidate.Annotations["k"] != "api" {
		t.Fatalf("api annotation should win, got %+v", set.Items[0].Candidate.Annotations)
	}
}

func TestRecordAttemptFailureLastError(t *testing.T) {
	state := &attemptState{}
	err := errors.New("upstream returned 429: rate limited")
	updateAttemptState(state, 42, 429, "rate limited", err)
	if state.LastErr != err {
		t.Fatalf("LastErr = %v", state.LastErr)
	}
	if state.LastJSErr == nil || state.LastJSErr.ProviderID != 42 || state.LastJSErr.StatusCode != 429 || state.LastJSErr.Message != "rate limited" {
		t.Fatalf("unexpected LastJSErr: %+v", state.LastJSErr)
	}
	if state.CurrentRetryCount != 1 || state.TotalAttemptCount != 1 {
		t.Fatalf("unexpected counters: %+v", state)
	}
}

func TestPersistContextSurvivesRequestCancelUntilTimeout(t *testing.T) {
	reqCtx, cancel := context.WithCancel(context.Background())
	r := httptest.NewRequest("POST", "/v1/messages", nil).WithContext(reqCtx)
	ctxs := newGatewayContexts(r, &configx.Config{GatewayReadTimeout: time.Hour})
	defer ctxs.CancelPersist()
	cancel()
	select {
	case <-ctxs.Persist.Done():
		t.Fatal("persist context should not be canceled by request cancellation")
	default:
	}
}

func TestPersistContextKeepsRequestValues(t *testing.T) {
	type key string
	reqCtx := context.WithValue(context.Background(), key("trace"), "value")
	r := httptest.NewRequest("POST", "/v1/messages", nil).WithContext(reqCtx)
	ctxs := newGatewayContexts(r, &configx.Config{})
	defer ctxs.CancelPersist()
	if got := ctxs.Persist.Value(key("trace")); got != "value" {
		t.Fatalf("persist context value = %v, want value", got)
	}
}
