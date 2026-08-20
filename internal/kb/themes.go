package kb

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/ajramos/giznews/internal/db"
	"github.com/ajramos/giznews/internal/llm"
)

// A theme is a group of notes that keep naming the same concepts together.
//
// Molecules used to exist only on demand, one per category, which made them a
// second table of contents: "Síntesis de research" listed every research note
// ever written and said nothing about any of them. What a reader wants from
// the level above the note is the opposite — the handful of stories the feed
// is actually telling right now, each with its own notes, its own concepts and
// its own thread.
//
// The clustering runs on the concept graph rather than on embeddings: two notes
// belong to the same theme when they name the same concepts, which the graph
// already knows exactly, offline and for free.
const (
	themeMinShared   = 2  // notes two concepts must share to sit in one theme
	themeMinConcepts = 2  // a lone concept is an electron, not a theme
	themeMinNotes    = 3  // notes a theme needs before it earns one of its own
	themeMaxConcepts = 6  // concepts listed as holding a theme together
	themeMaxNotes    = 20 // notes listed in a molecule's thread
	themeMaxThemes   = 12 // themes written per run
	themeMaxMentions = 5000
)

// ThemeResult reports what a theme pass wrote.
type ThemeResult struct {
	Found   int `json:"found"`
	Created int `json:"created"`
	Updated int `json:"updated"`
}

// BuildThemes clusters recent notes into themes and writes a Molecule for each.
// It is part of a build, and can be run on its own when the graph moved without
// new articles (a merge, notes written by hand).
func (s *Service) BuildThemes(ctx context.Context) (*ThemeResult, error) {
	conceptRepo := db.NewConceptRepo(s.db)
	kbRepo := db.NewKBRepo(s.db)
	themeRepo := db.NewThemeRepo(s.db)

	since := time.Now().UTC().AddDate(0, 0, -s.themeDays()).Format(time.RFC3339)
	mentions, err := conceptRepo.MentionsSince(ctx, since, themeMaxMentions)
	if err != nil {
		return nil, err
	}

	// Molecules name concepts too. Counting them would let a theme feed on the
	// note it wrote last run and grow on its own evidence.
	stored, err := themeRepo.List(ctx, 200)
	if err != nil {
		return nil, err
	}
	ours := make(map[int64]bool, len(stored))
	for _, t := range stored {
		if t.NoteID != 0 {
			ours[t.NoteID] = true
		}
	}

	graph := newConceptGraph(mentions, ours)
	clusters := graph.cluster(seedOrder(stored, graph))

	res := &ThemeResult{Found: len(clusters)}
	for _, c := range clusters {
		if err := s.writeTheme(ctx, kbRepo, conceptRepo, themeRepo, c, res); err != nil {
			// One unwritable theme must not cost the others.
			if s.logger != nil {
				s.logger.Printf("kb: theme %q skipped: %v", c.Slug, err)
			}
		}
	}
	if s.logger != nil && res.Found > 0 {
		s.logger.Printf("kb themes: %d found, %d created, %d updated", res.Found, res.Created, res.Updated)
	}
	return res, nil
}

// themeDays is how far back clustering looks. Themes are about what the feed is
// saying now, so the window is deliberately wider than the one that selects
// articles — a story told over two months is still one story — but not the
// whole archive, which would cluster 2024 into today's reading.
func (s *Service) themeDays() int {
	if s.opts.ThemeDays > 0 {
		return s.opts.ThemeDays
	}
	return 90
}

// noteInfo is what clustering needs to know about a note.
type noteInfo struct {
	ID    int64
	Slug  string
	Title string
}

// conceptGraph is the window of mentions, indexed both ways.
type conceptGraph struct {
	notes     map[int64]noteInfo
	byConcept map[string]map[int64]bool
	byNote    map[int64]map[string]bool
	names     map[string]string
}

func newConceptGraph(mentions []db.ConceptMention, skip map[int64]bool) *conceptGraph {
	g := &conceptGraph{
		notes:     map[int64]noteInfo{},
		byConcept: map[string]map[int64]bool{},
		byNote:    map[int64]map[string]bool{},
		names:     map[string]string{},
	}
	for _, m := range mentions {
		if skip[m.NoteID] {
			continue
		}
		g.notes[m.NoteID] = noteInfo{ID: m.NoteID, Slug: m.NoteSlug, Title: m.Title}
		g.names[m.Slug] = m.Name
		if g.byConcept[m.Slug] == nil {
			g.byConcept[m.Slug] = map[int64]bool{}
		}
		g.byConcept[m.Slug][m.NoteID] = true
		if g.byNote[m.NoteID] == nil {
			g.byNote[m.NoteID] = map[string]bool{}
		}
		g.byNote[m.NoteID][m.Slug] = true
	}
	return g
}

