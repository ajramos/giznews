package kb

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sort"
	"strings"

	"github.com/ajramos/giznews/internal/db"
	"github.com/ajramos/giznews/internal/llm"
)

// Options configures knowledge-graph building.
type Options struct {
	ImportanceThreshold int // min importance for an article to become an atom
	AgeDays             int // only articles fetched within this window
	Limit               int // max atoms per run
	MinOccurrences      int // min atoms citing a concept before an electron is created
	Model               string
	UseLLM              bool
}

// BuildResult reports what a build run did.
type BuildResult struct {
	AtomsCreated     int `json:"atoms_created"`
	ElectronsCreated int `json:"electrons_created"`
	ElectronsUpdated int `json:"electrons_updated"`
	MoleculesCreated int `json:"molecules_created"`
	ArticlesSkipped  int `json:"articles_skipped"`
}

// Service maintains the knowledge graph.
type Service struct {
	db     *db.DB
	vault  *Vault
	opts   Options
	prov   llm.Provider
	logger *log.Logger
}

// NewService builds the knowledge-graph service.
func NewService(database *db.DB, vaultRoot string, opts Options, prov llm.Provider, logger *log.Logger) (*Service, error) {
	vault, err := NewVault(vaultRoot)
	if err != nil {
		return nil, err
	}
	if opts.ImportanceThreshold <= 0 {
		opts.ImportanceThreshold = 2
	}
	if opts.MinOccurrences <= 0 {
		opts.MinOccurrences = 2
	}
	if opts.AgeDays <= 0 {
		opts.AgeDays = 30
	}
	if opts.Limit <= 0 {
		opts.Limit = 200
	}
	return &Service{db: database, vault: vault, opts: opts, prov: prov, logger: logger}, nil
}

type conceptAgg struct {
	Name string
	Refs []atomRef
}

// Build ingests pending articles into the graph as Atom notes and creates or
// updates Electron notes for recurring concepts.
func (s *Service) Build(ctx context.Context) (*BuildResult, error) {
	res := &BuildResult{}
	artRepo := db.NewArticleRepo(s.db)
	kbRepo := db.NewKBRepo(s.db)
	ingestRepo := db.NewIngestRepo(s.db)

	articles, err := artRepo.ListForKB(ctx, s.opts.ImportanceThreshold, s.opts.AgeDays, s.opts.Limit)
	if err != nil {
		return nil, fmt.Errorf("kb: list articles: %w", err)
	}
	res.ArticlesSkipped = len(articles)

	concepts := map[string]*conceptAgg{}

	for _, a := range articles {
		slugs := s.conceptSlugs(a)
		content := BuildAtom(a, slugs)
		slug := s.uniqueSlug(ctx, kbRepo, Slugify(stripBrackets(a.Title)))

		path, err := s.vault.Write("atom", slug, content)
		if err != nil {
			return nil, err
		}
		fm, _ := json.Marshal(map[string]any{
			"type": "atom", "category": a.Category, "source": a.SourceName,
			"url": a.URL, "rating": a.Importance, "tags": a.Tags,
		})
		note, err := kbRepo.Create(ctx, db.NewKBNote{
			Type: db.NoteAtom, Title: a.Title, Slug: slug, Path: path,
			Frontmatter: string(fm), Content: content,
			Tags: append([]string{"atom", "ai"}, a.Tags...), Wikilinks: slugs,
		})
		if err != nil {
			return nil, fmt.Errorf("kb: create atom: %w", err)
		}
		if err := ingestRepo.Record(ctx, "article", fmt.Sprintf("%d", a.ID), note.ID, "processed"); err != nil {
			return nil, err
		}

		for _, c := range slugs {
			agg, ok := concepts[c]
			if !ok {
				agg = &conceptAgg{Name: s.conceptName(a, c)}
				concepts[c] = agg
			}
			agg.Refs = append(agg.Refs, atomRef{Slug: slug, Title: a.Title})
		}
		res.AtomsCreated++
	}

	// Electrons for concepts cited by enough atoms.
	names := make([]string, 0, len(concepts))
	for k := range concepts {
		names = append(names, k)
	}
	sort.Strings(names)
	for _, slug := range names {
		agg := concepts[slug]
		if len(agg.Refs) < s.opts.MinOccurrences {
			continue
		}
		created, err := s.upsertElectron(ctx, kbRepo, slug, agg)
		if err != nil {
			return nil, err
		}
		if created {
			res.ElectronsCreated++
		} else {
			res.ElectronsUpdated++
		}
	}

	if s.logger != nil {
		s.logger.Printf("kb build: %d atoms, %d electrons", res.AtomsCreated, res.ElectronsCreated+res.ElectronsUpdated)
	}
	return res, nil
}

