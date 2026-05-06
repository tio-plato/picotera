package server

import (
	"context"
	"encoding/json"

	"picotera/pkg/artifacts"
	"picotera/pkg/contract"
	"picotera/pkg/llmbridge"
	"picotera/pkg/logx"
)

func responseAggregationFormat(endpointType int32) (llmbridge.Format, bool) {
	switch endpointType {
	case contract.EndpointType_AnthropicMessages:
		return llmbridge.FormatAnthropicMessages, true
	case contract.EndpointType_OpenAIChatCompletions:
		return llmbridge.FormatOpenAIChatCompletions, true
	case contract.EndpointType_OpenAIResponses:
		return llmbridge.FormatOpenAIResponses, true
	case contract.EndpointType_GeminiStreamGenerateContent:
		return llmbridge.FormatGeminiStreamGenerateContent, true
	default:
		return llmbridge.FormatUnknown, false
	}
}

func buildAggregatedArtifact(ctx context.Context, format llmbridge.Format, contentType string, body []byte, profile llmbridge.OutboundProfile) *artifacts.AggregatedResponse {
	kind := llmbridge.StreamAggregationKind(format, contentType)
	if kind == llmbridge.StreamAggregationNone {
		return nil
	}
	aggregated := &artifacts.AggregatedResponse{
		Format:       format.String(),
		BodyEncoding: "json",
	}
	resp, err := llmbridge.AggregateStream(ctx, format, contentType, body, profile)
	if err != nil {
		logx.WithContext(ctx).WithError(err).WithField("format", format.String()).Warn("artifact: aggregate response stream failed")
		aggregated.Error = err.Error()
		return aggregated
	}
	aggregated.Body = json.RawMessage(resp)
	return aggregated
}

func defaultAggregationProfile(format llmbridge.Format) (llmbridge.OutboundProfile, bool) {
	profile, err := llmbridge.DefaultOutboundProfileForFormat(format)
	if err != nil {
		return llmbridge.OutboundProfile{}, false
	}
	return profile, true
}