// shared counts the notes two concepts have in common.
func (g *conceptGraph) shared(a, b string) int {
	n := 0
	for id := range g.byConcept[a] {
		if g.byConcept[b][id] {
			n++
		}
	}
	return n
}

// themeCluster is one theme, before it becomes a note.
type themeCluster struct {
	Seed     string
	Slug     string
	Title    string
	Concepts []ThemeConcept
	NoteIDs  []int64
}

// seedOrder decides which concept gets to anchor a theme first: the themes that
// already exist keep their anchor, so a molecule written last week is updated
// rather than replaced by a near-identical one under a different slug. Only
// then do the busiest remaining concepts get their turn.
func seedOrder(stored []*db.Theme, g *conceptGraph) []string {
	var out []string
	seen := map[string]bool{}
	for _, t := range stored {
		if t.Seed == "" || seen[t.Seed] {
			continue
		}
		seen[t.Seed] = true
		out = append(out, t.Seed)
	}
	rest := make([]string, 0, len(g.byConcept))
	for slug := range g.byConcept {
		if !seen[slug] {
			rest = append(rest, slug)
		}
	}
	sort.Slice(rest, func(i, j int) bool {
		if a, b := len(g.byConcept[rest[i]]), len(g.byConcept[rest[j]]); a != b {
			return a > b
		}
		return rest[i] < rest[j]
	})
	return append(out, rest...)
}

// cluster grows one theme per seed: the concepts that share notes with it, and
// the notes naming at least two of them. Requiring two is what keeps a molecule
// from restating an electron — a note tied to the theme by a single concept is
// already listed on that concept's own page.
func (g *conceptGraph) cluster(seeds []string) []*themeCluster {
	used := map[string]bool{}
	var out []*themeCluster

	for _, seed := range seeds {
		if used[seed] || len(g.byConcept[seed]) < themeMinShared || len(out) >= themeMaxThemes {
			continue
		}
		partners := g.partnersOf(seed, used)
		if len(partners)+1 < themeMinConcepts {
			continue
		}
		members := append([]string{seed}, partners...)

		var noteIDs []int64
		for id, concepts := range g.byNote {
			hits := 0
			for _, m := range members {
				if concepts[m] {
					hits++
				}
			}
			if hits >= 2 {
				noteIDs = append(noteIDs, id)
			}
		}
		if len(noteIDs) < themeMinNotes {
			continue // the overlap was too thin to be a story
		}
		sort.Slice(noteIDs, func(i, j int) bool { return noteIDs[i] < noteIDs[j] })

		concepts := make([]ThemeConcept, 0, len(members))
		for _, m := range members {
			n := 0
			for _, id := range noteIDs {
				if g.byNote[id][m] {
					n++
				}
			}
			if n == 0 {
				continue
			}
			concepts = append(concepts, ThemeConcept{Slug: m, Name: g.names[m], Notes: n})
		}
		if len(concepts) < themeMinConcepts {
			continue
		}
		sort.Slice(concepts, func(i, j int) bool {
			if concepts[i].Notes != concepts[j].Notes {
				return concepts[i].Notes > concepts[j].Notes
			}
			return concepts[i].Slug < concepts[j].Slug
		})

		for _, m := range members {
			used[m] = true
		}
		out = append(out, &themeCluster{
			Seed: seed, Slug: "theme-" + seed, Title: themeTitle(concepts),
			Concepts: concepts, NoteIDs: noteIDs,
		})
	}
	return out
}

// partnersOf returns the concepts that share enough notes with the seed, the
// closest first.
func (g *conceptGraph) partnersOf(seed string, used map[string]bool) []string {
	type partner struct {
		slug   string
		shared int
	}
	var found []partner
	for slug := range g.byConcept {
		if slug == seed || used[slug] {
			continue
		}
		if n := g.shared(seed, slug); n >= themeMinShared {
			found = append(found, partner{slug: slug, shared: n})
		}
	}
	sort.Slice(found, func(i, j int) bool {
		if found[i].shared != found[j].shared {
			return found[i].shared > found[j].shared
		}
		return found[i].slug < found[j].slug
	})
	if len(found) > themeMaxConcepts-1 {
		found = found[:themeMaxConcepts-1]
	}
	out := make([]string, 0, len(found))
	for _, p := range found {
		out = append(out, p.slug)
	}
	return out
}

// themeTitle names a theme after the concepts carrying most of it.
func themeTitle(concepts []ThemeConcept) string {
	names := make([]string, 0, 3)
	for _, c := range concepts {
		if len(names) == 3 {
			break
		}
		names = append(names, c.Name)
	}
	return strings.Join(names, " · ")
}

