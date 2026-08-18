package kb

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ajramos/giznews/internal/db"
)

func TestSlugify(t *testing.T) {
	cases := map[string]string{
		"Agentic Workflows":  "agentic-workflows",
		"LoRA: Fine-tuning!": "lora-fine-tuning",
		"  Trim  me  ":       "trim-me",
		"RAG (Retrieval)":    "rag-retrieval",
		"Café AI":            "caf-ai",
		"!!!!":               "note",
	}
	for in, want := range cases {
		if got := Slugify(in); got != want {
			t.Errorf("Slugify(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestBuildAtom(t *testing.T) {
	a := &db.Article{
		Title: "OpenAI ships GPT-5", SourceName: "HN", URL: "https://x.com/1",
		Importance: 3, Category: "models", Summary: "Big release.",
		Tags: []string{"gpt-5"}, Entities: []db.Entity{{Name: "OpenAI", Type: "org"}},
	}
	content := BuildAtom(a, []string{"openai"})

	if !strings.HasPrefix(content, "---\ntype: atom\n") {
		t.Fatalf("missing frontmatter:\n%s", content)
	}
	if !strings.Contains(content, "rating: ★★★☆☆") {
		t.Fatalf("missing rating:\n%s", content)
	}
	if !strings.Contains(content, "[[openai]]") {
		t.Fatalf("missing wikilink:\n%s", content)
	}
	if !strings.Contains(content, "**OpenAI** (org)") {
		t.Fatalf("missing entity:\n%s", content)
	}
}

func TestBuildElectron(t *testing.T) {
	content := BuildElectron("LoRA", []atomRef{{Slug: "atom-1", Title: "LoRA paper"}})
	if !strings.Contains(content, "type: electron") {
		t.Fatalf("missing type:\n%s", content)
	}
	if !strings.Contains(content, "[[atom-1]]") {
		t.Fatalf("missing backlink:\n%s", content)
	}
}

func TestBuildMolecule(t *testing.T) {
	content := BuildMolecule("Síntesis de models", "Models dominated.", []atomRef{{Slug: "a", Title: "T"}})
	if !strings.Contains(content, "🧪") || !strings.Contains(content, "Models dominated.") {
		t.Fatalf("molecule content:\n%s", content)
	}
	if !strings.Contains(content, "[[a]]") {
		t.Fatalf("missing link:\n%s", content)
	}
}

func TestBuildService(t *testing.T) {
	d, err := db.Open(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	ctx := context.Background()
	vaultRoot := filepath.Join(t.TempDir(), "vault")

	src, _ := db.NewSourceRepo(d).Create(ctx, db.NewSource{Name: "HN", Type: db.SourceHackerNews, URL: "u"})
	repo := db.NewArticleRepo(d)
	// Three articles, all importance >= 2, sharing a concept "rag".
	for _, title := range []string{"RAG at scale", "RAG for agents", "RAG memory limits"} {
		id, _, _ := repo.Upsert(ctx, db.NewArticle{
			SourceID: src.ID, GUID: "g" + title, Title: title, Status: db.StatusUnread,
			Importance: 2, Category: "research",
			Entities: []db.Entity{{Name: "RAG", Type: "concept"}, {Name: "OpenAI", Type: "org"}},
		})
		_ = repo.ApplyClassification(ctx, id, "research", "s", []string{"rag"}, []db.Entity{{Name: "RAG", Type: "concept"}}, 2)
	}
	// One low-importance article that should NOT become an atom.
	_, _, _ = repo.Upsert(ctx, db.NewArticle{SourceID: src.ID, GUID: "low", Title: "low", Status: db.StatusUnread, Importance: 0})

	svc, err := NewService(d, vaultRoot, Options{
		ImportanceThreshold: 2, MinOccurrences: 2, AgeDays: 30, UseLLM: false,
	}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	res, err := svc.Build(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if res.AtomsCreated != 3 {
		t.Fatalf("atoms = %d, want 3", res.AtomsCreated)
	}
	if res.ElectronsCreated != 1 {
		t.Fatalf("electrons = %d, want 1 (rag)", res.ElectronsCreated)
	}

	// RAG appears in 3 atoms → electron "rag" exists.
	kbRepo := db.NewKBRepo(d)
	elec, err := kbRepo.GetBySlug(ctx, "rag")
	if err != nil {
		t.Fatal(err)
	}
	if len(elec.Wikilinks) != 3 {
		t.Fatalf("electron wikilinks = %v, want 3", elec.Wikilinks)
	}
	// "openai" appears once → no electron.
	if _, err := kbRepo.GetBySlug(ctx, "openai"); err != db.ErrNotFound {
		t.Fatalf("expected no openai electron, got %v", err)
	}

	// Vault files exist.
	atomPath := filepath.Join(vaultRoot, "02-Atoms", "rag-at-scale.md")
	if _, err := os.Stat(atomPath); err != nil {
		t.Fatalf("atom file missing: %v", err)
	}
	elecPath := filepath.Join(vaultRoot, "01-Electrons", "rag.md")
	if _, err := os.Stat(elecPath); err != nil {
		t.Fatalf("electron file missing: %v", err)
	}

	// Idempotent: second build creates nothing new.
	res2, err := svc.Build(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if res2.AtomsCreated != 0 || res2.ElectronsCreated != 0 {
		t.Fatalf("res2 = %+v, want all zeros", res2)
	}
}

func TestGraphNeighbors(t *testing.T) {
	d, err := db.Open(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	ctx := context.Background()
	vaultRoot := filepath.Join(t.TempDir(), "vault")

	kbRepo := db.NewKBRepo(d)
	svc, _ := NewService(d, vaultRoot, Options{}, nil, nil)

	n1, _ := kbRepo.Create(ctx, db.NewKBNote{
		Type: db.NoteAtom, Title: "A1", Slug: "a1", Content: "x", Tags: []string{"t1"}, Wikilinks: []string{"e1"},
	})
	_, _ = kbRepo.Create(ctx, db.NewKBNote{
		Type: db.NoteElectron, Title: "E1", Slug: "e1", Content: "y", Tags: []string{"t1"}, Wikilinks: []string{"a1"},
	})
	_, _ = kbRepo.Create(ctx, db.NewKBNote{
		Type: db.NoteAtom, Title: "A2", Slug: "a2", Content: "z", Tags: []string{"t1"}, Wikilinks: []string{"e1"},
	})

	// a1 connects via outgoing (e1), incoming (e1→a1) and shared tag (a2).
	neighbors, err := svc.GraphNeighbors(ctx, n1.ID, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(neighbors) != 2 {
		t.Fatalf("neighbors = %d (%v)", len(neighbors), neighbors)
	}
	seen := map[string]bool{}
	for _, n := range neighbors {
		seen[n.Slug] = true
	}
	if !seen["e1"] || !seen["a2"] {
		t.Fatalf("expected neighbors e1 and a2, got %v", seen)
	}
}

func TestSynthesize(t *testing.T) {
	d, err := db.Open(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	ctx := context.Background()
	vaultRoot := filepath.Join(t.TempDir(), "vault")

	kbRepo := db.NewKBRepo(d)
	fm, _ := json.Marshal(map[string]any{"type": "atom", "category": "models"})
	_, _ = kbRepo.Create(ctx, db.NewKBNote{
		Type: db.NoteAtom, Title: "M1", Slug: "m1", Content: "x", Frontmatter: string(fm), Tags: []string{"atom"},
	})
	_, _ = kbRepo.Create(ctx, db.NewKBNote{
		Type: db.NoteAtom, Title: "M2", Slug: "m2", Content: "y", Frontmatter: string(fm), Tags: []string{"atom"},
	})

	svc, _ := NewService(d, vaultRoot, Options{UseLLM: false}, nil, nil)
	res, err := svc.Synthesize(ctx, "models")
	if err != nil {
		t.Fatal(err)
	}
	if res.MoleculesCreated != 1 {
		t.Fatalf("res = %+v", res)
	}
	// Slug is "sintesis-models".
	note, err := kbRepo.GetBySlug(ctx, "sintesis-models")
	if err != nil {
		t.Fatal(err)
	}
	if len(note.Wikilinks) != 2 {
		t.Fatalf("molecule links = %v, want 2", note.Wikilinks)
	}
}

// buildFixture wires a service over a fresh db + vault and returns both.
func buildFixture(t *testing.T) (*Service, *db.DB, string, int64) {
	t.Helper()
	d, err := db.Open(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { d.Close() })
	vaultRoot := filepath.Join(t.TempDir(), "vault")
	svc, err := NewService(d, vaultRoot, Options{
		ImportanceThreshold: 2, MinOccurrences: 2, AgeDays: 30, UseLLM: false,
	}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	src, err := db.NewSourceRepo(d).Create(context.Background(), db.NewSource{
		Name: "HN", Type: db.SourceHackerNews, URL: "u",
	})
	if err != nil {
		t.Fatal(err)
	}
	return svc, d, vaultRoot, src.ID
}

// classifiedArticle inserts an article already carrying a classification.
func classifiedArticle(t *testing.T, d *db.DB, srcID int64, title string, tags []string) int64 {
	t.Helper()
	ctx := context.Background()
	repo := db.NewArticleRepo(d)
	id, _, err := repo.Upsert(ctx, db.NewArticle{
		SourceID: srcID, GUID: title, Title: title, Status: db.StatusUnread,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.ApplyClassification(ctx, id, "models", "summary", tags, nil, 3); err != nil {
		t.Fatal(err)
	}
	return id
}

// An electron created in one run and extended in the next must have its vault
// file rewritten — Obsidian reads the file, not the database row.
func TestElectronFileFollowsUpdates(t *testing.T) {
	svc, d, vaultRoot, srcID := buildFixture(t)
	ctx := context.Background()

	classifiedArticle(t, d, srcID, "First rag piece", []string{"rag"})
	classifiedArticle(t, d, srcID, "Second rag piece", []string{"rag"})
	if _, err := svc.Build(ctx); err != nil {
		t.Fatal(err)
	}

	classifiedArticle(t, d, srcID, "Third rag piece", []string{"rag"})
	res, err := svc.Build(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if res.ElectronsUpdated != 1 {
		t.Fatalf("electrons updated = %d, want 1", res.ElectronsUpdated)
	}

	onDisk, err := os.ReadFile(filepath.Join(vaultRoot, "01-Electrons", "rag.md"))
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Count(string(onDisk), "[["); got != 3 {
		t.Fatalf("electron file has %d backlinks, want 3:\n%s", got, onDisk)
	}
	note, err := db.NewKBRepo(d).GetBySlug(ctx, "rag")
	if err != nil {
		t.Fatal(err)
	}
	if len(note.Wikilinks) != 3 {
		t.Fatalf("note wikilinks = %v, want 3", note.Wikilinks)
	}
}

// An article titled exactly like a concept must not lose its note to that
// concept's electron.
func TestConceptSlugNeverOverwritesAtom(t *testing.T) {
	svc, d, vaultRoot, srcID := buildFixture(t)
	ctx := context.Background()

	classifiedArticle(t, d, srcID, "Mamba", []string{"mamba"})
	classifiedArticle(t, d, srcID, "State space models are back", []string{"mamba"})
	if _, err := svc.Build(ctx); err != nil {
		t.Fatal(err)
	}

	repo := db.NewKBRepo(d)
	electron, err := repo.GetBySlug(ctx, "mamba")
	if err != nil {
		t.Fatalf("electron: %v", err)
	}
	if electron.Type != db.NoteElectron {
		t.Fatalf("slug mamba owned by %s, want electron", electron.Type)
	}

	atoms, err := repo.List(ctx, db.NoteAtom, 10)
	if err != nil {
		t.Fatal(err)
	}
	var titled *db.KBNote
	for _, n := range atoms {
		if n.Title == "Mamba" {
			titled = n
		}
	}
	if titled == nil {
		t.Fatal("atom for the article titled Mamba is gone")
	}
	if !strings.Contains(titled.Content, "type: atom") {
		t.Fatalf("atom content was overwritten:\n%s", titled.Content)
	}
	// Both notes exist as separate files, each in its own vault folder.
	if _, err := os.Stat(filepath.Join(vaultRoot, "01-Electrons", "mamba.md")); err != nil {
		t.Fatalf("electron file: %v", err)
	}
	if _, err := os.Stat(titled.Path); err != nil {
		t.Fatalf("atom file: %v", err)
	}
	// Its sibling links to the electron, not to the same-named atom.
	sibling, err := repo.GetBySlug(ctx, "state-space-models-are-back")
	if err != nil {
		t.Fatal(err)
	}
	if len(sibling.Wikilinks) != 1 || sibling.Wikilinks[0] != "mamba" {
		t.Fatalf("sibling links = %v, want [mamba]", sibling.Wikilinks)
	}
}

// ArticlesSkipped counts what did NOT become an atom.
func TestBuildResultSkippedCount(t *testing.T) {
	svc, d, _, srcID := buildFixture(t)
	ctx := context.Background()

	classifiedArticle(t, d, srcID, "One", nil)
	classifiedArticle(t, d, srcID, "Two", nil)
	res, err := svc.Build(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if res.AtomsCreated != 2 || res.ArticlesSkipped != 0 {
		t.Fatalf("res = %+v, want 2 atoms and 0 skipped", res)
	}
}
