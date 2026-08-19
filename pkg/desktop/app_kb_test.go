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

func TestToNoteDTOFrontmatter(t *testing.T) {
	n := &db.KBNote{
		ID: 1, Type: db.NoteAtom, Title: "T", Slug: "t",
		Frontmatter: `{"type":"atom","category":"models","source":"HN RSS","url":"https://x.com","rating":3}`,
	}
	dto := toNoteDTO(n)
	if dto.Category != "models" || dto.Source != "HN RSS" || dto.URL != "https://x.com" || dto.Rating != 3 {
		t.Fatalf("dto = %+v", dto)
	}
	// electron frontmatter (no category/rating) leaves those fields empty.
	e := toNoteDTO(&db.KBNote{ID: 2, Type: db.NoteElectron, Title: "E", Frontmatter: `{"type":"electron","name":"E"}`})
	if e.Category != "" || e.Rating != 0 || e.URL != "" {
		t.Fatalf("electron dto = %+v", e)
	}
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

// The promotion queue is actionable: a concept below the threshold can be given
// its note on demand, and folding two concepts rewrites the notes involved.
func TestConceptsViaAPI(t *testing.T) {
	app := newAppForKB(t)
	ctx := context.Background()

	s, _ := app.AddSource(ctx, "HN", "hackernews", "u", "")
	repo := db.NewArticleRepo(app.db)
	for _, a := range []struct {
		title string
		tags  []string
	}{
		{"Sparse attention at scale", []string{"sparse-attention"}},
		{"Long context, cheaply", []string{"long-context"}},
	} {
		id, _, _ := repo.Upsert(ctx, db.NewArticle{
			SourceID: s.ID, GUID: a.title, Title: a.title, Status: db.StatusUnread,
		})
		_ = repo.ApplyClassification(ctx, id, "research", "sum", a.tags, nil, 3)
	}
	if _, err := app.KBuild(ctx); err != nil {
		t.Fatal(err)
	}

	concepts, err := app.ListConcepts(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(concepts) != 2 {
		t.Fatalf("concepts = %d, want 2", len(concepts))
	}
	for _, c := range concepts {
		if c.Promoted {
			t.Fatalf("%s promoted with %d mention(s); the threshold is 2", c.Slug, c.Mentions)
		}
	}

	// Promote one by hand.
	note, err := app.PromoteConcept(ctx, "sparse-attention")
	if err != nil {
		t.Fatal(err)
	}
	if note.Type != "electron" || note.Title != "Sparse Attention" {
		t.Fatalf("note = %+v", note)
	}
	concepts, _ = app.ListConcepts(ctx)
	for _, c := range concepts {
		if c.Slug == "sparse-attention" && (!c.Promoted || c.NoteID == 0) {
			t.Fatalf("concept not marked as promoted: %+v", c)
		}
	}

	// Fold the other one into it.
	merged, err := app.MergeConcepts(ctx, "long-context", "sparse-attention")
	if err != nil {
		t.Fatal(err)
	}
	if merged.NotesRelinked != 1 || merged.Mentions != 2 {
		t.Fatalf("merge = %+v, want 1 relinked and 2 mentions", merged)
	}
	concepts, _ = app.ListConcepts(ctx)
	if len(concepts) != 1 {
		t.Fatalf("concepts after merge = %d, want 1", len(concepts))
	}
}