// writeTheme turns one cluster into its Molecule note.
func (s *Service) writeTheme(ctx context.Context, kbRepo *db.KBRepo, conceptRepo *db.ConceptRepo,
	themeRepo *db.ThemeRepo, c *themeCluster, res *ThemeResult) error {

	notes, err := s.themeNotes(ctx, kbRepo, c.NoteIDs)
	if err != nil {
		return err
	}
	if len(notes) < themeMinNotes {
		return nil // the notes went away between the query and now
	}

	// Mention counts across the whole graph, not just this window: a concept
	// carrying three notes here and forty elsewhere is a different animal from
	// one that only exists inside this theme.
	concepts := make([]ThemeConcept, 0, len(c.Concepts))
	for _, tc := range c.Concepts {
		if concept, err := conceptRepo.Get(ctx, tc.Slug); err == nil {
			tc.Total = concept.Mentions
			if concept.Name != "" {
				tc.Name = concept.Name
			}
		} else if !errors.Is(err, db.ErrNotFound) {
			return err
		}
		concepts = append(concepts, tc)
	}
	view := MoleculeView{Title: themeTitle(concepts), Concepts: concepts, Notes: notes}

	stored, err := themeRepo.Get(ctx, c.Slug)
	if err != nil && !errors.Is(err, db.ErrNotFound) {
		return err
	}
	if stored != nil {
		// The note keeps the day the theme was first written: dating it by the
		// build would rewrite every molecule in the vault every night, for no
		// change a reader could see.
		if t, err := time.Parse(time.RFC3339, stored.FirstSeen); err == nil {
			view.Created = noteTime(t)
		}
	}
	key := themeSummaryKey(view)
	switch {
	case stored != nil && stored.SummaryKey == key:
		view.Summary = stored.Summary // nothing it was written from moved
	case s.opts.UseLLM && s.prov != nil:
		summary, err := s.summarizeTheme(ctx, view)
		if err != nil {
			if s.logger != nil {
				s.logger.Printf("kb: could not summarize theme %q: %v", c.Slug, err)
			}
			if stored != nil {
				view.Summary = stored.Summary
			}
		} else {
			view.Summary = summary
		}
	case stored != nil:
		view.Summary = stored.Summary // keep the last one we had
	}

	note, created, err := s.upsertMolecule(ctx, kbRepo, c.Slug, view)
	if err != nil {
		return err
	}
	if note == nil {
		return nil // the slug belongs to a note of another kind
	}
	if created {
		res.Created++
	} else {
		res.Updated++
	}

	slugs := make([]string, 0, len(concepts))
	for _, tc := range concepts {
		slugs = append(slugs, tc.Slug)
	}
	summaryKey := key
	if view.Summary == "" {
		summaryKey = "" // nothing was written, so nothing is cached
	}
	return themeRepo.Save(ctx, &db.Theme{
		Slug: c.Slug, Title: view.Title, Seed: c.Seed, Concepts: slugs,
		Summary: view.Summary, SummaryKey: summaryKey, NoteID: note.ID,
	})
}

// themeNotes loads a cluster's notes in the order they happened, keeping the
// most recent ones when there are more than a note can usefully list.
func (s *Service) themeNotes(ctx context.Context, repo *db.KBRepo, ids []int64) ([]atomRef, error) {
	refs := make([]atomRef, 0, len(ids))
	for _, id := range ids {
		note, err := repo.Get(ctx, id)
		if err != nil {
			if errors.Is(err, db.ErrNotFound) {
				continue
			}
			return nil, err
		}
		refs = append(refs, atomRef{Slug: note.Slug, Title: note.Title, NoteID: note.ID, Date: noteDay(note)})
	}
	sort.Slice(refs, func(i, j int) bool {
		if refs[i].Date != refs[j].Date {
			return refs[i].Date < refs[j].Date
		}
		return refs[i].NoteID < refs[j].NoteID
	})
	if len(refs) > themeMaxNotes {
		refs = refs[len(refs)-themeMaxNotes:]
	}
	return refs, nil
}

// noteDay is the day a note is about: what its frontmatter says, which for an
// atom is when the news was published, not when the build ran.
func noteDay(n *db.KBNote) string {
	if fm, _ := splitFrontmatter(n.Content); fm != "" {
		blocks, _ := frontmatterBlocks(fm)
		if lines := blocks["created"]; len(lines) > 0 {
			if _, value, ok := strings.Cut(strings.TrimRight(lines[0], "\n"), ":"); ok {
				if day := day(unquoteYAML(strings.TrimSpace(value))); day != "" {
					return day
				}
			}
		}
	}
	return day(n.CreatedAt)
}

