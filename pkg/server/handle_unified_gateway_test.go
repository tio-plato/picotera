package server

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"picotera/pkg/contract"
	"picotera/pkg/db"
	"picotera/pkg/llmbridge"
	"picotera/pkg/llmbridgeimpl"

	"github.com/go-chi/chi/v5"
)

// Smoke-coverage of the small helpers that translate between bridge
// formats, endpoint type ids, and the per-route stream behavior. The
// handler itself is not covered by tests yet — picotera has no postgres
// test harness and Server can't be built without one. See plan §8.

func TestSourceEndpointType(t *testing.T) {
	cases := map[llmbridge.Format]int32{
		llmbridge.FormatAnthropicMessages:           contract.EndpointType_AnthropicMessages,
		llmbridge.FormatOpenAIChatCompletions:       contract.EndpointType_OpenAIChatCompletions,
		llmbridge.FormatOpenAIResponses:             contract.EndpointType_OpenAIResponses,
		llmbridge.FormatGeminiGenerateContent:       contract.EndpointType_GeminiGenerateContent,
		llmbridge.FormatGeminiStreamGenerateContent: contract.EndpointType_GeminiStreamGenerateContent,
		llmbridge.FormatUnknown:                     contract.EndpointType_Unknown,
	}
	for f, want := range cases {
		if got := sourceEndpointType(f); got != want {
			t.Errorf("sourceEndpointType(%s) = %d, want %d", f, got, want)
		}
	}
}

func TestUpstreamFormatFor(t *testing.T) {
	cases := map[int32]llmbridge.Format{
		contract.EndpointType_AnthropicMessages:           llmbridge.FormatAnthropicMessages,
		contract.EndpointType_OpenAIChatCompletions:       llmbridge.FormatOpenAIChatCompletions,
		contract.EndpointType_OpenAIResponses:             llmbridge.FormatOpenAIResponses,
		contract.EndpointType_GeminiGenerateContent:       llmbridge.FormatGeminiGenerateContent,
		contract.EndpointType_GeminiStreamGenerateContent: llmbridge.FormatGeminiStreamGenerateContent,
		contract.EndpointType_AnthropicCountTokens:        llmbridge.FormatUnknown,
		contract.EndpointType_Unknown:                     llmbridge.FormatUnknown,
	}
	for t1, want := range cases {
		if got := upstreamFormatFor(t1); got != want {
			t.Errorf("upstreamFormatFor(%d) = %s, want %s", t1, got, want)
		}
	}
}

func TestResponseAggregationFormat(t *testing.T) {
	cases := []struct {
		endpointType int32
		wantFormat   llmbridge.Format
		wantOK       bool
	}{
		{contract.EndpointType_AnthropicMessages, llmbridge.FormatAnthropicMessages, true},
		{contract.EndpointType_OpenAIChatCompletions, llmbridge.FormatOpenAIChatCompletions, true},
		{contract.EndpointType_OpenAIResponses, llmbridge.FormatOpenAIResponses, true},
		{contract.EndpointType_GeminiStreamGenerateContent, llmbridge.FormatGeminiStreamGenerateContent, true},
		{contract.EndpointType_GeminiGenerateContent, llmbridge.FormatUnknown, false},
		{contract.EndpointType_General, llmbridge.FormatUnknown, false},
		{contract.EndpointType_Unknown, llmbridge.FormatUnknown, false},
	}
	for _, tt := range cases {
		gotFormat, gotOK := responseAggregationFormat(tt.endpointType)
		if gotFormat != tt.wantFormat || gotOK != tt.wantOK {
			t.Errorf("responseAggregationFormat(%d) = (%s, %v), want (%s, %v)", tt.endpointType, gotFormat, gotOK, tt.wantFormat, tt.wantOK)
		}
	}
}

func TestBuildAggregatedArtifactGeminiStreamAndNonStream(t *testing.T) {
	streamLine := `{"responseId":"resp-1","modelVersion":"gemini-test","candidates":[{"index":0,"content":{"role":"model","parts":[{"text":"ok"}]},"finishReason":"STOP"}]}`
	profile, err := llmbridge.DefaultOutboundProfileForFormat(llmbridge.FormatGeminiStreamGenerateContent)
	if err != nil {
		t.Fatal(err)
	}
	aggregated := buildAggregatedArtifact(context.Background(), fakeLLMBridge{}, llmbridge.FormatGeminiStreamGenerateContent, "application/json", []byte(streamLine+"\n"), profile)
	if aggregated == nil {
		t.Fatal("expected aggregated artifact")
	}
	if aggregated.Error != "" {
		t.Fatalf("unexpected aggregation error: %s", aggregated.Error)
	}
	if aggregated.Format != "geminiStreamGenerateContent" || !strings.Contains(string(aggregated.Body), `"responseId":"resp-1"`) {
		t.Fatalf("unexpected aggregated body: format=%s body=%s", aggregated.Format, aggregated.Body)
	}

	nonStreamProfile, err := llmbridge.DefaultOutboundProfileForFormat(llmbridge.FormatGeminiGenerateContent)
	if err != nil {
		t.Fatal(err)
	}
	aggregated = buildAggregatedArtifact(context.Background(), fakeLLMBridge{}, llmbridge.FormatGeminiGenerateContent, "application/json", []byte(`{"candidates":[]}`), nonStreamProfile)
	if aggregated != nil {
		t.Fatalf("Gemini non-stream should not aggregate, got %+v", aggregated)
	}
}

