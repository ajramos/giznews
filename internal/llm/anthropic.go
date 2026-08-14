package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// anthropicProvider implements the Anthropic Messages API (v1/messages).
type anthropicProvider struct {
	endpoint string
	apiKey   string
	timeout  time.Duration
}

func (p *anthropicProvider) Name() string { return "anthropic" }

func (p *anthropicProvider) Ping(ctx context.Context) error {
	if p.apiKey == "" {
		return fmt.Errorf("anthropic: no API key configured")
	}
	return nil
}

func (p *anthropicProvider) Complete(ctx context.Context, req CompletionRequest) (CompletionResponse, error) {
	messages := make([]map[string]string, 0, len(req.Messages))
	for _, m := range req.Messages {
		// Anthropic forbids the system role in messages[]; it goes top-level.
		if m.Role == RoleSystem {
			continue
		}
		messages = append(messages, map[string]string{"role": m.Role, "content": m.Content})
	}

	payload := map[string]any{
		"model":      req.Model,
		"messages":   messages,
		"max_tokens": maxTokens(req.MaxTokens),
	}
	if len(req.Messages) > 0 && req.Messages[0].Role == RoleSystem {
		payload["system"] = req.Messages[0].Content
	}
	if req.Temperature > 0 {
		payload["temperature"] = req.Temperature
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return CompletionResponse{}, err
	}

	ctx, cancel := context.WithTimeout(ctx, p.timeout)
	defer cancel()

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(p.endpoint, "/")+"/v1/messages", bytes.NewReader(body))
	if err != nil {
		return CompletionResponse{}, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", p.apiKey)
	httpReq.Header.Set("anthropic-version", "2023-06-01")

	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		return CompletionResponse{}, fmt.Errorf("anthropic messages: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		msg, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return CompletionResponse{}, fmt.Errorf("anthropic messages: status %d: %s", resp.StatusCode, strings.TrimSpace(string(msg)))
	}

	var out struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
		Usage struct {
			InputTokens  int `json:"input_tokens"`
			OutputTokens int `json:"output_tokens"`
		} `json:"usage"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return CompletionResponse{}, fmt.Errorf("anthropic decode: %w", err)
	}

	var text strings.Builder
	for _, c := range out.Content {
		if c.Type == "text" {
			text.WriteString(c.Text)
		}
	}
	return CompletionResponse{
		Content: text.String(),
		Usage:   Usage{PromptTokens: out.Usage.InputTokens, CompletionTokens: out.Usage.OutputTokens},
	}, nil
}

func (p *anthropicProvider) StreamingComplete(ctx context.Context, req CompletionRequest, onChunk func(string)) (CompletionResponse, error) {
	// Non-streaming fallback keeps the API surface honest without SSE wiring.
	resp, err := p.Complete(ctx, req)
	if err != nil {
		return CompletionResponse{}, err
	}
	if onChunk != nil {
		onChunk(resp.Content)
	}
	return resp, nil
}

func (p *anthropicProvider) Embed(ctx context.Context, req EmbeddingRequest) (EmbeddingResponse, error) {
	return EmbeddingResponse{}, fmt.Errorf("anthropic: %w", ErrNotImplemented)
}

func maxTokens(v int) int {
	if v <= 0 {
		return 4096
	}
	return v
}
