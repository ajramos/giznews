package desktop

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/ajramos/giznews/internal/config"
	"github.com/ajramos/giznews/internal/db"
)

func newTestApp(t *testing.T) *App {
	t.Helper()
	cfg := config.DefaultConfig()
	cfg.DBPath = filepath.Join(t.TempDir(), "test.db")
	d, err := db.Open(cfg.ResolveDBPath())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { d.Close() })
	return NewApp(cfg, d)
}

func TestSourcesRoundTrip(t *testing.T) {
	app := newTestApp(t)
	ctx := context.Background()

	s, err := app.AddSource(ctx, "HN", "hackernews", "https://hn.algolia.com", "community")
	if err != nil {
		t.Fatal(err)
	}
	if s.ID == 0 || !s.Enabled {
		t.Fatalf("source = %+v", s)
	}

	all, err := app.ListSources(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 1 || all[0].Name != "HN" {
		t.Fatalf("sources = %+v", all)
	}

	if err := app.SetSourceEnabled(ctx, s.ID, false); err != nil {
		t.Fatal(err)
	}
	all, _ = app.ListSources(ctx)
	if all[0].Enabled {
		t.Fatal("expected disabled")
	}
}

func TestArticlesRoundTrip(t *testing.T) {
	app := newTestApp(t)
	ctx := context.Background()

	s, _ := app.AddSource(ctx, "Blog", "rss", "https://blog.example/rss", "")
	repo := db.NewArticleRepo(app.db)

	id, created, err := repo.Upsert(ctx, db.NewArticle{
		SourceID: s.ID, GUID: "g1", URL: "https://example.com/1", Title: "First AI story",
		ContentMD: "Body", Importance: 3, Status: db.StatusUnread,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !created {
		t.Fatal("expected created")
	}

	list, err := app.ListArticles(ctx, ListArticlesOptions{ImportanceMin: 2})
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 || list[0].Title != "First AI story" {
		t.Fatalf("list = %+v", list)
	}
	if list[0].SourceName != "Blog" {
		t.Fatalf("source name = %q", list[0].SourceName)
	}

	got, err := app.GetArticle(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	if got.ContentMD != "Body" {
		t.Fatalf("content = %q", got.ContentMD)
	}

	if err := app.SetArticleStatus(ctx, id, "read"); err != nil {
		t.Fatal(err)
	}
	got, _ = app.GetArticle(ctx, id)
	if got.Status != "read" {
		t.Fatalf("status = %q", got.Status)
	}
}

func TestStatus(t *testing.T) {
	app := newTestApp(t)
	ctx := context.Background()

	s, _ := app.AddSource(ctx, "Blog", "rss", "https://blog.example/rss", "")
	repo := db.NewArticleRepo(app.db)
	_, _, _ = repo.Upsert(ctx, db.NewArticle{SourceID: s.ID, GUID: "a", Title: "x", Status: db.StatusUnread})

	st, err := app.Status(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if st.TotalArticles != 1 || st.UnreadArticles != 1 {
		t.Fatalf("status = %+v", st)
	}
	if st.LLMProvider != "ollama" {
		t.Fatalf("provider = %q", st.LLMProvider)
	}
}

func TestNotImplementedMethods(t *testing.T) {
	app := newTestApp(t)
	ctx := context.Background()
	for name, fn := range map[string]func() error{
		"fetch":     func() error { _, err := app.Fetch(ctx); return err },
		"digest":    func() error { _, err := app.Digest(ctx); return err },
		"listnotes": func() error { _, err := app.ListNotes(ctx, "atom"); return err },
		"getnote":   func() error { _, err := app.GetNote(ctx, 1); return err },
		"neighbors": func() error { _, err := app.GraphNeighbors(ctx, 1); return err },
		"search":    func() error { _, err := app.Search(ctx, "x", 10); return err },
	} {
		if err := fn(); err == nil {
			t.Fatalf("%s: expected not-implemented error", name)
		}
	}
}
