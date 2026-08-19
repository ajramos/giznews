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
	"gopkg.in/yaml.v3"
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

func TestDisplayName(t *testing.T) {
	cases := map[string]string{
		"rag":                "RAG",
		"llm":                "LLM",
		"lora":               "LoRA",
		"openai":             "OpenAI",
		"open-ai":            "OpenAI",
		"hugging-face":       "Hugging Face",
		"gpt-5":              "GPT-5",
		"llama-3":            "Llama-3",
		"state-space-models": "State Space Models",
		"agents":             "Agents",
		"ai-regulation":      "AI Regulation",
		"OpenAI":             "OpenAI", // already cased by the entity extractor
		"Mixture of Experts": "Mixture of Experts",
		"":                   "",
	}
	for in, want := range cases {
		if got := DisplayName(in); got != want {
			t.Errorf("DisplayName(%q) = %q, want %q", in, got, want)
		}
	}
}

// A concept derived from a lowercase tag must still read properly everywhere it
// is shown: the concept row, the electron note and the vault index.
func TestConceptNamesAreReadable(t *testing.T) {
	svc, d, vaultRoot, srcID := buildFixture(t)
	ctx := context.Background()

	classifiedArticle(t, d, srcID, "RAG in production", []string{"rag"})
	classifiedArticle(t, d, srcID, "RAG at the edge", []string{"rag"})
	if _, err := svc.Build(ctx); err != nil {
		t.Fatal(err)
	}

	c, err := db.NewConceptRepo(d).Get(ctx, "rag")
	if err != nil {
		t.Fatal(err)
	}
	if c.Name != "RAG" {
		t.Fatalf("concept name = %q, want RAG", c.Name)
	}
	note, err := db.NewKBRepo(d).GetBySlug(ctx, "rag")
	if err != nil {
		t.Fatal(err)
	}
	if note.Title != "RAG" {
		t.Fatalf("electron title = %q, want RAG", note.Title)
	}
	onDisk, err := os.ReadFile(filepath.Join(vaultRoot, "01-Electrons", "rag.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(onDisk), "# RAG") {
		t.Fatalf("electron heading:\n%s", onDisk)
	}
	index, err := os.ReadFile(filepath.Join(vaultRoot, "Index.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(index), "[[rag]] — RAG") {
		t.Fatalf("index entry:\n%s", index)
	}
	// The slug — the file name and the wikilink — stays lowercase.
	if note.Slug != "rag" {
		t.Fatalf("slug = %q, want rag", note.Slug)
	}
}

// Concepts stored before display names existed are repaired on the next build,
// note included.
func TestRepairsRawConceptNames(t *testing.T) {
	svc, d, vaultRoot, srcID := buildFixture(t)
	ctx := context.Background()
	conceptRepo := db.NewConceptRepo(d)
	kbRepo := db.NewKBRepo(d)

	classifiedArticle(t, d, srcID, "Serving models cheaply", []string{"vllm"})
	classifiedArticle(t, d, srcID, "Serving models fast", []string{"vllm"})
	if _, err := svc.Build(ctx); err != nil {
		t.Fatal(err)
	}

	// Rewind to what an older build would have stored: the slug as the name.
	if err := conceptRepo.Rename(ctx, "vllm", "vllm"); err != nil {
		t.Fatal(err)
	}
	note, err := kbRepo.GetBySlug(ctx, "vllm")
	if err != nil {
		t.Fatal(err)
	}
	note.Title = "vllm"
	note.Content = strings.ReplaceAll(note.Content, "# vLLM", "# vllm")
	if err := kbRepo.Update(ctx, note); err != nil {
		t.Fatal(err)
	}
	// A note written by a giznews that had neither markers nor hashes.
	if err := kbRepo.SetContentHash(ctx, note.ID, ""); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(note.Path, []byte(note.Content), 0o644); err != nil {
		t.Fatal(err)
	}

	// A build with nothing new to ingest still repairs what it finds.
	if _, err := svc.Build(ctx); err != nil {
		t.Fatal(err)
	}

	c, err := conceptRepo.Get(ctx, "vllm")
	if err != nil {
		t.Fatal(err)
	}
	if c.Name != "vLLM" {
		t.Fatalf("concept name = %q, want vLLM", c.Name)
	}
	onDisk, err := os.ReadFile(filepath.Join(vaultRoot, "01-Electrons", "vllm.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(onDisk), "# vLLM") {
		t.Fatalf("electron was not rewritten:\n%s", onDisk)
	}
}

func TestYAMLValueQuoting(t *testing.T) {
	cases := map[string]string{
		"models":           "models",
		"HN RSS":           "HN RSS",
		"★★★☆☆":            "★★★☆☆",
		"2026-08-18 07:30": `"2026-08-18 07:30"`, // the time separator is a colon
		"Ars Technica: AI": `"Ars Technica: AI"`,
		"https://x.com/a":  `"https://x.com/a"`,
		"#hashtag":         `"#hashtag"`,
		`he said "hi"`:     `"he said \"hi\""`,
		"- leading dash":   `"- leading dash"`,
		"":                 `""`,
		"two\nlines":       "two lines",
	}
	for in, want := range cases {
		if got := yamlValue(in); got != want {
			t.Errorf("yamlValue(%q) = %s, want %s", in, got, want)
		}
	}
}

// The frontmatter of a note whose fields carry colons, quotes and hashes must
// still parse as YAML — Obsidian reads it, and a broken block loses every field.
func TestAtomFrontmatterParses(t *testing.T) {
	a := &db.Article{
		Title:      "Anthropic: what it means",
		SourceName: "Ars Technica: AI",
		URL:        "https://arstechnica.com/a?b=1#c",
		Importance: 3,
		Category:   "industry",
		Summary:    "Something happened.",
		Tags:       []string{"anthropic", "#policy", "cost: high"},
		Published:  "2026-03-04T07:30:00Z",
	}
	content := BuildAtom(a, []string{"anthropic"})

	block, _, found := strings.Cut(strings.TrimPrefix(content, "---\n"), "\n---\n")
	if !found {
		t.Fatalf("no frontmatter block:\n%s", content)
	}
	var fm struct {
		Type     string   `yaml:"type"`
		Created  string   `yaml:"created"`
		Source   string   `yaml:"source"`
		URL      string   `yaml:"url"`
		Category string   `yaml:"category"`
		Tags     []string `yaml:"tags"`
	}
	if err := yaml.Unmarshal([]byte(block), &fm); err != nil {
		t.Fatalf("frontmatter does not parse: %v\n%s", err, block)
	}
	if fm.Source != "Ars Technica: AI" {
		t.Errorf("source = %q", fm.Source)
	}
	if fm.URL != "https://arstechnica.com/a?b=1#c" {
		t.Errorf("url = %q", fm.URL)
	}
	if len(fm.Tags) != 5 || fm.Tags[2] != "anthropic" || fm.Tags[3] != "#policy" || fm.Tags[4] != "cost: high" {
		t.Errorf("tags = %q", fm.Tags)
	}
	// The note is dated by the article, not by the moment of the build.
	if fm.Created != "2026-03-04 07:30" {
		t.Errorf("created = %q, want the publication date", fm.Created)
	}
}

// Without a publication date the fetch time stands in, and only then the clock.
func TestAtomDateFallsBackToFetch(t *testing.T) {
	a := &db.Article{Title: "T", URL: "u", FetchedAt: "2026-05-06T12:00:00Z"}
	if got := BuildAtom(a, nil); !strings.Contains(got, "created: \"2026-05-06 12:00\"") {
		t.Fatalf("created not taken from fetched_at:\n%s", got)
	}
	bare := BuildAtom(&db.Article{Title: "T", URL: "u"}, nil)
	if !strings.Contains(bare, "created: ") {
		t.Fatalf("missing created:\n%s", bare)
	}
}

// A paragraph typed in Obsidian must survive the article being re-classified.
func TestUserEditsSurviveRefresh(t *testing.T) {
	svc, d, _, srcID := buildFixture(t)
	ctx := context.Background()
	repo := db.NewArticleRepo(d)
	kbRepo := db.NewKBRepo(d)

	id := classifiedArticle(t, d, srcID, "Agents in production", []string{"agents"})
	if _, err := svc.Build(ctx); err != nil {
		t.Fatal(err)
	}
	note, err := kbRepo.GetBySlug(ctx, "agents-in-production")
	if err != nil {
		t.Fatal(err)
	}
	if note.ContentHash == "" {
		t.Fatal("no content hash recorded for the note")
	}

	// The reader adds their own thinking below the generated region, and a tag.
	onDisk, err := os.ReadFile(note.Path)
	if err != nil {
		t.Fatal(err)
	}
	edited := strings.Replace(string(onDisk), "tags:\n", "tags:\n  - my-own-tag\n", 1)
	edited += "\n## My take\nThis matters because of X.\n"
	if err := os.WriteFile(note.Path, []byte(edited), 0o644); err != nil {
		t.Fatal(err)
	}

	// The article is re-classified: a new summary, a new importance.
	if err := repo.ApplyClassification(ctx, id, "tools", "A much better summary.", []string{"agents", "orchestration"}, nil, 3); err != nil {
		t.Fatal(err)
	}
	res, err := svc.Build(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if res.AtomsRefreshed != 1 {
		t.Fatalf("refreshed %d atoms, want 1", res.AtomsRefreshed)
	}
	if res.EditedNotesKept != 1 {
		t.Fatalf("edited notes kept = %d, want 1", res.EditedNotesKept)
	}

	after, err := os.ReadFile(note.Path)
	if err != nil {
		t.Fatal(err)
	}
	got := string(after)
	// The user's prose and tag are still there...
	if !strings.Contains(got, "## My take\nThis matters because of X.") {
		t.Fatalf("user text was lost:\n%s", got)
	}
	if !strings.Contains(got, "my-own-tag") {
		t.Fatalf("user tag was lost:\n%s", got)
	}
	// ...and the regenerated part caught up with the article.
	if !strings.Contains(got, "A much better summary.") {
		t.Fatalf("generated region was not refreshed:\n%s", got)
	}
	if !strings.Contains(got, "orchestration") {
		t.Fatalf("new tag missing from frontmatter:\n%s", got)
	}
	// The frontmatter still parses after the merge.
	block, _, found := strings.Cut(strings.TrimPrefix(got, "---\n"), "\n---\n")
	if !found {
		t.Fatalf("no frontmatter after merge:\n%s", got)
	}
	var fm map[string]any
	if err := yaml.Unmarshal([]byte(block), &fm); err != nil {
		t.Fatalf("merged frontmatter does not parse: %v\n%s", err, block)
	}
}

// A note the user rewrote so thoroughly that the generated region is gone is
// never overwritten: giznews has nowhere safe to write, so it writes nothing.
func TestNoteWithoutRegionIsLeftAlone(t *testing.T) {
	svc, d, _, srcID := buildFixture(t)
	ctx := context.Background()
	repo := db.NewArticleRepo(d)
	kbRepo := db.NewKBRepo(d)

	id := classifiedArticle(t, d, srcID, "A note I rewrote", []string{"rewrites"})
	if _, err := svc.Build(ctx); err != nil {
		t.Fatal(err)
	}
	note, err := kbRepo.GetBySlug(ctx, "a-note-i-rewrote")
	if err != nil {
		t.Fatal(err)
	}
	mine := "---\ntype: atom\n---\n\n# Entirely mine now\n"
	if err := os.WriteFile(note.Path, []byte(mine), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := repo.ApplyClassification(ctx, id, "tools", "New summary.", nil, nil, 3); err != nil {
		t.Fatal(err)
	}
	res, err := svc.Build(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if res.EditedNotesKept != 1 {
		t.Fatalf("edited notes kept = %d, want 1", res.EditedNotesKept)
	}
	after, err := os.ReadFile(note.Path)
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != mine {
		t.Fatalf("the user's file was modified:\n%s", after)
	}
}

// An untouched note is replaced whole, and nothing is rewritten when the
// article has not moved.
func TestUntouchedNoteIsReplacedAndIdle(t *testing.T) {
	svc, d, _, srcID := buildFixture(t)
	ctx := context.Background()
	repo := db.NewArticleRepo(d)

	id := classifiedArticle(t, d, srcID, "Steady news", nil)
	if _, err := svc.Build(ctx); err != nil {
		t.Fatal(err)
	}

	// A second build with nothing new must not rewrite anything.
	res, err := svc.Build(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if res.AtomsRefreshed != 0 || res.EditedNotesKept != 0 {
		t.Fatalf("idle build did work: %+v", res)
	}

	// Re-classify: the note is replaced whole, markers and all.
	if err := repo.ApplyClassification(ctx, id, "models", "Fresh summary.", nil, nil, 3); err != nil {
		t.Fatal(err)
	}
	res, err = svc.Build(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if res.AtomsRefreshed != 1 || res.EditedNotesKept != 0 {
		t.Fatalf("res = %+v, want 1 refreshed and no edits found", res)
	}
	note, err := db.NewKBRepo(d).GetBySlug(ctx, "steady-news")
	if err != nil {
		t.Fatal(err)
	}
	onDisk, err := os.ReadFile(note.Path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(onDisk), "Fresh summary.") {
		t.Fatalf("note not refreshed:\n%s", onDisk)
	}
	if !strings.Contains(string(onDisk), markerBegin) {
		t.Fatalf("generated region markers missing:\n%s", onDisk)
	}
}

// A note written by hand in the vault joins the graph: it becomes searchable,
// it links like any other note, and its links count as concept mentions.
func TestSyncVaultImportsHandWrittenNotes(t *testing.T) {
	svc, d, vaultRoot, srcID := buildFixture(t) // MinOccurrences = 2
	ctx := context.Background()

	classifiedArticle(t, d, srcID, "Agents in production", []string{"agents"})
	if _, err := svc.Build(ctx); err != nil {
		t.Fatal(err)
	}
	conceptRepo := db.NewConceptRepo(d)
	before, err := conceptRepo.Get(ctx, "agents")
	if err != nil {
		t.Fatal(err)
	}
	if before.Mentions != 1 || before.NoteID != 0 {
		t.Fatalf("concept = %+v, want 1 mention and no note", before)
	}

	// The reader writes their own note, citing the same concept.
	mine := filepath.Join(vaultRoot, "00-Inbox", "my-thinking.md")
	body := "---\ntags:\n  - fieldwork\n  - \"cost: high\"\n---\n\n" +
		"# What I actually think about agents\n\n" +
		"We run three of these. See [[agents]] and [[agents-in-production]].\n"
	if err := os.WriteFile(mine, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}

	res, err := svc.SyncVault(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if res.Imported != 1 {
		t.Fatalf("imported %d notes, want 1", res.Imported)
	}
	if res.Mentions != 1 {
		t.Fatalf("recorded %d mentions, want 1 (agents exists, the atom link is not a concept)", res.Mentions)
	}

	note, err := db.NewKBRepo(d).GetBySlug(ctx, "my-thinking")
	if err != nil {
		t.Fatalf("note not imported: %v", err)
	}
	if note.Type != db.NoteInbox {
		t.Fatalf("type = %s, want inbox", note.Type)
	}
	if note.Title != "What I actually think about agents" {
		t.Fatalf("title = %q", note.Title)
	}
	if len(note.Tags) != 2 || note.Tags[0] != "fieldwork" || note.Tags[1] != "cost: high" {
		t.Fatalf("tags = %q", note.Tags)
	}
	if len(note.Wikilinks) != 2 {
		t.Fatalf("links = %q", note.Wikilinks)
	}

	// The user's mention counts towards promotion like any other.
	after, err := conceptRepo.Get(ctx, "agents")
	if err != nil {
		t.Fatal(err)
	}
	if after.Mentions != 2 {
		t.Fatalf("mentions = %d, want 2", after.Mentions)
	}
	if _, err := svc.Build(ctx); err != nil {
		t.Fatal(err)
	}
	promoted, err := conceptRepo.Get(ctx, "agents")
	if err != nil {
		t.Fatal(err)
	}
	if promoted.NoteID == 0 {
		t.Fatal("the concept did not graduate even though the reader's note pushed it over the threshold")
	}

	// Re-scanning an unchanged vault does nothing.
	again, err := svc.SyncVault(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if again.Imported != 0 || again.Updated != 0 {
		t.Fatalf("second scan = %+v, want no work", again)
	}

	// Editing the file updates the row, and never the file.
	edited := body + "\n## Later\nStill true.\n"
	if err := os.WriteFile(mine, []byte(edited), 0o644); err != nil {
		t.Fatal(err)
	}
	third, err := svc.SyncVault(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if third.Updated != 1 {
		t.Fatalf("third scan = %+v, want 1 updated", third)
	}
	onDisk, err := os.ReadFile(mine)
	if err != nil {
		t.Fatal(err)
	}
	if string(onDisk) != edited {
		t.Fatalf("the reader's file was rewritten:\n%s", onDisk)
	}
}

// The scan must not touch what giznews writes: neither its notes nor the
// generated views, even after a build has left them all in the vault.
func TestSyncVaultIgnoresGeneratedFiles(t *testing.T) {
	svc, d, _, srcID := buildFixture(t)
	ctx := context.Background()

	classifiedArticle(t, d, srcID, "One", []string{"agents"})
	classifiedArticle(t, d, srcID, "Two", []string{"agents"})
	if _, err := svc.Build(ctx); err != nil {
		t.Fatal(err)
	}
	countBefore, err := db.NewKBRepo(d).Count(ctx, "")
	if err != nil {
		t.Fatal(err)
	}

	res, err := svc.SyncVault(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if res.Imported != 0 || res.Updated != 0 {
		t.Fatalf("scan adopted generated files: %+v", res)
	}
	countAfter, err := db.NewKBRepo(d).Count(ctx, "")
	if err != nil {
		t.Fatal(err)
	}
	if countAfter != countBefore {
		t.Fatalf("note count went from %d to %d", countBefore, countAfter)
	}
}

// A note may cite a concept that does not exist yet. Counting mentions only
// when the file changes would drop that one forever, since the file never
// changes again.
func TestUserMentionCountsOnceTheConceptExists(t *testing.T) {
	svc, d, vaultRoot, srcID := buildFixture(t) // MinOccurrences = 2
	ctx := context.Background()
	conceptRepo := db.NewConceptRepo(d)

	// The reader writes about something giznews has never seen.
	mine := filepath.Join(vaultRoot, "00-Inbox", "early-thoughts.md")
	if err := os.MkdirAll(filepath.Dir(mine), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(mine, []byte("# Early thoughts\n\nOn [[mamba]].\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.SyncVault(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := conceptRepo.Get(ctx, "mamba"); !errors.Is(err, db.ErrNotFound) {
		t.Fatalf("a link invented a concept: %v", err)
	}

	// The news catches up: one article names it, so the concept now exists.
	classifiedArticle(t, d, srcID, "Mamba enters the arena", []string{"mamba"})
	if _, err := svc.Build(ctx); err != nil {
		t.Fatal(err)
	}

	// The reader's note has not changed, but its mention must now count — and
	// with two, the concept graduates.
	c, err := conceptRepo.Get(ctx, "mamba")
	if err != nil {
		t.Fatal(err)
	}
	if c.Mentions != 2 {
		t.Fatalf("mentions = %d, want 2 (the article and the reader's note)", c.Mentions)
	}
	if _, err := svc.Build(ctx); err != nil {
		t.Fatal(err)
	}
	promoted, err := conceptRepo.Get(ctx, "mamba")
	if err != nil {
		t.Fatal(err)
	}
	if promoted.NoteID == 0 {
		t.Fatal("concept did not graduate with two mentions")
	}
}

// A dry run has to predict the build that follows it, or it is worse than no
// dry run at all.
func TestPreviewMatchesTheBuildThatFollows(t *testing.T) {
	svc, d, vaultRoot, srcID := buildFixture(t)
	ctx := context.Background()

	classifiedArticle(t, d, srcID, "Agents in production", []string{"agents"})
	classifiedArticle(t, d, srcID, "Agents everywhere", []string{"agents"})
	classifiedArticle(t, d, srcID, "A quiet paper on retrieval", []string{"rag"})
	mine := filepath.Join(vaultRoot, "00-Inbox", "mine.md")
	if err := os.MkdirAll(filepath.Dir(mine), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(mine, []byte("# Mine\n\nOn [[rag]].\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	plan, err := svc.Preview(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Atoms) != 3 {
		t.Fatalf("plan has %d atoms, want 3", len(plan.Atoms))
	}
	// Both agents articles must plan the same concept slug: reserving per
	// article would give the second one a suffixed slug of its own.
	if got := plan.Atoms[0].Concepts; len(got) != 1 || got[0] != "agents" {
		t.Fatalf("first atom concepts = %v", got)
	}
	if got := plan.Atoms[1].Concepts; len(got) != 1 || got[0] != "agents" {
		t.Fatalf("second atom concepts = %v, want the same slug as the first", got)
	}
	// Both concepts reach two: "agents" from its two articles, and "rag" from
	// one article plus the reader's note — which the build counts once the
	// article has created the concept.
	if len(plan.Promoting) != 2 {
		t.Fatalf("promoting = %+v, want agents and rag", plan.Promoting)
	}
	for _, c := range plan.Promoting {
		if c.After != 2 {
			t.Fatalf("%s lands at %d, want 2", c.Slug, c.After)
		}
	}
	if plan.VaultNew != 1 {
		t.Fatalf("vault new = %d, want 1", plan.VaultNew)
	}

	// Nothing was written by the preview.
	if n, err := db.NewKBRepo(d).Count(ctx, ""); err != nil {
		t.Fatal(err)
	} else if n != 0 {
		t.Fatalf("preview wrote %d notes", n)
	}

	res, err := svc.Build(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if res.AtomsCreated != len(plan.Atoms) {
		t.Fatalf("build wrote %d atoms, the plan said %d", res.AtomsCreated, len(plan.Atoms))
	}
	if res.ElectronsCreated != len(plan.Promoting) {
		t.Fatalf("build promoted %d concepts, the plan said %d", res.ElectronsCreated, len(plan.Promoting))
	}
	if res.NotesImported != plan.VaultNew+plan.VaultEdits {
		t.Fatalf("build imported %d notes, the plan said %d", res.NotesImported, plan.VaultNew+plan.VaultEdits)
	}
	for _, a := range plan.Atoms {
		if _, err := db.NewKBRepo(d).GetBySlug(ctx, a.Slug); err != nil {
			t.Fatalf("planned slug %q was not the one written: %v", a.Slug, err)
		}
	}
}
