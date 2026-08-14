package desktop

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/ajramos/giznews/internal/config"
	"github.com/ajramos/giznews/internal/db"
)

func newAppForKB(t *testing.T) *App {
	t.Helper()
	cfg := config.DefaultConfig()
	cfg.DBPath = filepath.Join(t.TempDir(), "test.db")
	cfg.VaultPath = filepath.Join(t.TempDir(), "vault")
	d, err := db.Open(cfg.ResolveDBPath())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { d.Close() })
	return NewApp(cfg, d)
}

func TestKBuildViaAPI(t *testing.T) {
	app := newAppForKB(t)
	ctx := context.Background()

	s, _ := app.AddSource(ctx, "HN", "hackernews", "u", "")
	repo := db.NewArticleRepo(app.db)
	for _, title := range []string{"Agentic loops", "Agentic design"} {
		id, _, _ := repo.Upsert(ctx, db.NewArticle{
			SourceID: s.ID, GUID: "g" + title, Title: title, Status: db.StatusUnread,
			Entities: []db.Entity{{Name: "Agentic", Type: "product"}},
		})
		_ = repo.ApplyClassification(ctx, id, "research", "s", nil, []db.Entity{{Name: "Agentic", Type: "product"}}, 3)
	}

	res, err := app.KBuild(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if res.AtomsCreated != 2 {
		t.Fatalf("atoms = %d", res.AtomsCreated)
	}
	if res.ElectronsCreated != 1 {
		t.Fatalf("electrons = %d (want 1: agentic)", res.ElectronsCreated)
	}

	notes, err := app.ListNotes(ctx, "atom")
	if err != nil {
		t.Fatal(err)
	}
	if len(notes) != 2 {
		t.Fatalf("notes = %d", len(notes))
	}

	// Graph neighbors of an atom: the shared electron + sibling atom.
	neighbors, err := app.GraphNeighbors(ctx, notes[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(neighbors) < 2 {
		t.Fatalf("neighbors = %d (%v)", len(neighbors), neighbors)
	}
}

func TestKSynthesizeViaAPI(t *testing.T) {
	app := newAppForKB(t)
	ctx := context.Background()
	s, _ := app.AddSource(ctx, "HN", "hackernews", "u", "")
	repo := db.NewArticleRepo(app.db)
	id, _, _ := repo.Upsert(ctx, db.NewArticle{SourceID: s.ID, GUID: "g", Title: "T", Status: db.StatusUnread})
	_ = repo.ApplyClassification(ctx, id, "models", "s", nil, nil, 2)

	if _, err := app.KBuild(ctx); err != nil {
		t.Fatal(err)
	}
	res, err := app.KSynthesize(ctx, "models")
	if err != nil {
		t.Fatal(err)
	}
	if res.MoleculesCreated != 1 {
		t.Fatalf("res = %+v", res)
	}
	note, err := app.GetNote(ctx, 1)
	if err != nil {
		t.Fatal(err)
	}
	_ = note
}
