package kb

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/ajramos/giznews/internal/db"
)

// IndexResult reports what a vault index run wrote.
type IndexResult struct {
	TopConcepts int `json:"top_concepts"`
	Dangling    int `json:"dangling"`
	DailyNotes  int `json:"daily_notes"`
}

// BuildIndex writes the vault's entry points: a map of the whole graph, the
// day's note, and the list of concepts still waiting for one. Hundreds of atoms
// with no way in are a pile, not a knowledge base — these three files are what
// makes the vault navigable in Obsidian.
//
// They are generated views, rewritten on every run, and deliberately not stored
// as kb_notes: an index links to everything, which would make every note a
// neighbour of every other note in the graph.
func (s *Service) BuildIndex(ctx context.Context) (*IndexResult, error) {
	res := &IndexResult{}
	kbRepo := db.NewKBRepo(s.db)
	conceptRepo := db.NewConceptRepo(s.db)

	top, err := conceptRepo.Top(ctx, 40)
	if err != nil {
		return nil, err
	}
	dangling, err := conceptRepo.Dangling(ctx, 100)
	if err != nil {
		return nil, err
	}
	res.TopConcepts = len(top)
	res.Dangling = len(dangling)

	index, err := s.renderIndex(ctx, kbRepo, top, len(dangling))
	if err != nil {
		return nil, err
	}
	// Generated views are marked like any other note, so a reader who adds a
	// paragraph to Index.md keeps it across rebuilds.
	if _, err := s.vault.Sync(SyncInput{NoteType: "map", Slug: "Index", Content: index}); err != nil {
		return nil, err
	}
	if _, err := s.vault.Sync(SyncInput{
		NoteType: "map", Slug: "Unresolved concepts", Content: s.renderDangling(dangling),
	}); err != nil {
		return nil, err
	}

	// UTC, like note timestamps and digest dates, so the day never disagrees
	// with the rows it selects.
	day := time.Now().UTC().Format("2006-01-02")
	daily, wrote, err := s.renderDaily(ctx, kbRepo, day)
	if err != nil {
		return nil, err
	}
	if wrote {
		if _, err := s.vault.Sync(SyncInput{NoteType: "inbox", Slug: day, Content: daily}); err != nil {
			return nil, err
		}
		res.DailyNotes = 1
	}
	return res, nil
}

func (s *Service) renderIndex(ctx context.Context, repo *db.KBRepo, top []*db.Concept, dangling int) (string, error) {
	var b strings.Builder
	b.WriteString(mapFrontmatter("index"))
	b.WriteString("\n# AI knowledge index\n\n")

	counts := map[db.NoteType]int{}
	for _, t := range []db.NoteType{db.NoteAtom, db.NoteElectron, db.NoteMolecule} {
		n, err := repo.Count(ctx, t)
		if err != nil {
			return "", err
		}
		counts[t] = n
	}
	b.WriteString(fmt.Sprintf("%d atoms · %d electrons · %d molecules · %d concepts waiting for one\n\n",
		counts[db.NoteAtom], counts[db.NoteElectron], counts[db.NoteMolecule], dangling))

	if len(top) > 0 {
		b.WriteString("## Most mentioned\n")
		for _, c := range top {
			line := fmt.Sprintf("- [[%s]] — %s · %d mention(s)", c.Slug, c.Name, c.Mentions)
			if c.NoteID == 0 {
				line += " *(no note yet)*"
			}
			b.WriteString(line + "\n")
		}
		b.WriteString("\n")
	}

	categories, err := repo.Categories(ctx)
	if err != nil {
		return "", err
	}
	if len(categories) > 0 {
		b.WriteString("## By category\n")
		for _, cat := range categories {
			atoms, err := repo.ByCategory(ctx, cat, 10)
			if err != nil {
				return "", err
			}
			if len(atoms) == 0 {
				continue
			}
			b.WriteString("\n### " + cat + "\n")
			for _, n := range atoms {
				b.WriteString("- [[" + n.Slug + "]] — " + n.Title + "\n")
			}
		}
		b.WriteString("\n")
	}

	molecules, err := repo.List(ctx, db.NoteMolecule, 50)
	if err != nil {
		return "", err
	}
	if len(molecules) > 0 {
		b.WriteString("## Syntheses\n")
		for _, n := range molecules {
			b.WriteString("- [[" + n.Slug + "]] — " + n.Title + "\n")
		}
		b.WriteString("\n")
	}
	return b.String(), nil
}

func (s *Service) renderDangling(concepts []*db.Concept) string {
	var b strings.Builder
	b.WriteString(mapFrontmatter("index"))
	b.WriteString("\n# Unresolved concepts\n\n")
	b.WriteString(fmt.Sprintf("Concepts the notes link to that have no Electron of their own. They get one at %d mention(s).\n\n", s.opts.MinOccurrences))
	if len(concepts) == 0 {
		b.WriteString("Nothing pending — every concept mentioned so far has a note.\n")
		return b.String()
	}
	b.WriteString("| Concept | Mentions | First seen | Last seen |\n")
	b.WriteString("| --- | --- | --- | --- |\n")
	for _, c := range concepts {
		b.WriteString(fmt.Sprintf("| [[%s]] | %d | %s | %s |\n", c.Slug, c.Mentions, dayOf(c.FirstSeen), dayOf(c.LastSeen)))
	}
	return b.String()
}

// renderDaily builds the note for one day, reporting whether there was anything
// to write: an empty day gets no note.
func (s *Service) renderDaily(ctx context.Context, repo *db.KBRepo, day string) (string, bool, error) {
	notes, err := repo.CreatedOn(ctx, day, 200)
	if err != nil {
		return "", false, err
	}
	digest, err := db.NewDigestRepo(s.db).Get(ctx, day)
	if err != nil && !errors.Is(err, db.ErrNotFound) {
		return "", false, err
	}
	if len(notes) == 0 && (digest == nil || digest.Overview == "") {
		return "", false, nil
	}

	var b strings.Builder
	b.WriteString(mapFrontmatter("daily"))
	b.WriteString("\n# " + day + "\n\n")
	if digest != nil && digest.Overview != "" {
		b.WriteString("## Digest\n" + digest.Overview + "\n\n")
	}
	sections := []struct {
		title string
		kind  db.NoteType
	}{
		{"Notes added today", db.NoteAtom},
		{"Concepts that earned a note", db.NoteElectron},
		{"Syntheses", db.NoteMolecule},
	}
	for _, sec := range sections {
		var lines []string
		for _, n := range notes {
			if n.Type == sec.kind {
				lines = append(lines, "- [["+n.Slug+"]] — "+n.Title)
			}
		}
		if len(lines) == 0 {
			continue
		}
		b.WriteString("## " + sec.title + "\n" + strings.Join(lines, "\n") + "\n\n")
	}
	return b.String(), true, nil
}

func mapFrontmatter(kind string) string {
	return frontmatter{
		Type:    kind,
		Created: noteTime(time.Now()),
		Status:  "generated",
		Tags:    []string{"ai", kind},
	}.render()
}

// dayOf trims an RFC3339 timestamp to its date.
func dayOf(ts string) string {
	if len(ts) >= 10 {
		return ts[:10]
	}
	return ts
}