// upsertElectron creates an electron if missing, or recomputes its content from
// every atom that links to it.
func (s *Service) upsertElectron(ctx context.Context, repo *db.KBRepo, slug string, agg *conceptAgg) (created bool, err error) {
	existing, err := repo.GetBySlug(ctx, slug)
	if err != nil && err != db.ErrNotFound {
		return false, err
	}

	sources := make([]atomRef, 0, len(agg.Refs))
	if existing != nil {
		// Recompute from all current incoming atoms, not just this run's.
		incoming, err := repo.Incoming(ctx, slug, 500)
		if err != nil {
			return false, err
		}
		for _, n := range incoming {
			sources = append(sources, atomRef{Slug: n.Slug, Title: n.Title})
		}
	} else {
		sources = agg.Refs
	}

	content := BuildElectron(agg.Name, sources)
	fm, _ := json.Marshal(map[string]any{"type": "electron", "name": agg.Name, "tags": []string{"ai", "concept"}})
	tags := []string{"ai", "concept"}

	if existing != nil {
		existing.Content = content
		existing.Frontmatter = string(fm)
		existing.Tags = tags
		existing.Wikilinks = nil
		for _, s := range sources {
			existing.Wikilinks = append(existing.Wikilinks, s.Slug)
		}
		if err := repo.Update(ctx, existing); err != nil {
			return false, err
		}
		return false, nil
	}

	path, err := s.vault.Write("electron", slug, content)
	if err != nil {
		return false, err
	}
	links := make([]string, 0, len(sources))
	for _, s := range sources {
		links = append(links, s.Slug)
	}
	_, err = repo.Create(ctx, db.NewKBNote{
		Type: db.NoteElectron, Title: agg.Name, Slug: slug, Path: path,
		Frontmatter: string(fm), Content: content, Tags: tags, Wikilinks: links,
	})
	if err != nil {
		return false, err
	}
	return true, nil
}

// conceptSlugs returns the electron slugs an article links out to.
func (s *Service) conceptSlugs(a *db.Article) []string {
	seen := map[string]bool{}
	var out []string
	for _, e := range a.Entities {
		switch e.Type {
		case "org", "product", "model", "person", "paper":
			sl := Slugify(e.Name)
			if sl == "" || seen[sl] {
				continue
			}
			seen[sl] = true
			out = append(out, sl)
		}
	}
	for _, t := range a.Tags {
		t = strings.ToLower(strings.TrimSpace(t))
		if t == "" || t == "ai" || t == "llm" || t == "general" {
			continue
		}
		sl := Slugify(t)
		if sl == "" || seen[sl] {
			continue
		}
		seen[sl] = true
		out = append(out, sl)
	}
	sort.Strings(out)
	return out
}

func (s *Service) conceptName(a *db.Article, slug string) string {
	for _, e := range a.Entities {
		if Slugify(e.Name) == slug {
			return e.Name
		}
	}
	for _, t := range a.Tags {
		if Slugify(t) == slug {
			return t
		}
	}
	return slug
}

func (s *Service) uniqueSlug(ctx context.Context, repo *db.KBRepo, base string) string {
	slug := base
	for i := 2; ; i++ {
		if _, err := repo.GetBySlug(ctx, slug); err == db.ErrNotFound {
			return slug
		} else if err != nil {
			return base
		}
		slug = fmt.Sprintf("%s-%d", base, i)
	}
}

