package llmbridge

import (
	"fmt"
	"strings"
)

// Format identifies one of the supported generation formats. The two Gemini
// values distinguish stream vs. non-stream because Gemini routes that choice
// through the URL path rather than the request body.
type Format int

const (
	FormatUnknown Format = iota
	FormatAnthropicMessages
	FormatOpenAIChatCompletions
	FormatOpenAIResponses
	FormatGeminiGenerateContent       // non-stream
	FormatGeminiStreamGenerateContent // stream
)

// OutboundProfile selects the outbound transformer and JSON-object config for
// unified bridge attempts.
type OutboundProfile struct {
	Type   string         `json:"type"`
	Config map[string]any `json:"config"`
}

func (f Format) String() string {
	switch f {
	case FormatAnthropicMessages:
		return "anthropicMessages"
	case FormatOpenAIChatCompletions:
		return "openaiChatCompletions"
	case FormatOpenAIResponses:
		return "openaiResponses"
	case FormatGeminiGenerateContent:
		return "geminiGenerateContent"
	case FormatGeminiStreamGenerateContent:
		return "geminiStreamGenerateContent"
	default:
		return "unknown"
	}
}

func (f Format) IsStreaming() bool {
	return f == FormatGeminiStreamGenerateContent
}

func (f Format) IsGemini() bool {
	return f == FormatGeminiGenerateContent || f == FormatGeminiStreamGenerateContent
}

func DefaultOutboundProfileForFormat(f Format) (OutboundProfile, error) {
	switch f {
	case FormatAnthropicMessages:
		return OutboundProfile{Type: "anthropic", Config: map[string]any{}}, nil
	case FormatOpenAIChatCompletions:
		return OutboundProfile{Type: "openai", Config: map[string]any{}}, nil
	case FormatOpenAIResponses:
		return OutboundProfile{Type: "openaiResponses", Config: map[string]any{}}, nil
	case FormatGeminiGenerateContent, FormatGeminiStreamGenerateContent:
		return OutboundProfile{Type: "gemini", Config: map[string]any{}}, nil
	default:
		return OutboundProfile{}, fmt.Errorf("llmbridge: unsupported upstream format %q", f)
	}
}

// SyntheticGeminiPath returns the Path form Gemini Inbound expects.
func SyntheticGeminiPath(format Format, model string) string {
	if model == "" {
		model = "unknown"
	}
	switch format {
	case FormatGeminiStreamGenerateContent:
		return "/v1beta/models/" + model + ":streamGenerateContent"
	case FormatGeminiGenerateContent:
		fallthrough
	default:
		return "/v1beta/models/" + model + ":generateContent"
	}
}

type AggregationKind int

const (
	StreamAggregationNone AggregationKind = iota
	StreamAggregationSSE
	StreamAggregationJSONL
	StreamAggregationUnsupported
)

func StreamAggregationKind(format Format, contentType string) AggregationKind {
	ct := normalizedContentType(contentType)
	switch format {
	case FormatAnthropicMessages, FormatOpenAIChatCompletions, FormatOpenAIResponses:
		switch ct {
		case "text/event-stream":
			return StreamAggregationSSE
		case "", "application/json":
			return StreamAggregationNone
		default:
			return StreamAggregationUnsupported
		}
	case FormatGeminiStreamGenerateContent:
		switch ct {
		case "text/event-stream":
			return StreamAggregationSSE
		case "application/jsonl", "application/x-ndjson", "application/jsonlines", "application/ndjson", "application/json":
			return StreamAggregationJSONL
		default:
			return StreamAggregationUnsupported
		}
	default:
		return StreamAggregationNone
	}
}

func normalizedContentType(ct string) string {
	if i := strings.IndexByte(ct, ';'); i >= 0 {
		ct = ct[:i]
	}
	return strings.ToLower(strings.TrimSpace(ct))
}
