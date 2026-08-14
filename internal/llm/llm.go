// Package llm provides a provider-agnostic client for chat completions and
// embeddings, mirroring giztui's multi-provider design.
//
// Supported providers: ollama, openai, custom (OpenAI-compatible), anthropic.
// Bedrock is scaffolded but requires the AWS SDK and returns ErrNotImplemented.
package llm

import (
	"context"
	"errors"
)

// ErrNotImplemented is returned for providers/scapabilities not yet wired.
var ErrNotImplemented = errors.New("llm: not implemented yet")

// Role constants for chat messages.
const (
	RoleSystem    = "system"
	RoleUser      = "user"
	RoleAssistant = "assistant"
)

// Message is a single chat turn.
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// CompletionRequest is a chat completion call.
type CompletionRequest struct {
	Model       string
	Messages    []Message
	Temperature float64 // 0 disables temperature (provider default)
	MaxTokens   int     // 0 = provider default
}

// CompletionResponse is the result of a completion call.
type CompletionResponse struct {
	Content string
	Usage   Usage
}

// Usage tracks token consumption.
type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// EmbeddingRequest computes an embedding for a single text input.
type EmbeddingRequest struct {
	Model string
	Input string
}

// EmbeddingResponse carries the embedding vector.
type EmbeddingResponse struct {
	Embedding []float32
	Model     string
}

// Provider is the interface every LLM backend implements.
type Provider interface {
	// Name returns the provider identifier (e.g. "ollama").
	Name() string
	// Complete performs a non-streaming chat completion.
	Complete(ctx context.Context, req CompletionRequest) (CompletionResponse, error)
	// Embed computes an embedding for req.Input.
	Embed(ctx context.Context, req EmbeddingRequest) (EmbeddingResponse, error)
	// StreamingComplete streams completion tokens via onChunk. The final
	// content is also accumulated and returned.
	StreamingComplete(ctx context.Context, req CompletionRequest, onChunk func(chunk string)) (CompletionResponse, error)
	// Ping reports whether the backend is reachable.
	Ping(ctx context.Context) error
}
