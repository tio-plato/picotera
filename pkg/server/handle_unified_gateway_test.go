package server

import (
	"context"
	"net/http/httptest"
	"reflect"
	"testing"

	"picotera/pkg/contract"
	"picotera/pkg/db"
	"picotera/pkg/llmbridge"

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
		contract.EndpointType_GeneralListModels:           llmbridge.FormatUnknown,
	}
	for t1, want := range cases {
		if got := upstreamFormatFor(t1); got != want {
			t.Errorf("upstreamFormatFor(%d) = %s, want %s", t1, got, want)
		}
	}
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
