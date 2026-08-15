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

func TestFlowCounts(t *testing.T) {
	app := newTestApp(t)
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
