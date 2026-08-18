package kb

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

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

// The promotion threshold counts a concept's whole history: one mention per run
// must graduate exactly like several mentions in a single run.
func TestConceptGraduatesAcrossRuns(t *testing.T) {
	svc, d, vaultRoot, srcID := buildFixture(t) // MinOccurrences = 2
	ctx := context.Background()
	conceptRepo := db.NewConceptRepo(d)

	classifiedArticle(t, d, srcID, "Mamba enters the arena", []string{"mamba"})
	res, err := svc.Build(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if res.ElectronsCreated != 0 {
		t.Fatalf("run 1 created %d electrons, want 0 (one mention)", res.ElectronsCreated)
	}
	if res.ConceptsTracked != 1 {
		t.Fatalf("run 1 tracked %d concepts, want 1", res.ConceptsTracked)
	}
	// The mention is remembered even though nothing was promoted.
	c, err := conceptRepo.Get(ctx, "mamba")
	if err != nil {
		t.Fatalf("concept after run 1: %v", err)
	}
	if c.Mentions != 1 || c.NoteID != 0 {
		t.Fatalf("concept = %+v, want 1 mention and no note", c)
	}
	dangling, err := conceptRepo.Dangling(ctx, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(dangling) != 1 || dangling[0].Slug != "mamba" {
		t.Fatalf("dangling = %+v, want mamba", dangling)
	}

	// A second run, a single new article: the second mention graduates it.
	classifiedArticle(t, d, srcID, "Mamba two years on", []string{"mamba"})
	res, err = svc.Build(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if res.ElectronsCreated != 1 {
		t.Fatalf("run 2 created %d electrons, want 1", res.ElectronsCreated)
	}

	c, err = conceptRepo.Get(ctx, "mamba")
	if err != nil {
		t.Fatal(err)
	}
	if c.Mentions != 2 || c.NoteID == 0 {
		t.Fatalf("concept = %+v, want 2 mentions and a promoted note", c)
	}
	// The electron lists both atoms, including the one from the earlier run.
	onDisk, err := os.ReadFile(filepath.Join(vaultRoot, "01-Electrons", "mamba.md"))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"[[mamba-enters-the-arena]]", "[[mamba-two-years-on]]"} {
		if !strings.Contains(string(onDisk), want) {
			t.Fatalf("electron missing %s:\n%s", want, onDisk)
		}
	}
	dangling, err = conceptRepo.Dangling(ctx, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(dangling) != 0 {
		t.Fatalf("dangling after promotion = %+v, want none", dangling)
	}
}

// Two atoms citing the same concept are neighbours even before that concept has
// a note of its own, and structural tags never expand the graph.
func TestGraphNeighborsUseConcepts(t *testing.T) {
	svc, d, _, srcID := buildFixture(t)
	ctx := context.Background()

	classifiedArticle(t, d, srcID, "Sparse attention at scale", []string{"sparse-attention"})
	classifiedArticle(t, d, srcID, "Attention is still all you need", []string{"sparse-attention"})
	// An unrelated article: same structural tags (atom, ai), no shared concept.
	classifiedArticle(t, d, srcID, "Chip supply update", []string{"hardware"})
	if _, err := svc.Build(ctx); err != nil {
		t.Fatal(err)
	}

	repo := db.NewKBRepo(d)
	first, err := repo.GetBySlug(ctx, "sparse-attention-at-scale")
	if err != nil {
		t.Fatal(err)
	}
	neighbors, err := svc.GraphNeighbors(ctx, first.ID, 10)
	if err != nil {
		t.Fatal(err)
	}
	titles := map[string]bool{}
	for _, n := range neighbors {
		titles[n.Title] = true
	}
	if !titles["Attention is still all you need"] {
		t.Fatalf("sibling citing the same concept is missing: %v", titles)
	}
	if titles["Chip supply update"] {
		t.Fatalf("unrelated note pulled in by structural tags: %v", titles)
	}
}

// Spellings of the same concept must not split its mentions in two.
func TestConceptSpellingsFold(t *testing.T) {
	svc, d, vaultRoot, srcID := buildFixture(t) // MinOccurrences = 2
	ctx := context.Background()

	classifiedArticle(t, d, srcID, "Open AI ships something", []string{"open-ai"})
	classifiedArticle(t, d, srcID, "OpenAI ships something else", []string{"openai"})
	res, err := svc.Build(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if res.ConceptsTracked != 1 {
		t.Fatalf("tracked %d concepts, want 1 (open-ai and openai are one)", res.ConceptsTracked)
	}
	if res.ElectronsCreated != 1 {
		t.Fatalf("created %d electrons, want 1", res.ElectronsCreated)
	}

	conceptRepo := db.NewConceptRepo(d)
	c, err := conceptRepo.Get(ctx, "open-ai")
	if err != nil {
		t.Fatalf("concept: %v", err)
	}
	if c.Mentions != 2 {
		t.Fatalf("mentions = %d, want 2", c.Mentions)
	}
	// Both atoms link to the same electron.
	repo := db.NewKBRepo(d)
	for _, slug := range []string{"open-ai-ships-something", "openai-ships-something-else"} {
		n, err := repo.GetBySlug(ctx, slug)
		if err != nil {
			t.Fatal(err)
		}
		if len(n.Wikilinks) != 1 || n.Wikilinks[0] != "open-ai" {
			t.Fatalf("%s links = %v, want [open-ai]", slug, n.Wikilinks)
		}
	}
	if _, err := os.Stat(filepath.Join(vaultRoot, "01-Electrons", "openai.md")); err == nil {
		t.Fatal("a second electron was written for the same concept")
	}
}

// Merging moves mentions and rewrites the notes that pointed at the old slug,
// in the database and on disk.
func TestMergeConcepts(t *testing.T) {
	svc, d, vaultRoot, srcID := buildFixture(t)
	ctx := context.Background()

	classifiedArticle(t, d, srcID, "Retrieval augmented generation explained", []string{"retrieval-augmented-generation"})
	classifiedArticle(t, d, srcID, "RAG in production", []string{"rag"})
	classifiedArticle(t, d, srcID, "RAG at the edge", []string{"rag"})
	if _, err := svc.Build(ctx); err != nil {
		t.Fatal(err)
	}

	res, err := svc.MergeConcepts(ctx, "retrieval-augmented-generation", "rag")
	if err != nil {
		t.Fatal(err)
	}
	if res.NotesRelinked != 1 {
		t.Fatalf("relinked %d notes, want 1", res.NotesRelinked)
	}
	if res.Mentions != 3 {
		t.Fatalf("mentions = %d, want 3", res.Mentions)
	}

	conceptRepo := db.NewConceptRepo(d)
	if _, err := conceptRepo.Get(ctx, "retrieval-augmented-generation"); !errors.Is(err, db.ErrNotFound) {
		t.Fatalf("merged concept still exists: %v", err)
	}
	// The old spelling now resolves to the surviving concept.
	got, err := conceptRepo.Resolve(ctx, "retrieval-augmented-generation")
	if err != nil {
		t.Fatal(err)
	}
	if got != "rag" {
		t.Fatalf("resolve = %q, want rag", got)
	}

	// The relinked atom points at the survivor, in the row and in the file.
	repo := db.NewKBRepo(d)
	note, err := repo.GetBySlug(ctx, "retrieval-augmented-generation-explained")
	if err != nil {
		t.Fatal(err)
	}
	if len(note.Wikilinks) != 1 || note.Wikilinks[0] != "rag" {
		t.Fatalf("links = %v, want [rag]", note.Wikilinks)
	}
	onDisk, err := os.ReadFile(note.Path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(onDisk), "[[retrieval-augmented-generation]]") {
		t.Fatalf("atom file still links to the merged slug:\n%s", onDisk)
	}
	// And the electron lists all three atoms.
	electron, err := os.ReadFile(filepath.Join(vaultRoot, "01-Electrons", "rag.md"))
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Count(string(electron), "[["); got != 3 {
		t.Fatalf("electron has %d backlinks, want 3:\n%s", got, electron)
	}
}

// Every build leaves the vault with a way in.
func TestBuildWritesVaultIndex(t *testing.T) {
	svc, d, vaultRoot, srcID := buildFixture(t)
	ctx := context.Background()

	classifiedArticle(t, d, srcID, "Agents everywhere", []string{"agents"})
	classifiedArticle(t, d, srcID, "Agents at work", []string{"agents"})
	classifiedArticle(t, d, srcID, "A lonely topic", []string{"quantum"})
	if _, err := svc.Build(ctx); err != nil {
		t.Fatal(err)
	}

	index, err := os.ReadFile(filepath.Join(vaultRoot, "Index.md"))
	if err != nil {
		t.Fatalf("index: %v", err)
	}
	for _, want := range []string{"# AI knowledge index", "3 atoms", "[[agents]]", "## By category", "[[agents-everywhere]]"} {
		if !strings.Contains(string(index), want) {
			t.Fatalf("index missing %q:\n%s", want, index)
		}
	}

	// "quantum" has one mention: it is listed as pending, not as an electron.
	unresolved, err := os.ReadFile(filepath.Join(vaultRoot, "Unresolved concepts.md"))
	if err != nil {
		t.Fatalf("unresolved: %v", err)
	}
	if !strings.Contains(string(unresolved), "[[quantum]]") {
		t.Fatalf("unresolved is missing quantum:\n%s", unresolved)
	}
	if strings.Contains(string(unresolved), "[[agents]]") {
		t.Fatalf("promoted concept listed as unresolved:\n%s", unresolved)
	}

	day := time.Now().UTC().Format("2006-01-02")
	daily, err := os.ReadFile(filepath.Join(vaultRoot, "00-Inbox", day+".md"))
	if err != nil {
		t.Fatalf("daily note: %v", err)
	}
	if !strings.Contains(string(daily), "## Notes added today") || !strings.Contains(string(daily), "[[agents-everywhere]]") {
		t.Fatalf("daily note:\n%s", daily)
	}
	// Atoms and the concepts they promoted are listed apart.
	if !strings.Contains(string(daily), "## Concepts that earned a note") || !strings.Contains(string(daily), "[[agents]]") {
		t.Fatalf("daily note is missing the concepts section:\n%s", daily)
	}

	// The generated views must stay out of the graph.
	notes, err := db.NewKBRepo(d).List(ctx, "", 100)
	if err != nil {
		t.Fatal(err)
	}
	for _, n := range notes {
		if n.Slug == "Index" || n.Slug == "Unresolved concepts" || n.Slug == day {
			t.Fatalf("generated view %q was stored as a note", n.Slug)
		}
	}
}
