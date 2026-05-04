package transform

import "github.com/oapi-codegen/nullable"

type TextContentBlock struct {
	Text string
}

type ImageContentBlock struct {
	URL string
}

type ReasoningContentBlock struct {
	Reasoning          string
	ReasoningSignature string
}

type ToolUseContentBlock struct {
	ID    string
	Input map[string]any
	Name  string
}

type ToolResultContentBlock struct {
	ToolUseID string
	Content   []ContentBlockUnion
	IsError   bool
}

type ContentBlockUnion struct {
	TextContent       *TextContentBlock
	ImageContent      *ImageContentBlock
	ReasoningContent  *ReasoningContentBlock
	ToolUseContent    *ToolUseContentBlock
	ToolResultContent *ToolResultContentBlock

	CacheType string
}

type Message struct {
	Contents []ContentBlockUnion
	Role     string
}

type ToolChoice struct {
	DisableParallelToolUse bool
	ToolChoiceType         string
	ToolChoiceName         string
}

type Tool struct {
	InputSchema    map[string]any
	Name           string
	CacheType      string
	Description    string
	ServerToolName string
}

type GenerateRequest struct {
	MaxTokens       nullable.Nullable[int]
	Messages        []Message
	Model           string
	CacheType       string
	ReasoningEffort string
	Stream          bool
	Temperature     nullable.Nullable[float32]
	ToolChoice      ToolChoice
	Tools           []Tool
	TopK            nullable.Nullable[int]
	TopP            nullable.Nullable[int]
}
