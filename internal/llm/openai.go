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

// openAIProvider implements the OpenAI Chat Completions + Embeddings API, which
// is also the de-facto standard for "custom" OpenAI-compatible backends
// (LM Studio, vLLM, Groq, Together, …). name distinguishes "openai" from
// "custom" for reporting.
type openAIProvider struct {
	name     string
	endpoint string
	apiKey   string
	timeout  time.Duration
}

func (p *openAIProvider) Name() string { return p.name }

func (p *openAIProvider) Ping(ctx context.Context) error {
	if p.apiKey == "" {
		return fmt.Errorf("%s: no API key configured", p.name)
	}
	return nil
}

func (p *openAIProvider) Complete(ctx context.Context, req CompletionRequest) (CompletionResponse, error) {
	body, err := json.Marshal(map[string]any{
		"model":    req.Model,
		"messages": req.Messages,
		"stream":   false,
	})
	if err != nil {
		return CompletionResponse{}, err
	}
	if req.Temperature > 0 {
		var m map[string]any
		_ = json.Unmarshal(body, &m)
		m["temperature"] = req.Temperature
		body, _ = json.Marshal(m)
	}

	resp, err := p.do(ctx, "/chat/completions", body)
	if err != nil {
		return CompletionResponse{}, err
	}
	defer resp.Body.Close()

	var out struct {
		Choices []struct {
			Message Message `json:"message"`
		} `json:"choices"`
		Usage Usage `json:"usage"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return CompletionResponse{}, fmt.Errorf("%s decode: %w", p.name, err)
	}
	if len(out.Choices) == 0 {
		return CompletionResponse{}, fmt.Errorf("%s: no choices in response", p.name)
	}
	return CompletionResponse{Content: out.Choices[0].Message.Content, Usage: out.Usage}, nil
}

func (p *openAIProvider) StreamingComplete(ctx context.Context, req CompletionRequest, onChunk func(string)) (CompletionResponse, error) {
	body, err := json.Marshal(map[string]any{
		"model":    req.Model,
		"messages": req.Messages,
		"stream":   true,
	})
	if err != nil {
		return CompletionResponse{}, err
	}

	resp, err := p.do(ctx, "/chat/completions", body)
	if err != nil {
		return CompletionResponse{}, err
	}
	defer resp.Body.Close()

	var full strings.Builder
	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		data := strings.TrimPrefix(line, "data: ")
		if data == "[DONE]" {
			break
		}
		var chunk struct {
			Choices []struct {
				Delta struct {
					Content string `json:"content"`
				} `json:"delta"`
			} `json:"choices"`
		}
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			continue
		}
		for _, c := range chunk.Choices {
			if c.Delta.Content != "" {
				full.WriteString(c.Delta.Content)
				if onChunk != nil {
					onChunk(c.Delta.Content)
				}
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return CompletionResponse{}, err
	}
	return CompletionResponse{Content: full.String()}, nil
}

func (p *openAIProvider) Embed(ctx context.Context, req EmbeddingRequest) (EmbeddingResponse, error) {
	body, err := json.Marshal(map[string]any{"model": req.Model, "input": req.Input})
	if err != nil {
		return EmbeddingResponse{}, err
	}

	resp, err := p.do(ctx, "/embeddings", body)
	if err != nil {
		return EmbeddingResponse{}, err
	}
	defer resp.Body.Close()

	var out struct {
		Data []struct {
			Embedding []float32 `json:"embedding"`
		} `json:"data"`
		Model string `json:"model"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return EmbeddingResponse{}, fmt.Errorf("%s embed decode: %w", p.name, err)
	}
	if len(out.Data) == 0 {
		return EmbeddingResponse{}, fmt.Errorf("%s: empty embedding result", p.name)
	}
	return EmbeddingResponse{Embedding: out.Data[0].Embedding, Model: out.Model}, nil
}

func (p *openAIProvider) do(ctx context.Context, path string, body []byte) (*http.Response, error) {
	ctx, cancel := context.WithTimeout(ctx, p.timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(p.endpoint, "/")+path, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if p.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+p.apiKey)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%s %s: %w", p.name, path, err)
	}
	if resp.StatusCode != http.StatusOK {
		msg, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		resp.Body.Close()
		return nil, fmt.Errorf("%s %s: status %d: %s", p.name, path, resp.StatusCode, strings.TrimSpace(string(msg)))
	}
	return resp, nil
}