type fakeLLMBridge struct{}

func (fakeLLMBridge) Enabled() bool {
	return true
}

func (fakeLLMBridge) Close(ctx context.Context) error {
	return nil
}

func (fakeLLMBridge) BridgeRequest(ctx context.Context, src, dst llmbridge.Format, body []byte, headers http.Header, pendingURL string, profile llmbridge.OutboundProfile) ([]byte, string, error) {
	return body, "application/json", nil
}

func (fakeLLMBridge) BridgeNonStream(ctx context.Context, src, upstream llmbridge.Format, upstreamBody []byte, upstreamHeaders http.Header, profile llmbridge.OutboundProfile) ([]byte, string, error) {
	return upstreamBody, "application/json", nil
}

func (fakeLLMBridge) BridgeStream(ctx context.Context, src, upstream llmbridge.Format, upstreamBody io.ReadCloser, upstreamCT string, profile llmbridge.OutboundProfile) (io.ReadCloser, error) {
	return upstreamBody, nil
}

func (fakeLLMBridge) AggregateStream(ctx context.Context, format llmbridge.Format, contentType string, body []byte, profile llmbridge.OutboundProfile) ([]byte, error) {
	return llmbridgeimpl.AggregateStream(ctx, format, contentType, body, profile)
}

func TestCandidateEndpointTypes(t *testing.T) {
	// Anthropic / OpenAI sources: stream flag picks the Gemini variant.
	got := candidateEndpointTypes(llmbridge.FormatAnthropicMessages, false)
	want := []int32{
		contract.EndpointType_AnthropicMessages,
		contract.EndpointType_OpenAIChatCompletions,
		contract.EndpointType_OpenAIResponses,
		contract.EndpointType_GeminiGenerateContent,
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Anthropic non-stream set = %v, want %v", got, want)
	}
	got = candidateEndpointTypes(llmbridge.FormatOpenAIChatCompletions, true)
	want = []int32{
		contract.EndpointType_AnthropicMessages,
		contract.EndpointType_OpenAIChatCompletions,
		contract.EndpointType_OpenAIResponses,
		contract.EndpointType_GeminiStreamGenerateContent,
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("OpenAI stream set = %v, want %v", got, want)
	}

	// Gemini routes ignore the stream-flag arg and always use their own
	// fixed pair.
	got = candidateEndpointTypes(llmbridge.FormatGeminiStreamGenerateContent, false)
	if got[len(got)-1] != contract.EndpointType_GeminiStreamGenerateContent {
		t.Errorf("Gemini stream route returned wrong gemini variant: %v", got)
	}
}

func TestExtractUnifiedModelAndStream_BodyFormats(t *testing.T) {
	body := []byte(`{"model":"claude-3-5-sonnet","stream":true}`)
	r := httptest.NewRequest("POST", "/api/picotera/v1/messages", nil)
	model, stream, err := extractUnifiedModelAndStream(llmbridge.FormatAnthropicMessages, r, body)
	if err != nil {
		t.Fatal(err)
	}
	if model != "claude-3-5-sonnet" || !stream {
		t.Errorf("got model=%q stream=%v", model, stream)
	}

	// Missing model field: 400 MODEL_NOT_FOUND.
	_, _, err = extractUnifiedModelAndStream(llmbridge.FormatOpenAIChatCompletions, r, []byte(`{}`))
	if err == nil {
		t.Errorf("expected error for missing model, got nil")
	}
}

