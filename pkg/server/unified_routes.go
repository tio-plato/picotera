package server

import "picotera/pkg/llmbridge"

// unifiedRoute binds a unified generation route's URL path to its source
// format and display name. The unified routes are runtime constants mounted
// directly on the router (see server.go registerEndpoints) — they are NOT rows
// in the endpoint table — so this list is the single source of truth shared by:
//   - route registration (server.go),
//   - the simulator (handle_simulate.go unifiedRoutePath),
//   - the endpoint label list (handle_label.go), which surfaces them in the
//     requests page endpoint filter alongside the path-table endpoints.
var unifiedRoutes = []unifiedRoute{
	{Path: "/api/unified/v1/messages", Format: llmbridge.FormatAnthropicMessages, Name: "Unified Anthropic Messages"},
	{Path: "/api/unified/v1/responses", Format: llmbridge.FormatOpenAIResponses, Name: "Unified OpenAI Responses"},
	{Path: "/api/unified/v1/chat/completions", Format: llmbridge.FormatOpenAIChatCompletions, Name: "Unified OpenAI Chat Completions"},
	{Path: "/api/unified/v1beta/models/{model}:generateContent", Format: llmbridge.FormatGeminiGenerateContent, Name: "Unified Gemini GenerateContent"},
	{Path: "/api/unified/v1beta/models/{model}:streamGenerateContent", Format: llmbridge.FormatGeminiStreamGenerateContent, Name: "Unified Gemini streamGenerateContent"},
}

type unifiedRoute struct {
	Path   string
	Format llmbridge.Format
	Name   string
}

// unifiedRoutePath returns the canonical path the matching unified route is
// mounted on, or "" if the format has no unified route.
func unifiedRoutePath(f llmbridge.Format) string {
	for _, r := range unifiedRoutes {
		if r.Format == f {
			return r.Path
		}
	}
	return ""
}
