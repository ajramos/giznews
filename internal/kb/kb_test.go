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
