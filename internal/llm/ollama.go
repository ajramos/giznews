package llm

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// ollamaProvider talks to a local Ollama server (/api/chat, /api/embeddings).
type ollamaProvider struct {
	endpoint string
	client   *http.Client
}

func newOllamaProvider(endpoint string, timeout time.Duration) *ollamaProvider {
	return &ollamaProvider{endpoint: endpoint, client: &http.Client{Timeout: timeout}}
}

func (p *ollamaProvider) Name() string { return "ollama" }

func (p *ollamaProvider) Ping(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.endpoint+"/api/tags", nil)
	if err != nil {
		return err
	}
	resp, err := p.client.Do(req)
	if err != nil {
		return fmt.Errorf("ollama unreachable at %s: %w", p.endpoint, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("ollama ping: status %d", resp.StatusCode)
	}
	return nil
}

func (p *ollamaProvider) Complete(ctx context.Context, req CompletionRequest) (CompletionResponse, error) {
	body, err := json.Marshal(map[string]any{
		"model":    req.Model,
		"messages": req.Messages,
		"stream":   false,
		"options":  map[string]any{"temperature": req.Temperature},
	})
	if err != nil {
		return CompletionResponse{}, err
	}

	resp, err := p.do(ctx, "/api/chat", body)
	if err != nil {
		return CompletionResponse{}, err
	}
	defer resp.Body.Close()

	var out struct {
		Message   Message `json:"message"`
		Done      bool    `json:"done"`
		EvalCount int     `json:"eval_count"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return CompletionResponse{}, fmt.Errorf("ollama decode: %w", err)
	}
	return CompletionResponse{
		Content: out.Message.Content,
		Usage:   Usage{CompletionTokens: out.EvalCount},
	}, nil
}

func (p *ollamaProvider) StreamingComplete(ctx context.Context, req CompletionRequest, onChunk func(string)) (CompletionResponse, error) {
	body, err := json.Marshal(map[string]any{
		"model":    req.Model,
		"messages": req.Messages,
		"stream":   true,
	})
	if err != nil {
		return CompletionResponse{}, err
	}

	resp, err := p.do(ctx, "/api/chat", body)
	if err != nil {
		return CompletionResponse{}, err
	}
	defer resp.Body.Close()

	var full strings.Builder
	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		var chunk struct {
			Message Message `json:"message"`
			Done    bool    `json:"done"`
		}
		if err := json.Unmarshal(scanner.Bytes(), &chunk); err != nil {
			continue
		}
		if chunk.Message.Content != "" {
			full.WriteString(chunk.Message.Content)
			if onChunk != nil {
				onChunk(chunk.Message.Content)
			}
		}
		if chunk.Done {
			break
		}
	}
	if err := scanner.Err(); err != nil {
		return CompletionResponse{}, err
	}
	return CompletionResponse{Content: full.String()}, nil
}

func (p *ollamaProvider) Embed(ctx context.Context, req EmbeddingRequest) (EmbeddingResponse, error) {
	body, err := json.Marshal(map[string]any{"model": req.Model, "input": req.Input})
	if err != nil {
		return EmbeddingResponse{}, err
	}

	resp, err := p.do(ctx, "/api/embed", body)
	if err != nil {
		return EmbeddingResponse{}, err
	}
	defer resp.Body.Close()

	var out struct {
		Embeddings [][]float32 `json:"embeddings"`
		Model      string      `json:"model"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return EmbeddingResponse{}, fmt.Errorf("ollama embed decode: %w", err)
	}
	if len(out.Embeddings) == 0 {
		return EmbeddingResponse{}, fmt.Errorf("ollama embed: empty result for model %q", req.Model)
	}
	return EmbeddingResponse{Embedding: out.Embeddings[0], Model: out.Model}, nil
}

func (p *ollamaProvider) do(ctx context.Context, path string, body []byte) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.endpoint+path, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	// client.Timeout covers connection, request AND body reads, so long
	// generations are bounded without canceling mid-read.
	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("ollama %s: %w", path, err)
	}
	if resp.StatusCode != http.StatusOK {
		msg, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		resp.Body.Close()
		return nil, fmt.Errorf("ollama %s: status %d: %s", path, resp.StatusCode, strings.TrimSpace(string(msg)))
	}
	return resp, nil
}
