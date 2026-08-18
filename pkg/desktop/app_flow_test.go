package desktop

import (
	"context"
	"testing"

	"github.com/ajramos/giznews/internal/db"
)

func TestListArticlesUnclassified(t *testing.T) {
	app := newTestApp(t)
	ctx := context.Background()

	s, _ := app.AddSource(ctx, "S", "rss", "https://x.com/rss", "")
	repo := db.NewArticleRepo(app.db)
	// two articles: one classified, one not.
	cid, _, _ := repo.Upsert(ctx, db.NewArticle{SourceID: s.ID, GUID: "c", Title: "Classified", Category: "models", Status: db.StatusUnread})
	if err := repo.ApplyClassification(ctx, cid, "models", "sum", nil, nil, 2); err != nil {
		t.Fatal(err)
	}
	uid, _, _ := repo.Upsert(ctx, db.NewArticle{SourceID: s.ID, GUID: "u", Title: "Unclassified", Status: db.StatusUnread})

	// Unclassified filter returns only the pending one.
	all, err := app.ListArticles(ctx, ListArticlesOptions{Unclassified: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 1 || all[0].ID != uid {
		t.Fatalf("unclassified list = %+v", all)
	}
}

func TestArticleStarredAndFilters(t *testing.T) {
	app := newTestApp(t)
	ctx := context.Background()

	s, _ := app.AddSource(ctx, "S", "rss", "https://x.com/rss", "")
	repo := db.NewArticleRepo(app.db)
	id, _, _ := repo.Upsert(ctx, db.NewArticle{SourceID: s.ID, GUID: "g", Title: "T", Status: db.StatusRead})

	// star it; status stays read (orthogonal flag)
	if err := app.SetArticleStarred(ctx, id, true); err != nil {
		t.Fatal(err)
	}
	got, _ := app.GetArticle(ctx, id)
	if got.Status != "read" || !got.Starred {
		t.Fatalf("article = %+v", got)
	}

	// unarchived + starred → 1; unarchived + not starred → 0
	starred := true
	all, _ := app.ListArticles(ctx, ListArticlesOptions{Unarchived: true, Starred: &starred})
	if len(all) != 1 {
		t.Fatalf("starred unarchived = %+v", all)
	}
	notStarred := false
	all, _ = app.ListArticles(ctx, ListArticlesOptions{Unarchived: true, Starred: &notStarred})
	if len(all) != 0 {
		t.Fatalf("non-starred unarchived = %+v", all)
	}
	// archived + starred → 1
	if err := app.SetArticleStatus(ctx, id, "archived"); err != nil {
		t.Fatal(err)
	}
	all, _ = app.ListArticles(ctx, ListArticlesOptions{Status: "archived", Starred: &starred})
	if len(all) != 1 {
		t.Fatalf("archived starred = %+v", all)
	}
}

func TestFlowCounts(t *testing.T) {	app := newTestApp(t)
	ctx := context.Background()

	s, _ := app.AddSource(ctx, "S", "rss", "https://x.com/rss", "")
	repo := db.NewArticleRepo(app.db)
	if _, _, err := repo.Upsert(ctx, db.NewArticle{SourceID: s.ID, GUID: "g", Title: "T", Status: db.StatusUnread}); err != nil {
		t.Fatal(err)
	}

	fs, err := app.Flow(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if fs.SourcesTotal != 1 || fs.SourcesEnabled != 1 || fs.ArticlesTotal != 1 || fs.PendingClassify != 1 {
		t.Fatalf("flow = %+v", fs)
	}
	if fs.VaultPath == "" {
		t.Fatalf("flow vault path empty")
	}
}
