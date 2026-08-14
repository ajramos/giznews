package digest

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ajramos/giznews/internal/db"
	"github.com/ajramos/giznews/internal/llm"
)

// fakeProvider implements llm.Provider for deterministic tests.
type fakeProvider struct {
	reply string
}

func (f *fakeProvider) Name() string                   { return "fake" }
func (f *fakeProvider) Ping(ctx context.Context) error { return nil }
func (f *fakeProvider) Embed(ctx context.Context, req llm.EmbeddingRequest) (llm.EmbeddingResponse, error) {
	return llm.EmbeddingResponse{}, nil
}
func (f *fakeProvider) StreamingComplete(ctx context.Context, req llm.CompletionRequest, onChunk func(string)) (llm.CompletionResponse, error) {
	return f.Complete(ctx, req)
}
func (f *fakeProvider) Complete(ctx context.Context, req llm.CompletionRequest) (llm.CompletionResponse, error) {
	return llm.CompletionResponse{Content: f.reply}, nil
}

func newTestDB(t *testing.T) *db.DB {
	t.Helper()
	d, err := db.Open(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { d.Close() })
	return d
}

func seedArticles(t *testing.T, d *db.DB) {
	t.Helper()
	ctx := context.Background()
	src, err := db.NewSourceRepo(d).Create(ctx, db.NewSource{Name: "HN", Type: db.SourceHackerNews, URL: "u"})
	if err != nil {
		t.Fatal(err)
	}
	repo := db.NewArticleRepo(d)
	for i, c := range []struct{ guid, cat string }{
		{"a1", "models"}, {"a2", "models"}, {"a3", "research"}, {"a4", "research"}, {"a5", "models"},
	} {
		if _, _, err := repo.Upsert(ctx, db.NewArticle{
			SourceID: src.ID, GUID: c.guid, Title: "story " + c.guid,
			Category: c.cat, Importance: i % 3, Status: db.StatusUnread,
		}); err != nil {
			t.Fatal(err)
		}
	}
}

func TestDigestDeterministic(t *testing.T) {
	d := newTestDB(t)
	seedArticles(t, d)

	svc := NewService(d, Options{Days: 7, MaxArticlesPerTheme: 3, UseLLM: false}, nil, nil)
	dig, err := svc.Generate(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(dig.Themes) != 2 {
		t.Fatalf("themes = %d, want 2 (%+v)", len(dig.Themes), dig.Themes)
	}
	for _, th := range dig.Themes {
		if th.Name == "models" && len(th.Articles) != 3 {
			t.Fatalf("models articles = %d, want 3", len(th.Articles))
		}
	}
	if dig.Overview != "" {
		t.Fatalf("overview should be empty without LLM")
	}
}

func TestDigestWithLLM(t *testing.T) {
	d := newTestDB(t)
	seedArticles(t, d)

	reply, _ := json.Marshal(map[string]any{
		"overview": "Models dominated the week.",
		"themes": []map[string]any{
			{"name": "models", "summary": "Two model stories."},
			{"name": "research", "summary": "One research story."},
		},
	})
	prov := &fakeProvider{reply: string(reply)}

	svc := NewService(d, Options{Days: 7, MaxArticlesPerTheme: 3, UseLLM: true, Model: "m"}, prov, nil)
	dig, err := svc.Generate(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(dig.Overview, "Models dominated") {
		t.Fatalf("overview = %q", dig.Overview)
	}
	if dig.Themes[0].Summary == "" {
		t.Fatal("expected theme summary")
	}
}

func TestDigestLLMFallback(t *testing.T) {
	d := newTestDB(t)
	seedArticles(t, d)
	prov := &fakeProvider{reply: "this is not json"}

	svc := NewService(d, Options{Days: 7, MaxArticlesPerTheme: 3, UseLLM: true, Model: "m"}, prov, nil)
	dig, err := svc.Generate(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(dig.Themes) != 2 {
		t.Fatalf("themes = %d", len(dig.Themes))
	}
	if dig.Overview != "" {
		t.Fatal("overview should be empty after fallback")
	}
}