// Synthesize creates (or updates) a Molecule note summarizing a category by
// linking its atoms. Uses the LLM when enabled; degrades to a deterministic
// listing otherwise.
func (s *Service) Synthesize(ctx context.Context, category string) (*BuildResult, error) {
	repo := db.NewKBRepo(s.db)
	atoms, err := repo.ByCategory(ctx, category, 100)
	if err != nil {
		return nil, err
	}
	if len(atoms) == 0 {
		return &BuildResult{}, nil
	}

	refs := make([]atomRef, 0, len(atoms))
	for _, n := range atoms {
		refs = append(refs, atomRef{Slug: n.Slug, Title: n.Title})
	}

	title := "Síntesis de " + category
	summary := ""
	if s.opts.UseLLM && s.prov != nil {
		summary, err = s.summarizeCategory(ctx, category, refs)
		if err != nil && s.logger != nil {
			s.logger.Printf("kb: molecule summary failed: %v", err)
		}
	}

	content := BuildMolecule(title, summary, refs)
	slug := "sintesis-" + Slugify(category)

	existing, err := repo.GetBySlug(ctx, slug)
	if err != nil && err != db.ErrNotFound {
		return nil, err
	}

	res := &BuildResult{}
	fm, _ := json.Marshal(map[string]any{"type": "molecule", "category": category, "tags": []string{"synthesis", "ai"}})
	tags := []string{"synthesis", "ai"}
	links := make([]string, 0, len(refs))
	for _, r := range refs {
		links = append(links, r.Slug)
	}

	if existing != nil {
		existing.Title = title
		existing.Content = content
		existing.Frontmatter = string(fm)
		existing.Tags = tags
		existing.Wikilinks = links
		if err := repo.Update(ctx, existing); err != nil {
			return nil, err
		}
		_, _ = s.vault.Write("molecule", slug, content)
	} else {
		path, err := s.vault.Write("molecule", slug, content)
		if err != nil {
			return nil, err
		}
		if _, err := repo.Create(ctx, db.NewKBNote{
			Type: db.NoteMolecule, Title: title, Slug: slug, Path: path,
			Frontmatter: string(fm), Content: content, Tags: tags, Wikilinks: links,
		}); err != nil {
			return nil, err
		}
		res.MoleculesCreated = 1
	}
	return res, nil
}

func (s *Service) summarizeCategory(ctx context.Context, category string, refs []atomRef) (string, error) {
	headlines := make([]string, 0, len(refs))
	for _, r := range refs {
		headlines = append(headlines, "- "+r.Title)
	}
	resp, err := s.prov.Complete(ctx, llm.CompletionRequest{
		Model: s.opts.Model,
		Messages: []llm.Message{
			{Role: llm.RoleSystem, Content: "You synthesize AI-news notes into one insightful paragraph. Return plain text, 3-4 sentences, no markdown."},
			{Role: llm.RoleUser, Content: "Theme: " + category + "\nNotes:\n" + strings.Join(headlines, "\n")},
		},
		Temperature: 0.3,
	})
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(resp.Content), nil
}

// ListNotes returns notes, optionally filtered by type.
func (s *Service) ListNotes(ctx context.Context, noteType string, limit int) ([]*db.KBNote, error) {
	return db.NewKBRepo(s.db).List(ctx, db.NoteType(noteType), limit)
}

// GetNote returns a single note.
func (s *Service) GetNote(ctx context.Context, id int64) (*db.KBNote, error) {
	return db.NewKBRepo(s.db).Get(ctx, id)
}

// GraphNeighbors returns notes connected to the given note via wikilinks or
// shared tags.
func (s *Service) GraphNeighbors(ctx context.Context, id int64, limit int) ([]*db.KBNote, error) {
	repo := db.NewKBRepo(s.db)
	note, err := repo.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	if limit <= 0 {
		limit = 50
	}

	seen := map[int64]bool{id: true}
	var out []*db.KBNote
	add := func(n *db.KBNote) {
		if n == nil || seen[n.ID] {
			return
		}
		seen[n.ID] = true
		out = append(out, n)
	}

	// Outgoing: resolve wikilinks.
	for _, slug := range note.Wikilinks {
		if n, err := repo.GetBySlug(ctx, slug); err == nil {
			add(n)
		}
	}
	// Incoming: notes linking to this one.
	if incoming, err := repo.Incoming(ctx, note.Slug, limit); err == nil {
		for _, n := range incoming {
			add(n)
		}
	}
	// Shared tags.
	if shared, err := repo.BySharedTag(ctx, note.ID, note.Tags, limit); err == nil {
		for _, n := range shared {
			add(n)
		}
	}

	if len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}
