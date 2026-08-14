package extract

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ajramos/giznews/internal/db"
)

func openDB(t *testing.T) *db.DB {
	t.Helper()
	d, err := db.Open(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { d.Close() })
	return d
}

const articleHTML = `<!doctype html><html><head><title>Test Article</title></head><body>
<h1>Test Article</h1>
<article>
<p>This is the first substantial paragraph of the article body, long enough to pass the minimum readability threshold for extraction and conversion to markdown.</p>
<p>A second paragraph continues the story with more details about the topic, the people involved and the implications for the industry.</p>
</article></body></html>`

func TestExtractPending(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(articleHTML))
	}))
	defer srv.Close()

	d := openDB(t)
	ctx := context.Background()

	src, _ := db.NewSourceRepo(d).Create(ctx, db.NewSource{Name: "S", Type: db.SourceRSS, URL: "u"})
	repo := db.NewArticleRepo(d)

	// Article with short content → pending extraction.
	short, _, _ := repo.Upsert(ctx, db.NewArticle{SourceID: src.ID, GUID: "short", Title: "Short", URL: srv.URL, ContentMD: "[Comments](x)", Status: db.StatusUnread})
	// Article already extracted → should be skipped.
	_, _, _ = repo.Upsert(ctx, db.NewArticle{SourceID: src.ID, GUID: "done", Title: "Done", URL: srv.URL, ContentMD: "long enough content that exceeds the threshold by far 1234567890 1234567890 1234567890", Status: db.StatusUnread})
	_ = repo.SetExtracted(ctx, short, false)
	_ = repo.SetExtracted(ctx, 2, true)

	pending, err := repo.ListPendingExtract(ctx, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(pending) != 1 || pending[0].GUID != "short" {
		t.Fatalf("pending = %+v", pending)
	}

	svc := NewService(d)
	done, err := svc.ExtractPending(ctx, 10, 2)
	if err != nil {
		t.Fatal(err)
	}
	if done != 1 {
		t.Fatalf("extracted = %d, want 1", done)
	}

	art, err := repo.Get(ctx, short)
	if err != nil {
		t.Fatal(err)
	}
	if len(art.ContentMD) < MinLength {
		t.Fatalf("content_md too short after extract: %d", len(art.ContentMD))
	}
	if !strings.Contains(art.ContentMD, "first substantial paragraph") {
		t.Fatalf("extracted body wrong: %s", art.ContentMD[:200])
	}
	// Marked extracted → no longer pending.
	pending2, _ := repo.ListPendingExtract(ctx, 10)
	if len(pending2) != 0 {
		t.Fatalf("still pending: %+v", pending2)
	}
}

func TestExtractArticleNoContent(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(`<html><head><title>x</title></head><body><script>// js only</script></body></html>`))
	}))
	defer srv.Close()

	d := openDB(t)
	ctx := context.Background()
	src, _ := db.NewSourceRepo(d).Create(ctx, db.NewSource{Name: "S", Type: db.SourceRSS, URL: "u"})
	id, _, _ := db.NewArticleRepo(d).Upsert(ctx, db.NewArticle{SourceID: src.ID, GUID: "g", Title: "G", URL: srv.URL, ContentMD: "x", Status: db.StatusUnread})

	err := NewService(d).ExtractArticle(ctx, &db.Article{ID: id, URL: srv.URL})
	if err == nil {
		t.Fatal("expected error for non-readable content")
	}
}