func TestExtractUnifiedModelAndStream_GeminiFromPath(t *testing.T) {
	// Build a chi route context that simulates the chi router placing
	// {model} into the URL params.
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("model", "gemini-2.5-pro")
	r := httptest.NewRequest("POST", "/api/picotera/v1beta/models/gemini-2.5-pro:streamGenerateContent", nil)
	r = r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))

	model, stream, err := extractUnifiedModelAndStream(llmbridge.FormatGeminiStreamGenerateContent, r, []byte(`{"contents":[]}`))
	if err != nil {
		t.Fatal(err)
	}
	if model != "gemini-2.5-pro" || !stream {
		t.Errorf("got model=%q stream=%v", model, stream)
	}

	model, stream, err = extractUnifiedModelAndStream(llmbridge.FormatGeminiGenerateContent, r, []byte(`{"contents":[]}`))
	if err != nil {
		t.Fatal(err)
	}
	if model != "gemini-2.5-pro" || stream {
		t.Errorf("non-stream variant: got model=%q stream=%v", model, stream)
	}
}

func TestSetUnifiedModel(t *testing.T) {
	// Body-bearing source: model is rewritten via sjson.
	body := []byte(`{"model":"old","messages":[]}`)
	out, err := setUnifiedModel(llmbridge.FormatAnthropicMessages, body, "new")
	if err != nil {
		t.Fatal(err)
	}
	if string(out) == string(body) {
		t.Errorf("expected model rewrite, body unchanged: %s", out)
	}
	// Gemini: body unchanged because the model lives in the URL.
	body = []byte(`{"contents":[]}`)
	out, err = setUnifiedModel(llmbridge.FormatGeminiGenerateContent, body, "new")
	if err != nil {
		t.Fatal(err)
	}
	if string(out) != string(body) {
		t.Errorf("expected Gemini body unchanged, got %s", out)
	}
}

func TestDedupeUnifiedRows(t *testing.T) {
	row := func(providerID int32, et int32, path string) db.GetProvidersByEndpointTypesAndModelRow {
		return db.GetProvidersByEndpointTypesAndModelRow{
			ModelName:    "m",
			ProviderID:   providerID,
			EndpointType: et,
			EndpointPath: path,
		}
	}
	type want struct {
		providerID int32
		path       string
	}
	cases := []struct {
		name    string
		rows    []db.GetProvidersByEndpointTypesAndModelRow
		srcType int32
		want    []want
	}{
		{
			name:    "single",
			rows:    []db.GetProvidersByEndpointTypesAndModelRow{row(1, contract.EndpointType_OpenAIChatCompletions, "/v1/chat")},
			srcType: contract.EndpointType_OpenAIChatCompletions,
			want:    []want{{1, "/v1/chat"}},
		},
		{
			name: "src match",
			rows: []db.GetProvidersByEndpointTypesAndModelRow{
				row(1, contract.EndpointType_AnthropicMessages, "/a"),
				row(1, contract.EndpointType_OpenAIChatCompletions, "/c"),
			},
			srcType: contract.EndpointType_OpenAIChatCompletions,
			want:    []want{{1, "/c"}},
		},
		{
			name: "anthropic preferred",
			rows: []db.GetProvidersByEndpointTypesAndModelRow{
				row(1, contract.EndpointType_OpenAIResponses, "/r"),
				row(1, contract.EndpointType_AnthropicMessages, "/a"),
			},
			srcType: contract.EndpointType_GeminiGenerateContent,
			want:    []want{{1, "/a"}},
		},
		{
			name: "chat preferred",
			rows: []db.GetProvidersByEndpointTypesAndModelRow{
				row(1, contract.EndpointType_OpenAIResponses, "/r"),
				row(1, contract.EndpointType_OpenAIChatCompletions, "/c"),
			},
			srcType: contract.EndpointType_GeminiGenerateContent,
			want:    []want{{1, "/c"}},
		},
		{
			name: "path tiebreak",
			rows: []db.GetProvidersByEndpointTypesAndModelRow{
				row(1, contract.EndpointType_OpenAIResponses, "/z"),
				row(1, contract.EndpointType_OpenAIResponses, "/a"),
			},
			srcType: contract.EndpointType_GeminiGenerateContent,
			want:    []want{{1, "/a"}},
		},
		{
			name: "multi provider",
			rows: []db.GetProvidersByEndpointTypesAndModelRow{
				row(1, contract.EndpointType_OpenAIChatCompletions, "/c"),
				row(2, contract.EndpointType_AnthropicMessages, "/a"),
			},
			srcType: contract.EndpointType_OpenAIChatCompletions,
			want: []want{
				{1, "/c"},
				{2, "/a"},
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := dedupeUnifiedRows(tc.rows, tc.srcType)
			if len(got) != len(tc.want) {
				t.Fatalf("len = %d, want %d (got=%+v)", len(got), len(tc.want), got)
			}
			gotW := make([]want, len(got))
			for i, r := range got {
				gotW[i] = want{r.ProviderID, r.EndpointPath}
			}
			if !reflect.DeepEqual(gotW, tc.want) {
				t.Errorf("got %+v, want %+v", gotW, tc.want)
			}
		})
	}
}
