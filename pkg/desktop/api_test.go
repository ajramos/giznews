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

// Importing a URL that is already in the database (via a feed, or an earlier
// :url) must not stack a second copy under a different GUID. The dedup returns
// the existing row before any network fetch.
func TestIngestURLEarlyReturnsExisting(t *testing.T) {
	app := newTestApp(t)
	ctx := context.Background()
	const url = "https://example.com/some-post"
	src, err := app.AddSource(ctx, "Feed", "rss", "https://feed.example/rss", "x")
	if err != nil {
		t.Fatal(err)
	}

	id, _, err := db.NewArticleRepo(app.db).Upsert(ctx, db.NewArticle{
		SourceID: src.ID, GUID: "feed-guid", URL: url, Title: "Some post", Status: db.StatusUnread,
	})
	if err != nil {
		t.Fatal(err)
	}

	got, err := app.IngestURL(ctx, url)
	if err != nil {
		t.Fatalf("ingest: %v", err)
	}
	if got.ID != id {
		t.Fatalf("ingest returned article %d, want the existing %d", got.ID, id)
	}
	if got.Title != "Some post" {
		t.Fatalf("title = %q, want the original", got.Title)
	}

	all, err := db.NewArticleRepo(app.db).List(ctx, db.ListOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 1 {
		t.Fatalf("articles after re-import = %d, want 1", len(all))
	}
}
