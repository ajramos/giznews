package kb

import (
	"context"
	"errors"
	"fmt"
	"sort"

	"github.com/ajramos/giznews/internal/db"
)

// BuildPreview is what a build would do, worked out without doing any of it.
// `kb build` writes into a directory the reader keeps open in Obsidian, so it
// should be possible to ask first.
type BuildPreview struct {
	MinOccurrences int `json:"min_occurrences"`
	AgeDays        int `json:"age_days"`
	Limit          int `json:"limit"`
	Importance     int `json:"importance_threshold"`

	Atoms      []PreviewAtom    `json:"atoms"`
	Promoting  []PreviewConcept `json:"promoting"`
	Pending    []PreviewConcept `json:"pending"`
	StaleAtoms int              `json:"stale_atoms"`
	VaultNew   int              `json:"vault_new"`
	VaultEdits int              `json:"vault_edits"`
}

// PreviewAtom is an article that would become a note.
type PreviewAtom struct {
	Title      string   `json:"title"`
	Slug       string   `json:"slug"`
	Category   string   `json:"category"`
	Importance int      `json:"importance"`
	Concepts   []string `json:"concepts"`
}

// PreviewConcept is a concept and where its mention count would land.
type PreviewConcept struct {
	Slug     string `json:"slug"`
	Name     string `json:"name"`
	Mentions int    `json:"mentions"` // recorded so far
	After    int    `json:"after"`    // once this run is counted
}

// Preview reports what Build would write. It touches nothing: no note, no row,
// no mention. Numbers come from the same queries the build itself runs, so a
// preview and the run that follows it see the same world unless something else
// changes in between.
func (s *Service) Preview(ctx context.Context) (*BuildPreview, error) {
	kbRepo := db.NewKBRepo(s.db)
	conceptRepo := db.NewConceptRepo(s.db)
	artRepo := db.NewArticleRepo(s.db)

	out := &BuildPreview{
		MinOccurrences: s.opts.MinOccurrences,
		AgeDays:        s.opts.AgeDays,
		Limit:          s.opts.Limit,
		Importance:     s.opts.ImportanceThreshold,
	}

	incoming := map[string]int{} // concept slug -> mentions this run would add
	names := map[string]string{} // and the name each would carry

	// What the news would add.
	articles, err := artRepo.ListForKB(ctx, s.opts.ImportanceThreshold, s.opts.AgeDays, s.opts.Limit)
	if err != nil {
		return nil, err
	}
	// One slug per concept, decided once — reserving per article would hand the
	// second article citing a concept a suffixed slug of its own.
	reserved := map[string]bool{}
	resolved := map[string]string{}
	for _, a := range articles {
		for _, raw := range s.conceptSlugs(a) {
			if _, done := resolved[raw]; done {
				continue
			}
			key, err := conceptRepo.Resolve(ctx, raw)
			if err != nil {
				return nil, err
			}
			slug := s.electronSlugFor(ctx, kbRepo, key, reserved)
			reserved[slug] = true
			resolved[raw] = slug
			if _, ok := names[slug]; !ok {
				names[slug] = s.conceptName(a, raw)
			}
		}
	}

	taken := map[string]bool{}
	for _, a := range articles {
		links := make([]string, 0, 4)
		for _, raw := range s.conceptSlugs(a) {
			slug := resolved[raw]
			links = appendUnique(links, slug)
			incoming[slug]++
		}
		sort.Strings(links)

		slug := s.uniqueSlug(ctx, kbRepo, Slugify(stripBrackets(a.Title)), taken)
		taken[slug] = true
		out.Atoms = append(out.Atoms, PreviewAtom{
			Title: a.Title, Slug: slug, Category: a.Category,
			Importance: a.Importance, Concepts: links,
		})
	}

	// What the reader's own notes would add. Their links count for concepts that
	// already exist and for the ones this run is about to create — the build
	// counts them after writing its atoms, so a note can name something before
	// any article does.
	candidates, counts, err := s.scanVault(ctx, kbRepo)
	if err != nil {
		return nil, err
	}
	out.VaultNew, out.VaultEdits = counts.Imported, counts.Updated

	willExist := map[string]bool{}
	for _, slug := range resolved {
		willExist[slug] = true
	}
	mine, err := kbRepo.ByOrigin(ctx, vaultOrigin, 1000)
	if err != nil {
		return nil, err
	}
	type userNote struct {
		id      int64
		content string
	}
	notes := make([]userNote, 0, len(mine)+len(candidates))
	fresh := map[string]bool{}
	for _, c := range candidates {
		fresh[c.Path] = true
		id := int64(0)
		if c.Existing != nil {
			id = c.Existing.ID
		}
		notes = append(notes, userNote{id: id, content: c.Content})
	}
	for _, n := range mine {
		if fresh[n.Path] {
			continue // already queued above, with its newer content
		}
		notes = append(notes, userNote{id: n.ID, content: n.Content})
	}

	for _, n := range notes {
		for _, link := range parseWikilinks(n.content) {
			slug, err := conceptRepo.Resolve(ctx, link)
			if err != nil {
				return nil, err
			}
			concept, err := conceptRepo.Get(ctx, slug)
			switch {
			case err == nil:
				if n.id != 0 {
					counted, err := conceptRepo.HasMention(ctx, concept.Slug, n.id)
					if err != nil {
						return nil, err
					}
					if counted {
						continue // this note is already counted towards it
					}
				}
				incoming[concept.Slug]++
				names[concept.Slug] = concept.Name
			case errors.Is(err, db.ErrNotFound):
				if willExist[slug] {
					incoming[slug]++ // this run creates it, and then counts this
				}
			default:
				return nil, err
			}
		}
	}

	// Where every touched concept would land, plus the ones already queued.
	seen := map[string]bool{}
	for slug, added := range incoming {
		current := 0
		promoted := false
		if c, err := conceptRepo.Get(ctx, slug); err == nil {
			current, promoted = c.Mentions, c.NoteID != 0
			if c.Name != "" {
				names[slug] = c.Name
			}
		} else if !errors.Is(err, db.ErrNotFound) {
			return nil, err
		}
		seen[slug] = true
		if promoted {
			continue // it already has its note
		}
		entry := PreviewConcept{Slug: slug, Name: names[slug], Mentions: current, After: current + added}
		if entry.After >= s.opts.MinOccurrences {
			out.Promoting = append(out.Promoting, entry)
		} else {
			out.Pending = append(out.Pending, entry)
		}
	}
	queued, err := conceptRepo.Dangling(ctx, 500)
	if err != nil {
		return nil, err
	}
	for _, c := range queued {
		if seen[c.Slug] {
			continue
		}
		entry := PreviewConcept{Slug: c.Slug, Name: c.Name, Mentions: c.Mentions, After: c.Mentions}
		if entry.After >= s.opts.MinOccurrences {
			out.Promoting = append(out.Promoting, entry) // the queue sweep would take it
		} else {
			out.Pending = append(out.Pending, entry)
		}
	}
	sortConcepts(out.Promoting)
	sortConcepts(out.Pending)

	// Notes whose article moved on and would be rewritten.
	stale, err := artRepo.ListStaleNotes(ctx, s.opts.Limit)
	if err != nil {
		return nil, err
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
			return nil, err
		}
		if BuildAtom(a, links) != note.Content {
			out.StaleAtoms++
		}
	}
	return out, nil
}

func sortConcepts(list []PreviewConcept) {
	sort.Slice(list, func(i, j int) bool {
		if list[i].After != list[j].After {
			return list[i].After > list[j].After
		}
		return list[i].Slug < list[j].Slug
	})
}