// day takes the date out of a timestamp in any of the shapes the vault and the
// database write.
func day(ts string) string {
	if len(ts) < 10 {
		return ""
	}
	head := ts[:10]
	if _, err := time.Parse("2006-01-02", head); err != nil {
		return ""
	}
	return head
}

// themeSummaryKey fingerprints what a theme's summary was written from.
func themeSummaryKey(v MoleculeView) string {
	parts := make([]string, 0, len(v.Notes)+1)
	parts = append(parts, v.Title)
	for _, n := range v.Notes {
		parts = append(parts, n.Slug)
	}
	sort.Strings(parts[1:])
	return hashOf([]byte(strings.Join(parts, "\x00")))
}

// summarizeTheme asks the model what the theme's notes have in common.
func (s *Service) summarizeTheme(ctx context.Context, v MoleculeView) (string, error) {
	names := make([]string, 0, len(v.Concepts))
	for _, c := range v.Concepts {
		names = append(names, c.Name)
	}
	headlines := make([]string, 0, len(v.Notes))
	for _, n := range v.Notes {
		line := "- " + n.Title
		if n.Date != "" {
			line = "- " + n.Date + " · " + n.Title
		}
		headlines = append(headlines, line)
	}
	resp, err := s.prov.Complete(ctx, llm.CompletionRequest{
		Model: s.opts.Model,
		Messages: []llm.Message{
			{Role: llm.RoleSystem, Content: "You open a synthesis note that ties several AI-news items into one theme. " +
				"Say what the through-line is and why it matters, in 3-4 sentences of plain text. " +
				"No markdown, no preamble, no list of the headlines back." +
				llm.LanguageInstruction(s.opts.Language)},
			{Role: llm.RoleUser, Content: "Concepts: " + strings.Join(names, ", ") + "\nNotes:\n" + strings.Join(headlines, "\n")},
		},
		Temperature: 0.3,
	})
	if err != nil {
		return "", err
	}
	summary := strings.TrimSpace(resp.Content)
	if summary == "" {
		return "", fmt.Errorf("empty summary")
	}
	return summary, nil
}

// upsertMolecule writes a Molecule note, creating it or rewriting the one that
// is there. A slug owned by another kind of note is left alone, exactly as an
// electron's is: overwriting it would destroy that note.
func (s *Service) upsertMolecule(ctx context.Context, repo *db.KBRepo, slug string, v MoleculeView) (*db.KBNote, bool, error) {
	existing, err := repo.GetBySlug(ctx, slug)
	if err != nil && !errors.Is(err, db.ErrNotFound) {
		return nil, false, err
	}
	if existing != nil && existing.Type != db.NoteMolecule {
		if s.logger != nil {
			s.logger.Printf("kb: molecule %q skipped, slug owned by %s note #%d", slug, existing.Type, existing.ID)
		}
		return nil, false, nil
	}

	content := BuildMolecule(v)
	links := make([]string, 0, len(v.Notes)+len(v.Concepts))
	for _, n := range v.Notes {
		links = appendUnique(links, n.Slug)
	}
	for _, c := range v.Concepts {
		links = appendUnique(links, c.Slug)
	}
	tags := []string{"synthesis", "ai", "theme"}
	fm, _ := json.Marshal(map[string]any{"type": "molecule", "title": v.Title, "tags": tags})

	in := SyncInput{NoteType: "molecule", Slug: slug, Content: content}
	if existing != nil {
		in.LastHash = existing.ContentHash
		in.LastTags = existing.Tags
	}
	written, err := s.syncNote(in)
	if err != nil {
		return nil, false, err
	}

	if existing != nil {
		existing.Title = v.Title
		existing.Content = content
		existing.Frontmatter = string(fm)
		existing.Tags = tags
		existing.Wikilinks = links
		if err := repo.Update(ctx, existing); err != nil {
			return nil, false, err
		}
		if err := repo.SetContentHash(ctx, existing.ID, written.Hash); err != nil {
			return nil, false, err
		}
		existing.ContentHash = written.Hash
		return existing, false, nil
	}

	note, err := repo.Create(ctx, db.NewKBNote{
		Type: db.NoteMolecule, Title: v.Title, Slug: slug, Path: written.Path,
		Frontmatter: string(fm), Content: content, Tags: tags, Wikilinks: links,
		ContentHash: written.Hash,
	})
	if err != nil {
		_ = os.Remove(written.Path)
		return nil, false, fmt.Errorf("kb: create molecule: %w", err)
	}
	return note, true, nil
}
