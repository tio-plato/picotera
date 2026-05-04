// Package llmbridge converts LLM request and response payloads between four
// generation formats (Anthropic Messages, OpenAI Chat Completions, OpenAI
// Responses, Gemini GenerateContent), so that the unified gateway routes
// in pkg/server/handle_unified_gateway.go can dispatch any source-format
// request to any candidate upstream regardless of its protocol.
//
// All conversion is delegated to github.com/looplj/axonhub/llm. This package
// is the only place picotera imports axonhub types, so the rest of the
// codebase sees a small, picotera-shaped surface.
package llmbridge

import (
	"fmt"

	"github.com/looplj/axonhub/llm/auth"
	"github.com/looplj/axonhub/llm/transformer"
	anthropictrans "github.com/looplj/axonhub/llm/transformer/anthropic"
	geminitrans "github.com/looplj/axonhub/llm/transformer/gemini"
	openaitrans "github.com/looplj/axonhub/llm/transformer/openai"
	openairesponses "github.com/looplj/axonhub/llm/transformer/openai/responses"
)

// Format identifies one of the four supported generation formats. The two
// Gemini values distinguish stream vs. non-stream because Gemini routes the
// stream flag through the URL path rather than the request body, and our
// inbound handler needs to know which variant the client called.
type Format int

const (
	FormatUnknown Format = iota
	FormatAnthropicMessages
	FormatOpenAIChatCompletions
	FormatOpenAIResponses
	FormatGeminiGenerateContent       // non-stream
	FormatGeminiStreamGenerateContent // stream
)

// String renders the format for log lines and error messages.
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

// IsStreaming reports whether the format inherently signals a streaming
// response. Anthropic and OpenAI formats carry a "stream" body field, so
// IsStreaming returns false; the caller reads the flag from the parsed
// llm.Request. Gemini distinguishes stream vs. non-stream at the format
// level itself.
func (f Format) IsStreaming() bool {
	return f == FormatGeminiStreamGenerateContent
}

// IsGemini reports whether the format is one of the two Gemini variants.
// Used at the boundary where Gemini Inbound needs a synthetic httpReq.Path
// containing the model and stream marker.
func (f Format) IsGemini() bool {
	return f == FormatGeminiGenerateContent || f == FormatGeminiStreamGenerateContent
}

// inboundFor returns the axonhub Inbound transformer for parsing a body
// written in this format. We treat the four canonical Inbound choices as
// stateless singletons; the constructors only allocate an empty struct.
func inboundFor(f Format) (transformer.Inbound, error) {
	switch f {
	case FormatAnthropicMessages:
		return anthropictrans.NewInboundTransformer(), nil
	case FormatOpenAIChatCompletions:
		return openaitrans.NewInboundTransformer(), nil
	case FormatOpenAIResponses:
		return openairesponses.NewInboundTransformer(), nil
	case FormatGeminiGenerateContent, FormatGeminiStreamGenerateContent:
		return geminitrans.NewInboundTransformer(), nil
	default:
		return nil, fmt.Errorf("llmbridge: unsupported source format %q", f)
	}
}

// outboundFor returns the axonhub Outbound transformer for writing a body
// in this format. The transformers expect a baseURL and APIKeyProvider for
// URL construction and auth — picotera handles both itself, so we hand them
// throwaway placeholders. Only the body bytes are read out of the result.
func outboundFor(f Format) (transformer.Outbound, error) {
	const placeholderURL = "https://upstream.invalid"
	const placeholderKey = "placeholder"

	switch f {
	case FormatAnthropicMessages:
		return anthropictrans.NewOutboundTransformerWithConfig(&anthropictrans.Config{
			Type:           anthropictrans.PlatformDirect,
			BaseURL:        placeholderURL,
			APIKeyProvider: auth.NewStaticKeyProvider(placeholderKey),
		})
	case FormatOpenAIChatCompletions:
		return openaitrans.NewOutboundTransformerWithConfig(&openaitrans.Config{
			PlatformType:   openaitrans.PlatformOpenAI,
			BaseURL:        placeholderURL,
			APIKeyProvider: auth.NewStaticKeyProvider(placeholderKey),
		})
	case FormatOpenAIResponses:
		return openairesponses.NewOutboundTransformerWithConfig(&openairesponses.Config{
			BaseURL:        placeholderURL,
			APIKeyProvider: auth.NewStaticKeyProvider(placeholderKey),
		})
	case FormatGeminiGenerateContent, FormatGeminiStreamGenerateContent:
		return geminitrans.NewOutboundTransformerWithConfig(geminitrans.Config{
			BaseURL:        placeholderURL,
			APIKeyProvider: auth.NewStaticKeyProvider(placeholderKey),
		})
	default:
		return nil, fmt.Errorf("llmbridge: unsupported upstream format %q", f)
	}
}
