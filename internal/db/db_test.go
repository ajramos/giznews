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
	if version != 16 {
		t.Fatalf("user_version = %d, want 16", version)
	}
}

func TestMigrateFromV1(t *testing.T) {
	// Create a V1 database, then reopen it and confirm later migrations run.
	path := filepath.Join(t.TempDir(), "old.db")
	d, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	// Strip V2..V9 additions to simulate a V1 db.
	if _, err := d.sql.Exec("DROP INDEX IF EXISTS idx_articles_classified;"); err != nil {
		t.Fatal(err)
	}
	if _, err := d.sql.Exec("ALTER TABLE articles DROP COLUMN classified;"); err != nil {
		t.Fatal(err)
	}
	if _, err := d.sql.Exec("ALTER TABLE articles DROP COLUMN embedding;"); err != nil {
		t.Fatal(err)
	}
	if _, err := d.sql.Exec("DROP INDEX IF EXISTS idx_sources_hidden;"); err != nil {
		t.Fatal(err)
	}
	if _, err := d.sql.Exec("ALTER TABLE sources DROP COLUMN hidden;"); err != nil {
		t.Fatal(err)
	}
	if _, err := d.sql.Exec("DROP INDEX IF EXISTS idx_articles_extracted;"); err != nil {
		t.Fatal(err)
	}
	if _, err := d.sql.Exec("ALTER TABLE articles DROP COLUMN extracted;"); err != nil {
		t.Fatal(err)
	}
	if _, err := d.sql.Exec("DROP TABLE IF EXISTS digests;"); err != nil {
		t.Fatal(err)
	}
	for _, stmt := range []string{
		"DROP TABLE IF EXISTS kb_links;",
		"DROP TABLE IF EXISTS concepts;",
		"DROP TABLE IF EXISTS concept_mentions;",
		"DROP TABLE IF EXISTS concept_aliases;",
	} {
		if _, err := d.sql.Exec(stmt); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := d.sql.Exec("DROP INDEX IF EXISTS idx_articles_starred;"); err != nil {
		t.Fatal(err)
	}
	if _, err := d.sql.Exec("ALTER TABLE articles DROP COLUMN starred;"); err != nil {
		t.Fatal(err)
	}
	if _, err := d.sql.Exec("ALTER TABLE kb_notes DROP COLUMN content_hash;"); err != nil {
		t.Fatal(err)
	}
	// v13 added story_id; same treatment, or the replay hits a column that is
	// already there.
	if _, err := d.sql.Exec("DROP INDEX IF EXISTS idx_articles_story;"); err != nil {
		t.Fatal(err)
	}
	if _, err := d.sql.Exec("ALTER TABLE articles DROP COLUMN story_id;"); err != nil {
		t.Fatal(err)
	}
	// v16 added the source-health columns; same treatment.
	for _, col := range []string{"last_error", "last_ok", "consecutive_failures", "empty_cycles"} {
		if _, err := d.sql.Exec("ALTER TABLE sources DROP COLUMN " + col + ";"); err != nil {
			t.Fatal(err)
		}
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
	if version != 16 {
		t.Fatalf("user_version after reopen = %d, want 16", version)
	}
	// Columns must exist now.
	var n int
	if err := d2.sql.QueryRow("SELECT COUNT(*) FROM articles WHERE classified = 0 AND embedding IS NULL;").Scan(&n); err != nil {
		t.Fatalf("columns missing after migration: %v", err)
	}
	if rows, err := d2.sql.Query("SELECT id FROM sources WHERE hidden = 0"); err != nil {
		t.Fatalf("sources.hidden missing after migration: %v", err)
	} else {
		rows.Close()
	}
	if rows, err := d2.sql.Query("SELECT date FROM digests"); err != nil {
		t.Fatalf("digests table missing after migration: %v", err)
	} else {
		rows.Close()
	}
	if rows, err := d2.sql.Query("SELECT from_note, to_slug FROM kb_links"); err != nil {
		t.Fatalf("kb_links table missing after migration: %v", err)
	} else {
		rows.Close()
	}
	if rows, err := d2.sql.Query("SELECT slug, mentions FROM concepts"); err != nil {
		t.Fatalf("concepts table missing after migration: %v", err)
	} else {
		rows.Close()
	}
	if rows, err := d2.sql.Query("SELECT id FROM articles WHERE starred = 0"); err != nil {
		t.Fatalf("articles.starred missing after migration: %v", err)
	} else {
		rows.Close()
	}
}

// The v8 migration must recover the graph from what earlier versions had
// already written into kb_notes.wikilinks — including the concepts that never
// became notes, whose mentions were previously unrecoverable.
func TestMigrateV8BackfillsGraph(t *testing.T) {
	path := filepath.Join(t.TempDir(), "v6.db")
	d, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, stmt := range []string{
		"DROP TABLE IF EXISTS kb_links;",
		"DROP TABLE IF EXISTS concepts;",
		"DROP TABLE IF EXISTS concept_mentions;",
		"DROP TABLE IF EXISTS concept_aliases;",
	} {
		if _, err := d.sql.Exec(stmt); err != nil {
			t.Fatal(err)
		}
	}
	// v7 added the starred column; strip it so the reopen re-applies it cleanly.
	if _, err := d.sql.Exec("DROP INDEX IF EXISTS idx_articles_starred;"); err != nil {
		t.Fatal(err)
	}
	if _, err := d.sql.Exec("ALTER TABLE articles DROP COLUMN starred;"); err != nil {
		t.Fatal(err)
	}
	if _, err := d.sql.Exec("ALTER TABLE kb_notes DROP COLUMN content_hash;"); err != nil {
		t.Fatal(err)
	}
	// v13 added story_id; strip it too, or the replay hits a column that is
	// already there.
	if _, err := d.sql.Exec("DROP INDEX IF EXISTS idx_articles_story;"); err != nil {
		t.Fatal(err)
	}
	if _, err := d.sql.Exec("ALTER TABLE articles DROP COLUMN story_id;"); err != nil {
		t.Fatal(err)
	}
	// v16 added the source-health columns; strip them so the replay re-adds them.
	for _, col := range []string{"last_error", "last_ok", "consecutive_failures", "empty_cycles"} {
		if _, err := d.sql.Exec("ALTER TABLE sources DROP COLUMN " + col + ";"); err != nil {
			t.Fatal(err)
		}
	}
	// Three atoms citing "rag" (never promoted) and an existing "mamba" electron
	// cited by one of them, written the way pre-v8 builds did: JSON only.
	notes := []struct {
		id       int
		noteType string
		title    string
		slug     string
		links    string
	}{
		{1, "atom", "A one", "a-one", `["rag","mamba"]`},
		{2, "atom", "A two", "a-two", `["rag"]`},
		{3, "atom", "A three", "a-three", `["rag"]`},
		{4, "electron", "Mamba", "mamba", `["a-one"]`},
	}
	for _, n := range notes {
		if _, err := d.sql.Exec(`
			INSERT INTO kb_notes (id, note_type, title, slug, path, frontmatter, content, tags, wikilinks, created_at, updated_at)
			VALUES (?, ?, ?, ?, 'p', '{}', '', '[]', ?, '2026-08-01T00:00:00Z', '2026-08-01T00:00:00Z')`,
			n.id, n.noteType, n.title, n.slug, n.links); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := d.sql.Exec("PRAGMA user_version = 6;"); err != nil {
		t.Fatal(err)
	}
	d.Close()

	d2, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer d2.Close()

	var links int
	if err := d2.sql.QueryRow("SELECT COUNT(*) FROM kb_links").Scan(&links); err != nil {
		t.Fatal(err)
	}
	if links != 5 {
		t.Fatalf("kb_links = %d, want 5", links)
	}

	repo := NewConceptRepo(d2)
	ctx := context.Background()

	// "rag" was only ever a dangling link; its three mentions are recovered.
	rag, err := repo.Get(ctx, "rag")
	if err != nil {
		t.Fatalf("concept rag: %v", err)
	}
	if rag.Mentions != 3 || rag.NoteID != 0 {
		t.Fatalf("rag = %+v, want 3 mentions and no note", rag)
	}

	// "mamba" already had an electron: it is promoted, with its real name.
	mamba, err := repo.Get(ctx, "mamba")
	if err != nil {
		t.Fatalf("concept mamba: %v", err)
	}
	if mamba.NoteID != 4 || mamba.Name != "Mamba" || mamba.Mentions != 1 {
		t.Fatalf("mamba = %+v, want note 4, name Mamba, 1 mention", mamba)
	}

	// The electron's own outgoing link is not a mention of a concept.
	dangling, err := repo.Dangling(ctx, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(dangling) != 1 || dangling[0].Slug != "rag" {
		t.Fatalf("dangling = %+v, want only rag", dangling)
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

	if err := repo.SetStatus(ctx, id1, StatusRead, ActorUser); err != nil {
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

func TestKBIncomingIsExact(t *testing.T) {
	d := openTestDB(t)
	ctx := context.Background()
	repo := NewKBRepo(d)

	if _, err := repo.Create(ctx, NewKBNote{Type: NoteAtom, Title: "Turbo", Slug: "a1", Path: "p1",
		Wikilinks: []string{"gpt-5-turbo"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.Create(ctx, NewKBNote{Type: NoteAtom, Title: "Plain", Slug: "a2", Path: "p2",
		Wikilinks: []string{"gpt-5"}}); err != nil {
		t.Fatal(err)
	}

	got, err := repo.Incoming(ctx, "gpt-5", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Slug != "a2" {
		t.Fatalf("incoming(gpt-5) = %v, want only a2 (a1 links to gpt-5-turbo)", got)
	}
}

func TestKBUpdateDropsEmbedding(t *testing.T) {
	d := openTestDB(t)
	ctx := context.Background()
	repo := NewKBRepo(d)

	n, err := repo.Create(ctx, NewKBNote{Type: NoteElectron, Title: "E", Slug: "e", Path: "p", Content: "old"})
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.SetEmbedding(ctx, n.ID, []float32{0.1, 0.2}); err != nil {
		t.Fatal(err)
	}

	n.Content = "new"
	if err := repo.Update(ctx, n); err != nil {
		t.Fatal(err)
	}

	// The stale vector must be gone so the next index run recomputes it.
	emb, err := repo.GetEmbedding(ctx, n.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(emb) != 0 {
		t.Fatalf("embedding = %v, want none after update", emb)
	}
}
