package desktop

import (
	"context"
	"hash/fnv"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ajramos/giznews/internal/config"
	"github.com/ajramos/giznews/internal/db"
	"github.com/ajramos/giznews/internal/llm"
)

// bagEmbeddingProvider returns deterministic bag-of-words vectors.
type bagEmbeddingProvider struct{}

func (bagEmbeddingProvider) Name() string                   { return "bag" }
func (bagEmbeddingProvider) Ping(ctx context.Context) error { return nil }
func (bagEmbeddingProvider) Embed(ctx context.Context, req llm.EmbeddingRequest) (llm.EmbeddingResponse, error) {
	vec := make([]float32, 12)
	for _, tok := range strings.Fields(strings.ToLower(req.Input)) {
		h := fnv.New32a()
		h.Write([]byte(tok))
		vec[h.Sum32()%12] += 1
	}
	return llm.EmbeddingResponse{Embedding: vec, Model: req.Model}, nil
}
func (bagEmbeddingProvider) Complete(ctx context.Context, req llm.CompletionRequest) (llm.CompletionResponse, error) {
	return llm.CompletionResponse{}, nil
}
func (bagEmbeddingProvider) StreamingComplete(ctx context.Context, req llm.CompletionRequest, onChunk func(string)) (llm.CompletionResponse, error) {
	return llm.CompletionResponse{}, nil
}

func newAppForSearch(t *testing.T) *App {
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
	app.SetProvider(bagEmbeddingProvider{})
	return app
}

func TestSearchIndexAndSearchViaAPI(t *testing.T) {
	app := newAppForSearch(t)
	ctx := context.Background()

	s, _ := app.AddSource(ctx, "HN", "hackernews", "u", "")
	repo := db.NewArticleRepo(app.db)
	_, _, _ = repo.Upsert(ctx, db.NewArticle{SourceID: s.ID, GUID: "g", Title: "agents with tools", ContentMD: "autonomous agents", Status: db.StatusUnread})
	_, _, _ = repo.Upsert(ctx, db.NewArticle{SourceID: s.ID, GUID: "g2", Title: "banana prices", ContentMD: "economics", Status: db.StatusUnread})

	kb := db.NewKBRepo(app.db)
	_, _ = kb.Create(ctx, db.NewKBNote{Type: db.NoteAtom, Title: "Agentic design", Slug: "agentic", Content: "agent loops"})

	ir, err := app.SearchIndex(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if ir.NotesEmbedded != 1 || ir.ArticlesEmbedded != 2 {
		t.Fatalf("index = %+v", ir)
	}
	if ir.FTSNotes != 1 || ir.FTSArticles != 2 {
		t.Fatalf("fts = %+v", ir)
	}

	res, err := app.Search(ctx, "agents", 5)
	if err != nil {
		t.Fatal(err)
	}
	if len(res) == 0 {
		t.Fatal("expected results")
	}
	top := strings.ToLower(res[0].Title)
	if strings.Contains(top, "banana") {
		t.Fatalf("semantic rank wrong: %+v", res)
	}
}
