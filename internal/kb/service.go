package kb

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
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
	Language            string // ISO 639-1 for LLM-generated synthesis
}

// BuildResult reports what a build run did.
type BuildResult struct {
	AtomsCreated     int `json:"atoms_created"`
	ElectronsCreated int `json:"electrons_created"`
	ElectronsUpdated int `json:"electrons_updated"`
	MoleculesCreated int `json:"molecules_created"`
	// ArticlesSkipped counts articles that were selected for the graph but did
	// not produce an atom (a vault write or insert failed).
	ArticlesSkipped int `json:"articles_skipped"`
	// ConceptsTracked counts the distinct concepts this run recorded mentions
	// for, promoted to an electron or not.
	ConceptsTracked int `json:"concepts_tracked"`
	// AtomsRefreshed counts atoms rewritten because their article changed after
	// the note was written (a re-classification, a body extracted later).
	AtomsRefreshed int `json:"atoms_refreshed"`
	// EditedNotesKept counts how many of those files the user had edited: their
	// text was preserved, by merging the generated region or by leaving the
	// file alone when there was none to merge.
	EditedNotesKept int `json:"edited_notes_kept"`
	// NotesImported counts hand-written vault notes read into the graph.
	NotesImported int `json:"notes_imported"`
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

// conceptAgg accumulates, for one concept, the atoms citing it during a build.
type conceptAgg struct {
	Name string
	Slug string // slug its electron uses; may differ from the raw concept slug
	Refs []atomRef
}

// Build ingests pending articles into the graph as Atom notes and creates or
// updates Electron notes for recurring concepts.
func (s *Service) Build(ctx context.Context) (*BuildResult, error) {
	res := &BuildResult{}
	artRepo := db.NewArticleRepo(s.db)
	kbRepo := db.NewKBRepo(s.db)
	ingestRepo := db.NewIngestRepo(s.db)
	conceptRepo := db.NewConceptRepo(s.db)

	// The reader's own notes join the graph before anything else is written;
	// what they cite is counted further down, once this run's atoms exist.
	if imported, err := s.importVaultFiles(ctx); err != nil {
		if s.logger != nil {
			s.logger.Printf("kb: vault scan failed: %v", err)
		}
	} else {
		res.NotesImported = imported.Imported + imported.Updated
	}

	articles, err := artRepo.ListForKB(ctx, s.opts.ImportanceThreshold, s.opts.AgeDays, s.opts.Limit)
	if err != nil {
		return nil, fmt.Errorf("kb: list articles: %w", err)
	}

	// Pass 1: collect the concepts of the whole batch and reserve the slug each
	// electron will use, before any atom is written. An article titled exactly
	// like a concept ("Mamba") would otherwise take that slug and get its note
	// overwritten when the electron is upserted.
	// Spellings are folded first: "Open AI" and "OpenAI" are one concept, and so
	// is anything the user merged by hand.
	concepts := map[string]*conceptAgg{} // keyed by the concept's canonical slug
	canonical := map[string]string{}     // slug derived from an article -> that key
	byCanonKey := map[string]string{}    // canonical key -> the spelling this run kept
	for _, a := range articles {
		for _, raw := range s.conceptSlugs(a) {
			key, ok := canonical[raw]
			if !ok {
				key, err = conceptRepo.Resolve(ctx, raw)
				if err != nil {
					return nil, fmt.Errorf("kb: resolve concept %q: %w", raw, err)
				}
				// Resolve only knows the concepts already stored; two spellings
				// arriving in the same batch must fold too, first one winning.
				if kept, seen := byCanonKey[db.CanonKey(key)]; seen {
					key = kept
				} else {
					byCanonKey[db.CanonKey(key)] = key
				}
				canonical[raw] = key
			}
			if _, ok := concepts[key]; !ok {
				concepts[key] = &conceptAgg{Name: s.conceptName(a, raw)}
			}
		}
	}
	conceptKeys := make([]string, 0, len(concepts))
	for key := range concepts {
		conceptKeys = append(conceptKeys, key)
	}
	sort.Strings(conceptKeys)
	reserved := make(map[string]bool, len(concepts))
	for _, key := range conceptKeys {
		agg := concepts[key]
		agg.Slug = s.electronSlugFor(ctx, kbRepo, key, reserved)
		reserved[agg.Slug] = true
	}

	// Pass 2: one atom per article, linking to the resolved concept slugs.
	for _, a := range articles {
		raws := s.conceptSlugs(a)
		links := make([]string, 0, len(raws))
		for _, raw := range raws {
			links = appendUnique(links, concepts[canonical[raw]].Slug)
		}
		sort.Strings(links)

		slug := s.uniqueSlug(ctx, kbRepo, Slugify(stripBrackets(a.Title)), reserved)
		note, err := s.writeAtom(ctx, kbRepo, ingestRepo, a, slug, links)
		if err != nil {
			// One unwritable article must not abort the whole run.
			if s.logger != nil {
				s.logger.Printf("kb: skipping article #%d: %v", a.ID, err)
			}
			res.ArticlesSkipped++
			continue
		}

		for _, raw := range raws {
			agg := concepts[canonical[raw]]
			agg.Refs = append(agg.Refs, atomRef{Slug: slug, Title: a.Title, NoteID: note.ID})
		}
		res.AtomsCreated++
	}

	// Electrons for concepts mentioned often enough. Mentions are counted over
	// the concept's whole history, not just this run, so a topic that surfaces
	// once a day graduates like one that surfaces five times in an afternoon.
	for _, key := range conceptKeys {
		agg := concepts[key]
		if len(agg.Refs) == 0 {
			continue
		}
		concept, err := s.recordMentions(ctx, conceptRepo, agg)
		if err != nil {
			return nil, err
		}
		res.ConceptsTracked++

		promoted := concept.NoteID != 0
		if !promoted && concept.Mentions < s.opts.MinOccurrences {
			continue
		}

		sources, err := s.conceptSources(ctx, conceptRepo, agg)
		if err != nil {
			return nil, err
		}
		outcome, note, err := s.upsertElectron(ctx, kbRepo, agg.Slug, agg.Name, sources)
		if err != nil {
			return nil, err
		}
		switch outcome {
		case electronCreated:
			res.ElectronsCreated++
			if err := conceptRepo.Promote(ctx, agg.Slug, note.ID); err != nil {
				return nil, err
			}
		case electronUpdated:
			res.ElectronsUpdated++
		}
	}

	// Now that this run's atoms exist, what the reader's notes cite can be
	// counted — including concepts that had no name in the graph until today.
	if _, err := s.recordVaultMentions(ctx); err != nil && s.logger != nil {
		s.logger.Printf("kb: counting your notes' mentions failed: %v", err)
	}

	// A concept can cross the threshold without any of this run's articles
	// naming it: a merge, or the reader's own notes, count towards it too. The
	// queue is swept so nothing sits there already qualified.
	if err := s.promoteQueued(ctx, kbRepo, conceptRepo, res); err != nil {
		return nil, err
	}

	// Notes whose article moved on since they were written catch up here.
	if err := s.refreshStaleAtoms(ctx, kbRepo, conceptRepo, res); err != nil && s.logger != nil {
		s.logger.Printf("kb: atom refresh failed: %v", err)
	}

	// Concepts first seen through a lowercase tag kept it as their name; give
	// them a readable one now that they can get it.
	if err := s.repairConceptNames(ctx, kbRepo, conceptRepo); err != nil && s.logger != nil {
		s.logger.Printf("kb: concept name repair failed: %v", err)
	}

	// The vault's entry points are a view over what the build just wrote; a
	// failure here leaves the notes themselves intact, so it is logged, not
	// returned.
	if _, err := s.BuildIndex(ctx); err != nil && s.logger != nil {
		s.logger.Printf("kb: index refresh failed: %v", err)
	}

	if s.logger != nil {
		s.logger.Printf("kb build: %d atoms, %d electrons", res.AtomsCreated, res.ElectronsCreated+res.ElectronsUpdated)
	}
	return res, nil
}

// repairConceptNames upgrades the concepts whose name is still their raw slug.
// A concept named from a tag before this could read "rag" forever, because only
// a new mention would ever rewrite it; its electron is rebuilt so the vault
// shows the same name as the index.
func (s *Service) repairConceptNames(ctx context.Context, kbRepo *db.KBRepo, repo *db.ConceptRepo) error {
	raw, err := repo.RawNamed(ctx, 500)
	if err != nil {
		return err
	}
	for _, c := range raw {
		name := DisplayName(c.Slug)
		if name == c.Name {
			continue
		}
		if err := repo.Rename(ctx, c.Slug, name); err != nil {
			return err
		}
		if c.NoteID == 0 {
			continue
		}
		sources, err := s.conceptSources(ctx, repo, &conceptAgg{Slug: c.Slug, Name: name})
		if err != nil {
			return err
		}
		if _, _, err := s.upsertElectron(ctx, kbRepo, c.Slug, name, sources); err != nil {
			return err
		}
	}
	return nil
}

// recordMentions persists this run's mentions of a concept and returns its
// accumulated state.
func (s *Service) recordMentions(ctx context.Context, repo *db.ConceptRepo, agg *conceptAgg) (*db.Concept, error) {
	var (
		concept *db.Concept
		err     error
	)
	for _, ref := range agg.Refs {
		concept, err = repo.Touch(ctx, agg.Slug, agg.Name, ref.NoteID)
		if err != nil {
			return nil, fmt.Errorf("kb: record concept %q: %w", agg.Slug, err)
		}
	}
	if concept == nil {
		return repo.Touch(ctx, agg.Slug, agg.Name, 0)
	}
	return concept, nil
}

// conceptSources returns every note mentioning the concept, so an electron
// always lists its full history and not only the atoms of the current run.
func (s *Service) conceptSources(ctx context.Context, repo *db.ConceptRepo, agg *conceptAgg) ([]atomRef, error) {
	notes, err := repo.MentionedBy(ctx, agg.Slug, 500)
	if err != nil {
		return nil, err
	}
	out := make([]atomRef, 0, len(notes))
	for _, n := range notes {
		out = append(out, atomRef{Slug: n.Slug, Title: n.Title, NoteID: n.ID})
	}
	if len(out) == 0 {
		out = agg.Refs
	}
	return out, nil
}

// syncNote writes a note file through the vault and says out loud when it found
// the user's own edits there.
func (s *Service) syncNote(in SyncInput) (SyncResult, error) {
	res, err := s.vault.Sync(in)
	if err != nil || s.logger == nil {
		return res, err
	}
	switch res.Outcome {
	case SyncMerged:
		s.logger.Printf("kb: %s was edited in the vault; refreshed only the generated region", res.Path)
	case SyncKept:
		s.logger.Printf("kb: %s was edited and has no generated region; left untouched", res.Path)
	}
	return res, err
}

// promoteQueued gives a note to every concept that has enough mentions but none
// yet, whoever recorded those mentions.
func (s *Service) promoteQueued(ctx context.Context, kbRepo *db.KBRepo, conceptRepo *db.ConceptRepo, res *BuildResult) error {
	queued, err := conceptRepo.Dangling(ctx, 500)
	if err != nil {
		return err
	}
	for _, c := range queued {
		if c.Mentions < s.opts.MinOccurrences {
			continue
		}
		sources, err := s.conceptSources(ctx, conceptRepo, &conceptAgg{Slug: c.Slug, Name: c.Name})
		if err != nil {
			return err
		}
		outcome, note, err := s.upsertElectron(ctx, kbRepo, c.Slug, c.Name, sources)
		if err != nil {
			return err
		}
		if outcome == electronCreated {
			if err := conceptRepo.Promote(ctx, c.Slug, note.ID); err != nil {
				return err
			}
			res.ElectronsCreated++
		}
	}
	return nil
}

// refreshStaleAtoms rewrites the atoms whose article changed after the note was
// written. Without this an atom froze at the moment it was created: a later
// re-classification, a better summary or a body extracted afterwards never
// reached the vault, because an ingested article is never selected again.
func (s *Service) refreshStaleAtoms(ctx context.Context, kbRepo *db.KBRepo, conceptRepo *db.ConceptRepo, res *BuildResult) error {
	stale, err := db.NewArticleRepo(s.db).ListStaleNotes(ctx, s.opts.Limit)
	if err != nil {
		return err
	}
	ingestRepo := db.NewIngestRepo(s.db)
	for _, a := range stale {
		noteID, err := ingestRepo.NoteID(ctx, "article", fmt.Sprintf("%d", a.ID))
		if err != nil || noteID == 0 {
			continue
		}
		note, err := kbRepo.Get(ctx, noteID)
		if err != nil {
			continue
		}

		links, err := s.resolveConcepts(ctx, kbRepo, conceptRepo, a)
		if err != nil {
			return err
		}
		content := BuildAtom(a, links)
		if content == note.Content {
			// Nothing a reader would notice changed (an archive, a status flip):
			// mark the note fresh so it stops being reported, leave the file be.
			if err := kbRepo.MarkFresh(ctx, note.ID); err != nil {
				return err
			}
			continue
		}

		written, err := s.syncNote(SyncInput{
			NoteType: "atom", Slug: note.Slug, Content: content,
			LastHash: note.ContentHash, LastTags: note.Tags,
		})
		if err != nil {
			return err
		}
		fm, _ := json.Marshal(map[string]any{
			"type": "atom", "category": a.Category, "source": a.SourceName,
			"url": a.URL, "rating": a.Importance, "tags": a.Tags,
		})
		note.Title = a.Title
		note.Content = content
		note.Frontmatter = string(fm)
		note.Tags = append([]string{"atom", "ai"}, a.Tags...)
		note.Wikilinks = links
		if err := kbRepo.Update(ctx, note); err != nil {
			return err
		}
		if err := kbRepo.SetContentHash(ctx, note.ID, written.Hash); err != nil {
			return err
		}
		res.AtomsRefreshed++
		if written.Outcome == SyncMerged || written.Outcome == SyncKept {
			res.EditedNotesKept++
		}
	}
	return nil
}

// resolveConcepts is the concept-slug resolution one article needs on its own,
// outside a batch: fold spellings, then avoid slugs an atom already owns.
func (s *Service) resolveConcepts(ctx context.Context, kbRepo *db.KBRepo, conceptRepo *db.ConceptRepo, a *db.Article) ([]string, error) {
	reserved := map[string]bool{}
	var links []string
	for _, raw := range s.conceptSlugs(a) {
		key, err := conceptRepo.Resolve(ctx, raw)
		if err != nil {
			return nil, err
		}
		resolved := s.electronSlugFor(ctx, kbRepo, key, reserved)
		reserved[resolved] = true
		links = appendUnique(links, resolved)
	}
	sort.Strings(links)
	return links, nil
}

// writeAtom renders an article as an Atom note, writes it to the vault, mirrors
// it in SQLite and marks the article ingested. A failed insert removes the file
// again so the vault never keeps a note the database does not know about.
func (s *Service) writeAtom(ctx context.Context, kbRepo *db.KBRepo, ingestRepo *db.IngestRepo, a *db.Article, slug string, links []string) (*db.KBNote, error) {
	content := BuildAtom(a, links)
	written, err := s.syncNote(SyncInput{NoteType: "atom", Slug: slug, Content: content})
	if err != nil {
		return nil, err
	}
	fm, _ := json.Marshal(map[string]any{
		"type": "atom", "category": a.Category, "source": a.SourceName,
		"url": a.URL, "rating": a.Importance, "tags": a.Tags,
	})
	note, err := kbRepo.Create(ctx, db.NewKBNote{
		Type: db.NoteAtom, Title: a.Title, Slug: slug, Path: written.Path,
		Frontmatter: string(fm), Content: content,
		Tags: append([]string{"atom", "ai"}, a.Tags...), Wikilinks: links,
		ContentHash: written.Hash,
	})
	if err != nil {
		_ = os.Remove(written.Path)
		return nil, fmt.Errorf("kb: create atom: %w", err)
	}
	if err := ingestRepo.Record(ctx, "article", fmt.Sprintf("%d", a.ID), note.ID, "processed"); err != nil {
		return nil, err
	}
	return note, nil
}

// electronView gathers what an Electron note says: the concept's definition —
// written by the model when one is available, and reused while the notes behind
// it have not changed — plus when it is mentioned and what it shares notes with.
func (s *Service) electronView(ctx context.Context, slug, name string, sources []atomRef) (ElectronView, error) {
	view := ElectronView{Name: name, Sources: sources}
	repo := db.NewConceptRepo(s.db)

	related, err := repo.CoOccurring(ctx, slug, 8)
	if err != nil {
		return view, err
	}
	view.Related = related

	timeline, err := repo.MentionsByMonth(ctx, slug)
	if err != nil {
		return view, err
	}
	view.Timeline = timeline

	concept, err := repo.Get(ctx, slug)
	if err != nil && !errors.Is(err, db.ErrNotFound) {
		return view, err
	}
	key := definitionKey(name, sources)
	if concept != nil && concept.DefinitionKey == key {
		view.Definition = concept.Definition // nothing it was written from moved
		return view, nil
	}
	if !s.opts.UseLLM || s.prov == nil || concept == nil {
		if concept != nil {
			view.Definition = concept.Definition // keep the last one we had
		}
		return view, nil
	}

	definition, err := s.defineConcept(ctx, name, sources)
	if err != nil {
		if s.logger != nil {
			s.logger.Printf("kb: could not define %q: %v", slug, err)
		}
		view.Definition = concept.Definition
		return view, nil
	}
	if err := repo.SetDefinition(ctx, slug, definition, key); err != nil {
		return view, err
	}
	view.Definition = definition
	return view, nil
}

// definitionKey fingerprints what a definition was written from, so it is asked
// for again when the notes change and reused when they have not.
func definitionKey(name string, sources []atomRef) string {
	titles := make([]string, 0, len(sources))
	for _, src := range sources {
		titles = append(titles, src.Slug)
	}
	sort.Strings(titles)
	return hashOf([]byte(name + "\x00" + strings.Join(titles, "\x00")))
}

// defineConcept asks the model what the concept is, from the notes that name it.
func (s *Service) defineConcept(ctx context.Context, name string, sources []atomRef) (string, error) {
	const maxTitles = 14
	titles := make([]string, 0, maxTitles)
	for _, src := range sources {
		if len(titles) == maxTitles {
			break
		}
		titles = append(titles, "- "+src.Title)
	}
	resp, err := s.prov.Complete(ctx, llm.CompletionRequest{
		Model: s.opts.Model,
		Messages: []llm.Message{
			{Role: llm.RoleSystem, Content: "You write the opening of a knowledge-base note about one concept in AI. " +
				"Say what it is and why it keeps coming up, in 2-3 sentences of plain text. " +
				"No markdown, no preamble, no repetition of the headlines." +
				llm.LanguageInstruction(s.opts.Language)},
			{Role: llm.RoleUser, Content: "Concept: " + name + "\nSeen in these notes:\n" + strings.Join(titles, "\n")},
		},
		Temperature: 0.2,
	})
	if err != nil {
		return "", err
	}
	definition := strings.TrimSpace(resp.Content)
	if definition == "" {
		return "", fmt.Errorf("empty definition")
	}
	return definition, nil
}

// electronOutcome reports what upsertElectron did with one concept.
type electronOutcome int

const (
	electronCreated electronOutcome = iota
	electronUpdated
	electronSkipped
)

// upsertElectron writes the Electron note for a concept: it creates the note or
// rewrites an existing one from the given sources. A slug owned by a note of
// another type is left untouched — overwriting it would destroy that note.
func (s *Service) upsertElectron(ctx context.Context, repo *db.KBRepo, slug, name string, sources []atomRef) (electronOutcome, *db.KBNote, error) {
	existing, err := repo.GetBySlug(ctx, slug)
	if err != nil && !errors.Is(err, db.ErrNotFound) {
		return electronSkipped, nil, err
	}
	if existing != nil && existing.Type != db.NoteElectron {
		if s.logger != nil {
			s.logger.Printf("kb: electron %q skipped, slug owned by %s note #%d", slug, existing.Type, existing.ID)
		}
		return electronSkipped, nil, nil
	}

	view, err := s.electronView(ctx, slug, name, sources)
	if err != nil {
		return electronSkipped, nil, err
	}
	content := BuildElectron(view)
	fm, _ := json.Marshal(map[string]any{"type": "electron", "name": name, "tags": []string{"ai", "concept"}})
	tags := []string{"ai", "concept"}
	links := make([]string, 0, len(sources))
	for _, src := range sources {
		links = append(links, src.Slug)
	}

	// The vault file is rewritten on every upsert: it is what the user reads in
	// Obsidian, so it must carry the same backlinks as the database row — but
	// never at the cost of what the user wrote around them.
	in := SyncInput{NoteType: "electron", Slug: slug, Content: content}
	if existing != nil {
		in.LastHash = existing.ContentHash
		in.LastTags = existing.Tags
	}
	written, err := s.syncNote(in)
	if err != nil {
		return electronSkipped, nil, err
	}

	if existing != nil {
		existing.Title = name
		existing.Content = content
		existing.Frontmatter = string(fm)
		existing.Tags = tags
		existing.Wikilinks = links
		if err := repo.Update(ctx, existing); err != nil {
			return electronSkipped, nil, err
		}
		if err := repo.SetContentHash(ctx, existing.ID, written.Hash); err != nil {
			return electronSkipped, nil, err
		}
		existing.ContentHash = written.Hash
		return electronUpdated, existing, nil
	}

	note, err := repo.Create(ctx, db.NewKBNote{
		Type: db.NoteElectron, Title: name, Slug: slug, Path: written.Path,
		Frontmatter: string(fm), Content: content, Tags: tags, Wikilinks: links,
		ContentHash: written.Hash,
	})
	if err != nil {
		_ = os.Remove(written.Path)
		return electronSkipped, nil, err
	}
	return electronCreated, note, nil
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

// conceptName is the display name for a concept derived from an article: the
// entity's own spelling when the model gave one, otherwise the tag or slug
// expanded into something readable.
func (s *Service) conceptName(a *db.Article, slug string) string {
	for _, e := range a.Entities {
		if Slugify(e.Name) == slug {
			return DisplayName(e.Name)
		}
	}
	for _, t := range a.Tags {
		if Slugify(t) == slug {
			return DisplayName(t)
		}
	}
	return DisplayName(slug)
}

// uniqueSlug returns a free slug derived from base, avoiding both the slugs
// already in the database and the ones this run reserved for electrons.
func (s *Service) uniqueSlug(ctx context.Context, repo *db.KBRepo, base string, reserved map[string]bool) string {
	const maxAttempts = 50
	for i := 1; i <= maxAttempts; i++ {
		slug := base
		if i > 1 {
			slug = fmt.Sprintf("%s-%d", base, i)
		}
		if reserved[slug] {
			continue
		}
		if _, err := repo.GetBySlug(ctx, slug); errors.Is(err, db.ErrNotFound) {
			return slug
		}
	}
	return fmt.Sprintf("%s-%d", base, maxAttempts+1)
}

// electronSlugFor picks the slug an electron for concept may live at. The plain
// concept slug is preferred so [[wikilinks]] stay readable, but a slug already
// owned by an atom or a molecule is never reused — the electron is suffixed
// instead. The rule is deterministic, so later runs resolve the same concept to
// the same slug.
func (s *Service) electronSlugFor(ctx context.Context, repo *db.KBRepo, concept string, reserved map[string]bool) string {
	candidates := []string{concept, concept + "-concept"}
	for i := 2; i <= 10; i++ {
		candidates = append(candidates, fmt.Sprintf("%s-concept-%d", concept, i))
	}
	for _, cand := range candidates {
		if reserved[cand] {
			continue
		}
		n, err := repo.GetBySlug(ctx, cand)
		if errors.Is(err, db.ErrNotFound) || (err == nil && n.Type == db.NoteElectron) {
			return cand
		}
	}
	return concept + "-concept"
}

func appendUnique(list []string, s string) []string {
	for _, existing := range list {
		if existing == s {
			return list
		}
	}
	return append(list, s)
}

// EnsureArticleNote creates (if missing) an Atom note for a single article,
// regardless of its importance threshold. If the note already exists it is
// re-rendered from the current article, so a later re-classification is
// reflected in the vault.
func (s *Service) EnsureArticleNote(ctx context.Context, articleID int64) (*db.KBNote, error) {
	artRepo := db.NewArticleRepo(s.db)
	kbRepo := db.NewKBRepo(s.db)
	ingestRepo := db.NewIngestRepo(s.db)

	art, err := artRepo.Get(ctx, articleID)
	if err != nil {
		return nil, err
	}

	conceptRepo := db.NewConceptRepo(s.db)
	links, err := s.resolveConcepts(ctx, kbRepo, conceptRepo, art)
	if err != nil {
		return nil, err
	}
	names := map[string]string{}
	for _, raw := range s.conceptSlugs(art) {
		if key, err := conceptRepo.Resolve(ctx, raw); err == nil {
			names[key] = s.conceptName(art, raw)
		}
	}

	content := BuildAtom(art, links)
	fm, _ := json.Marshal(map[string]any{
		"type": "atom", "category": art.Category, "source": art.SourceName,
		"url": art.URL, "rating": art.Importance, "tags": art.Tags,
	})
	tags := append([]string{"atom", "ai"}, art.Tags...)

	refID := fmt.Sprintf("%d", articleID)
	var note *db.KBNote
	if noteID, _ := ingestRepo.NoteID(ctx, "article", refID); noteID != 0 {
		existing, err := kbRepo.Get(ctx, noteID)
		if err != nil {
			return nil, err
		}
		written, err := s.syncNote(SyncInput{
			NoteType: "atom", Slug: existing.Slug, Content: content,
			LastHash: existing.ContentHash, LastTags: existing.Tags,
		})
		if err != nil {
			return nil, err
		}
		existing.Title = art.Title
		existing.Frontmatter = string(fm)
		existing.Content = content
		existing.Tags = tags
		existing.Wikilinks = links
		if err := kbRepo.Update(ctx, existing); err != nil {
			return nil, err
		}
		if err := kbRepo.SetContentHash(ctx, existing.ID, written.Hash); err != nil {
			return nil, err
		}
		existing.ContentHash = written.Hash
		note = existing
	} else {
		// The atom must not take a slug one of its own concepts will need.
		reserved := make(map[string]bool, len(links))
		for _, link := range links {
			reserved[link] = true
		}
		slug := s.uniqueSlug(ctx, kbRepo, Slugify(stripBrackets(art.Title)), reserved)
		created, err := s.writeAtom(ctx, kbRepo, ingestRepo, art, slug, links)
		if err != nil {
			return nil, err
		}
		note = created
	}

	// A note materialized on demand still counts towards its concepts, so the
	// graph panel does not build a parallel history the next run ignores.
	for _, link := range links {
		if _, err := conceptRepo.Touch(ctx, link, names[link], note.ID); err != nil {
			return nil, err
		}
	}
	return note, nil
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
		written, err := s.syncNote(SyncInput{
			NoteType: "molecule", Slug: slug, Content: content,
			LastHash: existing.ContentHash, LastTags: existing.Tags,
		})
		if err != nil {
			return nil, err
		}
		if err := repo.SetContentHash(ctx, existing.ID, written.Hash); err != nil {
			return nil, err
		}
	} else {
		written, err := s.syncNote(SyncInput{NoteType: "molecule", Slug: slug, Content: content})
		if err != nil {
			return nil, err
		}
		if _, err := repo.Create(ctx, db.NewKBNote{
			Type: db.NoteMolecule, Title: title, Slug: slug, Path: written.Path,
			Frontmatter: string(fm), Content: content, Tags: tags, Wikilinks: links,
			ContentHash: written.Hash,
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
			{Role: llm.RoleSystem, Content: "You synthesize AI-news notes into one insightful paragraph. Return plain text, 3-4 sentences, no markdown." + llm.LanguageInstruction(s.opts.Language)},
			{Role: llm.RoleUser, Content: "Theme: " + category + "\nNotes:\n" + strings.Join(headlines, "\n")},
		},
		Temperature: 0.3,
	})
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(resp.Content), nil
}

// PromoteConcept gives a concept its Electron note now, whatever its mention
// count. The threshold decides what is promoted automatically; a reader looking
// at the promotion queue has already decided this one is worth a note.
func (s *Service) PromoteConcept(ctx context.Context, slug string) (*db.KBNote, error) {
	kbRepo := db.NewKBRepo(s.db)
	conceptRepo := db.NewConceptRepo(s.db)

	concept, err := conceptRepo.Get(ctx, slug)
	if err != nil {
		return nil, fmt.Errorf("kb: promote %q: %w", slug, err)
	}
	sources, err := s.conceptSources(ctx, conceptRepo, &conceptAgg{Slug: concept.Slug, Name: concept.Name})
	if err != nil {
		return nil, err
	}
	outcome, note, err := s.upsertElectron(ctx, kbRepo, concept.Slug, concept.Name, sources)
	if err != nil {
		return nil, err
	}
	if note == nil {
		return nil, fmt.Errorf("kb: %q cannot have a note: its slug belongs to another one", slug)
	}
	if outcome == electronCreated {
		if err := conceptRepo.Promote(ctx, concept.Slug, note.ID); err != nil {
			return nil, err
		}
	}
	return note, nil
}

// MergeResult reports what a concept merge changed.
type MergeResult struct {
	NotesRelinked int  `json:"notes_relinked"`
	Mentions      int  `json:"mentions"`
	Redirected    bool `json:"redirected"`
}

// MergeConcepts folds one concept into another — "open-ai" into "openai",
// "gpt4" into "gpt-5" — moving its mentions and rewriting every note that
// linked to it, in the database and in the vault. The merged concept's own note
// becomes a redirect rather than disappearing.
func (s *Service) MergeConcepts(ctx context.Context, from, to string) (*MergeResult, error) {
	kbRepo := db.NewKBRepo(s.db)
	conceptRepo := db.NewConceptRepo(s.db)

	source, err := conceptRepo.Get(ctx, from)
	if err != nil {
		return nil, fmt.Errorf("kb: merge source %q: %w", from, err)
	}
	sourceNoteID := source.NoteID
	sourceName := source.Name

	affected, err := conceptRepo.Merge(ctx, from, to)
	if err != nil {
		return nil, err
	}

	res := &MergeResult{}
	for _, id := range affected {
		if id == sourceNoteID {
			continue // rewritten as the redirect below
		}
		note, err := kbRepo.Get(ctx, id)
		if err != nil {
			continue
		}
		note.Content = strings.ReplaceAll(note.Content, "[["+from+"]]", "[["+to+"]]")
		note.Wikilinks = replaceInSlice(note.Wikilinks, from, to)
		if err := s.syncExisting(ctx, kbRepo, note); err != nil {
			return nil, err
		}
		res.NotesRelinked++
	}

	target, err := conceptRepo.Get(ctx, to)
	if err != nil {
		return nil, err
	}
	res.Mentions = target.Mentions

	if sourceNoteID != 0 {
		if note, err := kbRepo.Get(ctx, sourceNoteID); err == nil {
			note.Content = BuildRedirect(sourceName, target.Name, to)
			note.Wikilinks = []string{to}
			if err := s.syncExisting(ctx, kbRepo, note); err != nil {
				return nil, err
			}
			res.Redirected = true
		}
	}

	// The surviving concept is rewritten from its now larger mention list.
	if target.NoteID != 0 || target.Mentions >= s.opts.MinOccurrences {
		sources, err := s.conceptSources(ctx, conceptRepo, &conceptAgg{Slug: to, Name: target.Name})
		if err != nil {
			return nil, err
		}
		outcome, note, err := s.upsertElectron(ctx, kbRepo, to, target.Name, sources)
		if err != nil {
			return nil, err
		}
		if outcome == electronCreated {
			if err := conceptRepo.Promote(ctx, to, note.ID); err != nil {
				return nil, err
			}
		}
	}
	return res, nil
}

// syncExisting persists an already-loaded note and mirrors it to the vault,
// keeping whatever the user wrote around the generated region.
func (s *Service) syncExisting(ctx context.Context, repo *db.KBRepo, note *db.KBNote) error {
	written, err := s.syncNote(SyncInput{
		NoteType: string(note.Type), Slug: note.Slug, Content: note.Content,
		LastHash: note.ContentHash, LastTags: note.Tags,
	})
	if err != nil {
		return err
	}
	if err := repo.Update(ctx, note); err != nil {
		return err
	}
	if err := repo.SetContentHash(ctx, note.ID, written.Hash); err != nil {
		return err
	}
	note.ContentHash = written.Hash
	return nil
}

func replaceInSlice(list []string, from, to string) []string {
	var out []string
	for _, s := range list {
		if s == from {
			s = to
		}
		out = appendUnique(out, s)
	}
	return out
}

// ListNotes returns notes, optionally filtered by type.
func (s *Service) ListNotes(ctx context.Context, noteType string, limit int) ([]*db.KBNote, error) {
	return db.NewKBRepo(s.db).List(ctx, db.NoteType(noteType), limit)
}

// GetNote returns a single note.
func (s *Service) GetNote(ctx context.Context, id int64) (*db.KBNote, error) {
	return db.NewKBRepo(s.db).Get(ctx, id)
}

// genericTags are structural: every atom carries "atom" and "ai", every
// electron "concept". Expanding the graph on them would make every note a
// neighbour of every other note, which is the same as having no graph.
var genericTags = map[string]bool{"ai": true, "atom": true, "concept": true, "synthesis": true}

// GraphNeighbors returns the notes connected to the given one: what it links
// to, what links to it, what shares a concept with it, and finally what shares
// a meaningful tag.
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
	// Siblings: notes citing at least one of the same concepts.
	if siblings, err := repo.CoMentioned(ctx, note.ID, limit); err == nil {
		for _, n := range siblings {
			add(n)
		}
	}
	// Shared tags, ignoring the ones every note of a type carries.
	var tags []string
	for _, t := range note.Tags {
		if !genericTags[t] {
			tags = append(tags, t)
		}
	}
	if len(tags) > 0 {
		if shared, err := repo.BySharedTag(ctx, note.ID, tags, limit); err == nil {
			for _, n := range shared {
				add(n)
			}
		}
	}

	if len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}
