package desktop

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/ajramos/giznews/internal/config"
	"github.com/ajramos/giznews/internal/db"
	"github.com/ajramos/giznews/internal/llm"
)

// stubProvider returns canned replies per method.
type stubProvider struct {
	complete string
	echo     bool // echo a classification per request id (for classify)
}

func (s *stubProvider) Name() string                   { return "stub" }
func (s *stubProvider) Ping(ctx context.Context) error { return nil }
func (s *stubProvider) Embed(ctx context.Context, req llm.EmbeddingRequest) (llm.EmbeddingResponse, error) {
	return llm.EmbeddingResponse{}, nil
}
func (s *stubProvider) StreamingComplete(ctx context.Context, req llm.CompletionRequest, onChunk func(string)) (llm.CompletionResponse, error) {
	return s.Complete(ctx, req)
}
func (s *stubProvider) Complete(ctx context.Context, req llm.CompletionRequest) (llm.CompletionResponse, error) {
	if s.echo {
		var items []struct {
			ID int64 `json:"id"`
		}
		_ = json.Unmarshal([]byte(req.Messages[1].Content), &items)
		out := make([]map[string]any, 0, len(items))
		for _, it := range items {
			out = append(out, map[string]any{
				"id": it.ID, "category": "industry", "importance": 2,
				"tags": []string{"meta"}, "summary": "Meta ships.", "headline": "h",
			})
		}
		b, _ := json.Marshal(out)
		return llm.CompletionResponse{Content: string(b)}, nil
	}
	return llm.CompletionResponse{Content: s.complete}, nil
}

func newAppWithLLM(t *testing.T, reply string, enabled bool) *App {
	t.Helper()
	cfg := config.DefaultConfig()
	cfg.DBPath = filepath.Join(t.TempDir(), "test.db")
	cfg.LLM.Enabled = enabled
	d, err := db.Open(cfg.ResolveDBPath())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { d.Close() })
	app := NewApp(cfg, d)
	if enabled {
		app.SetProvider(&stubProvider{complete: reply})
	}
	return app
}

func newAppWithEchoProvider(t *testing.T) *App {
	t.Helper()
	cfg := config.DefaultConfig()
	cfg.DBPath = filepath.Join(t.TempDir(), "test.db")
	cfg.LLM.Enabled = true
	d, err := db.Open(cfg.ResolveDBPath())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { d.Close() })
	app := NewApp(cfg, d)
	app.SetProvider(&stubProvider{echo: true})
	return app
}

func TestClassifyViaAPI(t *testing.T) {
	app := newAppWithEchoProvider(t)
	ctx := context.Background()

	s, _ := app.AddSource(ctx, "S", "rss", "https://x.com/rss", "")
	repo := db.NewArticleRepo(app.db)
	id, _, _ := repo.Upsert(ctx, db.NewArticle{SourceID: s.ID, GUID: "g", Title: "Meta releases new model", Status: db.StatusUnread})

	res, err := app.Classify(ctx, 10)
	if err != nil {
		t.Fatal(err)
	}
	if res.ByLLM != 1 {
		t.Fatalf("res = %+v", res)
	}
	got, _ := app.GetArticle(ctx, id)
	if got.Category != "industry" || got.Importance != 2 {
		t.Fatalf("article = %+v", got)
	}
}

func TestClassifyWithoutLLMDefaults(t *testing.T) {
	app := newAppWithLLM(t, "", false)
	ctx := context.Background()
	s, _ := app.AddSource(ctx, "S", "rss", "https://x.com/rss", "")
	repo := db.NewArticleRepo(app.db)
	id, _, _ := repo.Upsert(ctx, db.NewArticle{SourceID: s.ID, GUID: "g", Title: "OpenAI news", Status: db.StatusUnread})

	res, err := app.Classify(ctx, 10)
	if err != nil {
		t.Fatal(err)
	}
	if res.SkippedNoLLM != 1 {
		t.Fatalf("res = %+v", res)
	}
	got, _ := app.GetArticle(ctx, id)
	if got.Category != "general" || got.Importance != 2 {
		t.Fatalf("article = %+v", got)
	}
}

func TestSummarizeArticleViaAPI(t *testing.T) {
	app := newAppWithLLM(t, "Concise summary here.", true)
	ctx := context.Background()
	s, _ := app.AddSource(ctx, "S", "rss", "https://x.com/rss", "")
	repo := db.NewArticleRepo(app.db)
	id, _, _ := repo.Upsert(ctx, db.NewArticle{SourceID: s.ID, GUID: "g", Title: "T", ContentMD: "long body", Status: db.StatusUnread})

	got, err := app.SummarizeArticle(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	if got.Summary != "Concise summary here." {
		t.Fatalf("summary = %q", got.Summary)
	}
}

func TestSummarizeDisabled(t *testing.T) {
	app := newAppWithLLM(t, "", false)
	ctx := context.Background()
	s, _ := app.AddSource(ctx, "S", "rss", "https://x.com/rss", "")
	repo := db.NewArticleRepo(app.db)
	id, _, _ := repo.Upsert(ctx, db.NewArticle{SourceID: s.ID, GUID: "g", Title: "T", Status: db.StatusUnread})

	if _, err := app.SummarizeArticle(ctx, id); err == nil {
		t.Fatal("expected error when LLM disabled")
	}
}

func TestDigestViaAPI(t *testing.T) {
	app := newAppWithLLM(t, `{"overview":"Big week.","themes":[{"name":"models","summary":"Models moved."}]}`, true)
	ctx := context.Background()
	s, _ := app.AddSource(ctx, "S", "rss", "https://x.com/rss", "")
	repo := db.NewArticleRepo(app.db)
	_, _, _ = repo.Upsert(ctx, db.NewArticle{SourceID: s.ID, GUID: "g", Title: "T", Category: "models", Status: db.StatusUnread})

	d, err := app.Digest(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if d.Overview != "Big week." {
		t.Fatalf("overview = %q", d.Overview)
	}
	if len(d.Themes) != 1 || d.Themes[0].Theme != "models" {
		t.Fatalf("themes = %+v", d.Themes)
	}
	if len(d.Themes[0].Articles) != 1 {
		t.Fatalf("articles = %d", len(d.Themes[0].Articles))
	}
}
