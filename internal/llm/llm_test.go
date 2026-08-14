package llm

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func TestFactoryUnknownProvider(t *testing.T) {
	if _, err := NewProvider(Options{Provider: "wat"}); err == nil {
		t.Fatal("expected error for unknown provider")
	}
}

func TestFactoryBedrockNotImplemented(t *testing.T) {
	if _, err := NewProvider(Options{Provider: "bedrock"}); err == nil {
		t.Fatal("expected ErrNotImplemented for bedrock")
	}
}

func TestFactoryDefaults(t *testing.T) {
	p, err := NewProvider(Options{Provider: "ollama"})
	if err != nil {
		t.Fatal(err)
	}
	if p.Name() != "ollama" {
		t.Fatalf("name = %q", p.Name())
	}
}

func TestOllamaCompleteAndEmbed(t *testing.T) {
	var embedCalls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/tags":
			_ = json.NewEncoder(w).Encode(map[string]any{"models": []any{}})
		case "/api/chat":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"message":    map[string]any{"role": "assistant", "content": "hello from ollama"},
				"done":       true,
				"eval_count": 7,
			})
		case "/api/embed":
			embedCalls.Add(1)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"embeddings": [][]float32{{0.1, 0.2, 0.3}},
				"model":      "nomic-embed-text",
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	p := &ollamaProvider{endpoint: srv.URL, timeout: 5 * time.Second}
	ctx := context.Background()

	resp, err := p.Complete(ctx, CompletionRequest{Model: "llama3.2", Messages: []Message{{Role: RoleUser, Content: "hi"}}})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Content != "hello from ollama" {
		t.Fatalf("content = %q", resp.Content)
	}
	if resp.Usage.CompletionTokens != 7 {
		t.Fatalf("completion tokens = %d", resp.Usage.CompletionTokens)
	}

	emb, err := p.Embed(ctx, EmbeddingRequest{Model: "nomic-embed-text", Input: "text"})
	if err != nil {
		t.Fatal(err)
	}
	if len(emb.Embedding) != 3 || emb.Embedding[2] != 0.3 {
		t.Fatalf("embedding = %v", emb.Embedding)
	}

	if err := p.Ping(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestOllamaStreaming(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/chat" {
			w.Write([]byte("{\"message\":{\"role\":\"assistant\",\"content\":\"a\"}}\n"))
			w.Write([]byte("{\"message\":{\"role\":\"assistant\",\"content\":\"b\"},\"done\":true}\n"))
		}
	}))
	defer srv.Close()

	p := &ollamaProvider{endpoint: srv.URL, timeout: 5 * time.Second}
	var got string
	resp, err := p.StreamingComplete(context.Background(),
		CompletionRequest{Model: "m", Messages: []Message{{Role: RoleUser, Content: "x"}}},
		func(chunk string) { got += chunk })
	if err != nil {
		t.Fatal(err)
	}
	if resp.Content != "ab" || got != "ab" {
		t.Fatalf("content=%q streamed=%q", resp.Content, got)
	}
}

func TestOpenAICompleteAndEmbed(t *testing.T) {
	var embedCalls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer sk-test" {
			http.Error(w, "missing auth", http.StatusUnauthorized)
			return
		}
		switch r.URL.Path {
		case "/chat/completions":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"choices": []any{map[string]any{
					"message": map[string]any{"role": "assistant", "content": "ok"},
				}},
				"usage": map[string]any{"prompt_tokens": 3, "completion_tokens": 2, "total_tokens": 5},
			})
		case "/embeddings":
			embedCalls.Add(1)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data":  []any{map[string]any{"embedding": []float32{0.5, 0.25}}},
				"model": "text-embedding-3-small",
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	p := &openAIProvider{name: "openai", endpoint: srv.URL, apiKey: "sk-test", timeout: 5 * time.Second}
	ctx := context.Background()

	resp, err := p.Complete(ctx, CompletionRequest{Model: "gpt-4o-mini", Messages: []Message{{Role: RoleUser, Content: "hi"}}})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Content != "ok" {
		t.Fatalf("content = %q", resp.Content)
	}
	if resp.Usage.TotalTokens != 5 {
		t.Fatalf("total tokens = %d", resp.Usage.TotalTokens)
	}

	emb, err := p.Embed(ctx, EmbeddingRequest{Model: "text-embedding-3-small", Input: "text"})
	if err != nil {
		t.Fatal(err)
	}
	if len(emb.Embedding) != 2 {
		t.Fatalf("embedding = %v", emb.Embedding)
	}
	if embedCalls.Load() != 1 {
		t.Fatalf("embed calls = %d", embedCalls.Load())
	}
}

func TestOpenAIStreaming(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"he\"}}]}\n\n"))
		w.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"llo\"}}]}\n\n"))
		w.Write([]byte("data: [DONE]\n\n"))
	}))
	defer srv.Close()

	p := &openAIProvider{name: "custom", endpoint: srv.URL, timeout: 5 * time.Second}
	var got string
	resp, err := p.StreamingComplete(context.Background(),
		CompletionRequest{Model: "m", Messages: []Message{{Role: RoleUser, Content: "x"}}},
		func(chunk string) { got += chunk })
	if err != nil {
		t.Fatal(err)
	}
	if resp.Content != "hello" || got != "hello" {
		t.Fatalf("content=%q streamed=%q", resp.Content, got)
	}
}

func TestAnthropicComplete(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/messages" {
			http.NotFound(w, r)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"content": []any{
				map[string]any{"type": "text", "text": "anthropic answer"},
			},
			"usage": map[string]any{"input_tokens": 10, "output_tokens": 3},
		})
	}))
	defer srv.Close()

	p := &anthropicProvider{endpoint: srv.URL, apiKey: "sk-ant", timeout: 5 * time.Second}
	resp, err := p.Complete(context.Background(), CompletionRequest{
		Model: "claude-sonnet-4-20250514",
		Messages: []Message{
			{Role: RoleSystem, Content: "be brief"},
			{Role: RoleUser, Content: "hello"},
		},
		MaxTokens: 128,
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Content != "anthropic answer" {
		t.Fatalf("content = %q", resp.Content)
	}
	if resp.Usage.PromptTokens != 10 {
		t.Fatalf("prompt tokens = %d", resp.Usage.PromptTokens)
	}
}
