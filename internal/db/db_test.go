package db

import (
	"context"
	"path/filepath"
	"testing"
)

func openTestDB(t *testing.T) *DB {
	t.Helper()
	d, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { d.Close() })
	return d
}

func TestMigrateFresh(t *testing.T) {
	d := openTestDB(t)
	var version int
	if err := d.sql.QueryRow("PRAGMA user_version;").Scan(&version); err != nil {
		t.Fatal(err)
	}
	if version != 2 {
		t.Fatalf("user_version = %d, want 2", version)
	}
}

func TestMigrateFromV1(t *testing.T) {
	// Create a V1 database, then reopen it and confirm V2 migration runs.
	path := filepath.Join(t.TempDir(), "old.db")
	d, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	// Force version back to 1 and drop the V2 column to simulate a V1 db.
	if _, err := d.sql.Exec("DROP INDEX IF EXISTS idx_articles_classified;"); err != nil {
		t.Fatal(err)
	}
	if _, err := d.sql.Exec("ALTER TABLE articles DROP COLUMN classified;"); err != nil {
		t.Fatal(err)
	}
	if _, err := d.sql.Exec("PRAGMA user_version = 1;"); err != nil {
		t.Fatal(err)
	}
	d.Close()

	d2, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer d2.Close()
	var version int
	if err := d2.sql.QueryRow("PRAGMA user_version;").Scan(&version); err != nil {
		t.Fatal(err)
	}
	if version != 2 {
		t.Fatalf("user_version after reopen = %d, want 2", version)
	}
	// Column must exist now.
	var n int
	if err := d2.sql.QueryRow("SELECT COUNT(*) FROM articles WHERE classified = 0;").Scan(&n); err != nil {
		t.Fatalf("classified column missing: %v", err)
	}
}

func TestSourceCRUD(t *testing.T) {
	d := openTestDB(t)
	ctx := context.Background()
	repo := NewSourceRepo(d)

	s, err := repo.Create(ctx, NewSource{
		Name: "OpenAI Blog", Type: SourceRSS, URL: "https://openai.com/blog/rss.xml",
		Group: "labs", Enabled: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if s.ID == 0 {
		t.Fatal("expected non-zero id")
	}
	if !s.Enabled {
		t.Fatal("expected enabled")
	}

	got, err := repo.GetByName(ctx, "OpenAI Blog")
	if err != nil {
		t.Fatal(err)
	}
	if got.URL != "https://openai.com/blog/rss.xml" {
		t.Fatalf("url = %q", got.URL)
	}

	if _, err := repo.Create(ctx, NewSource{Name: "OpenAI Blog", Type: SourceRSS}); err == nil {
		t.Fatal("expected duplicate name error")
	}

	all, err := repo.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 1 {
		t.Fatalf("sources = %d, want 1", len(all))
	}

	if err := repo.SetEnabled(ctx, s.ID, false); err != nil {
		t.Fatal(err)
	}
	got, _ = repo.Get(ctx, s.ID)
	if got.Enabled {
		t.Fatal("expected disabled")
	}
	enabled, _ := repo.ListEnabled(ctx)
	if len(enabled) != 0 {
		t.Fatalf("enabled sources = %d, want 0", len(enabled))
	}

	if err := repo.Delete(ctx, s.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.Get(ctx, s.ID); err != ErrNotFound {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestArticleUpsertAndQuery(t *testing.T) {
	d := openTestDB(t)
	ctx := context.Background()
	sources := NewSourceRepo(d)
	repo := NewArticleRepo(d)

	s, err := sources.Create(ctx, NewSource{Name: "HN", Type: SourceHackerNews, URL: "https://hn.algolia.com"})
	if err != nil {
		t.Fatal(err)
	}

	na := NewArticle{
		SourceID:   s.ID,
		GUID:       "hn-424242",
		URL:        "https://news.ycombinator.com/item?id=424242",
		Title:      "Introducing GizNews",
		Author:     "ajramos",
		ContentMD:  "A keyboard-first AI news reader.",
		Tags:       []string{"tui", "go"},
		Entities:   []Entity{{Name: "GizNews", Type: "product"}},
		Importance: 2,
		Status:     StatusUnread,
	}

	id1, created, err := repo.Upsert(ctx, na)
	if err != nil {
		t.Fatal(err)
	}
	if !created {
		t.Fatal("expected created=true on first insert")
	}

	// Upsert again: should update, not duplicate.
	id2, created, err := repo.Upsert(ctx, na)
	if err != nil {
		t.Fatal(err)
	}
	if created {
		t.Fatal("expected created=false on conflict")
	}
	if id1 != id2 {
		t.Fatalf("id changed: %d -> %d", id1, id2)
	}

	count, _ := repo.Count(ctx, "")
	if count != 1 {
		t.Fatalf("count = %d, want 1", count)
	}

	got, err := repo.Get(ctx, id1)
	if err != nil {
		t.Fatal(err)
	}
	if got.SourceName != "HN" {
		t.Fatalf("source name = %q", got.SourceName)
	}
	if len(got.Tags) != 2 || got.Tags[0] != "tui" {
		t.Fatalf("tags = %v", got.Tags)
	}
	if len(got.Entities) != 1 || got.Entities[0].Name != "GizNews" {
		t.Fatalf("entities = %v", got.Entities)
	}

	if err := repo.SetStatus(ctx, id1, StatusRead); err != nil {
		t.Fatal(err)
	}
	got, _ = repo.Get(ctx, id1)
	if got.Status != StatusRead {
		t.Fatalf("status = %q", got.Status)
	}

	// List with status filter.
	list, err := repo.List(ctx, ListOptions{Status: StatusRead})
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 {
		t.Fatalf("read articles = %d, want 1", len(list))
	}

	// Importance filter.
	list, err = repo.List(ctx, ListOptions{ImportanceMin: 3})
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 0 {
		t.Fatalf("importance>=3 should be empty, got %d", len(list))
	}
}

func TestArticleSimhash(t *testing.T) {
	d := openTestDB(t)
	ctx := context.Background()
	sources := NewSourceRepo(d)
	repo := NewArticleRepo(d)
	s, _ := sources.Create(ctx, NewSource{Name: "Src", Type: SourceRSS})

	_, _, err := repo.Upsert(ctx, NewArticle{SourceID: s.ID, GUID: "a", Title: "x", SimHash: 12345})
	if err != nil {
		t.Fatal(err)
	}
	exists, err := repo.ExistsSimhash(ctx, 12345)
	if err != nil {
		t.Fatal(err)
	}
	if !exists {
		t.Fatal("expected simhash to exist")
	}
	exists, _ = repo.ExistsSimhash(ctx, 999)
	if exists {
		t.Fatal("expected simhash to not exist")
	}
}
