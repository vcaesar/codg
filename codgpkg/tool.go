// Package codgpkg defines the core types shared between the Codg
// plugin system and its host application. It is intentionally
// dependency-free so that plugin authors can import it without
// pulling in provider SDKs or heavy transitive modules.
package codgpkg

import (
	"context"
	"encoding/json"
)

// ToolInfo describes a tool that can be offered to a language model.
type ToolInfo struct {
	Name        string         `json:"name"`        // Unique identifier.
	Description string         `json:"description"` // Human-readable purpose.
	Parameters  map[string]any `json:"parameters"`  // JSON Schema describing accepted input.
	Required    []string       `json:"required"`    // Required parameter names.
	Parallel    bool           `json:"parallel"`    // Whether this tool can run in parallel with others.
}

// ToolCall represents a single tool invocation issued by the model.
type ToolCall struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Input string `json:"input"`
}

// ToolResponse carries the result of a [ToolCall] execution back to
// the model.
type ToolResponse struct {
	Type    string `json:"type"`
	Content string `json:"content"`
	Data      []byte `json:"data,omitempty"`       // Binary payload (images, audio, etc.).
	MediaType string `json:"media_type,omitempty"` // MIME type of Data (e.g. "image/png").
	Metadata  string `json:"metadata,omitempty"`
	IsError   bool   `json:"is_error"`
}

// NewTextResponse returns a successful text [ToolResponse].
func NewTextResponse(content string) ToolResponse {
	return ToolResponse{
		Type:    "text",
		Content: content,
	}
}

// NewTextErrorResponse returns a text [ToolResponse] marked as an error.
func NewTextErrorResponse(content string) ToolResponse {
	return ToolResponse{
		Type:    "text",
		Content: content,
		IsError: true,
	}
}

// NewImageResponse returns a [ToolResponse] carrying an image payload.
func NewImageResponse(data []byte, mediaType string) ToolResponse {
	return ToolResponse{
		Type:      "image",
		Data:      data,
		MediaType: mediaType,
	}
}

// NewMediaResponse returns a [ToolResponse] carrying arbitrary media
// (audio, video, etc.).
func NewMediaResponse(data []byte, mediaType string) ToolResponse {
	return ToolResponse{
		Type:      "media",
		Data:      data,
		MediaType: mediaType,
	}
}

// WithResponseMetadata returns a copy of response with the given
// metadata JSON-marshalled into [ToolResponse.Metadata].
func WithResponseMetadata(response ToolResponse, metadata any) ToolResponse {
	if metadata != nil {
		metadataBytes, err := json.Marshal(metadata)
		if err != nil {
			return response
		}
		response.Metadata = string(metadataBytes)
	}
	return response
}

// AgentTool is the interface that every tool callable by a language
// model must implement.
type AgentTool interface {
	Info() ToolInfo
	Run(ctx context.Context, params ToolCall) (ToolResponse, error)
	ProviderOptions() ProviderOptions
	SetProviderOptions(opts ProviderOptions)
}
