package kb

import (
	"fmt"
	"strings"
	"time"

	"github.com/ajramos/giznews/internal/db"
)

// starRating converts importance (0-3) to the chronicles star rating.
func starRating(importance int) string {
	if importance <= 0 {
		return "☆☆☆☆☆"
	}
	return strings.Repeat("★", importance) + strings.Repeat("☆", 5-importance)
}

// frontmatter is rendered as YAML for Obsidian.
type frontmatter struct {
	Type     string
	Created  string
	Source   string
	URL      string
	Rating   string
	Category string
	Status   string
	Tags     []string
}

func (f frontmatter) render() string {
	var b strings.Builder
	b.WriteString("---\n")
	b.WriteString("type: " + yamlValue(f.Type) + "\n")
	b.WriteString("created: " + yamlValue(f.Created) + "\n")
	if f.Status != "" {
		b.WriteString("status: " + yamlValue(f.Status) + "\n")
	}
	if f.Source != "" {
		b.WriteString("source: " + yamlValue(f.Source) + "\n")
	}
	if f.URL != "" {
		b.WriteString("url: " + yamlValue(f.URL) + "\n")
	}
	if f.Rating != "" {
		b.WriteString("rating: " + yamlValue(f.Rating) + "\n")
	}
	if f.Category != "" {
		b.WriteString("category: " + yamlValue(f.Category) + "\n")
	}
	if len(f.Tags) > 0 {
		b.WriteString("tags:\n")
		for _, t := range f.Tags {
			b.WriteString("  - " + yamlValue(t) + "\n")
		}
	}
	b.WriteString("---\n")
	return b.String()
}

// yamlSpecial are the characters that give a bare scalar a second meaning: a
// source named "Ars Technica: AI" or a tag holding a "#" silently turns the
// frontmatter into something Obsidian parses differently, or not at all.
const yamlSpecial = ":#[]{}&*!|>'\"%@`,"

// yamlValue renders a string as a YAML scalar, quoting it only when a bare one
// would be ambiguous. Newlines always collapse to spaces — a frontmatter value
// is a single line by construction.
func yamlValue(s string) string {
	s = strings.ReplaceAll(strings.ReplaceAll(s, "\r", " "), "\n", " ")
	if s == "" {
		return `""`
	}
	needsQuote := strings.ContainsAny(s, yamlSpecial) ||
		s != strings.TrimSpace(s) ||
		strings.HasPrefix(s, "-") ||
		strings.HasPrefix(s, "?")
	if !needsQuote {
		return s
	}
	escaped := strings.ReplaceAll(s, `\`, `\\`)
	escaped = strings.ReplaceAll(escaped, `"`, `\"`)
	return `"` + escaped + `"`
}

// noteTime formats a timestamp the way Obsidian reads it in frontmatter. UTC,
// like every other timestamp in the database, so a note never claims a day the
// rows behind it disagree with.
func noteTime(t time.Time) string {
	return t.UTC().Format("2006-01-02 15:04")
}

// articleTime is the moment an atom should be dated by: when the news was
// published, not when the build happened. Dating notes by build time collapses
// a backfill of months of articles onto a single day, which makes the vault's
// timeline useless.
func articleTime(a *db.Article) time.Time {
	for _, ts := range []string{a.Published, a.FetchedAt} {
		if ts == "" {
			continue
		}
		if t, err := time.Parse(time.RFC3339, ts); err == nil {
			return t
		}
	}
	return time.Now()
}

// atomRef is a link from an electron back to an atom.
type atomRef struct {
	Slug   string
	Title  string
	NoteID int64
}

// BuildAtom produces the Atom note for an article.
// concepts are the electron slugs this atom links out to.
func BuildAtom(a *db.Article, concepts []string) string {
	fm := frontmatter{
		Type:     "atom",
		Created:  noteTime(articleTime(a)),
		Source:   a.SourceName,
		URL:      a.URL,
		Rating:   starRating(a.Importance),
		Category: a.Category,
		Tags:     append([]string{"atom", "ai"}, a.Tags...),
	}

	var b strings.Builder
	b.WriteString(fm.render())
	b.WriteString("\n# " + a.Title + "\n\n")

	if a.Summary != "" {
		b.WriteString("## Resumen\n" + a.Summary + "\n\n")
	}
	if a.ContentMD != "" {
		b.WriteString("## Contenido destacado\n" + excerpt(a.ContentMD, 600) + "\n\n")
	}
	if len(a.Entities) > 0 {
		b.WriteString("## Entidades\n")
		for _, e := range a.Entities {
			b.WriteString("- **" + e.Name + "** (" + e.Type + ")\n")
		}
		b.WriteString("\n")
	}
	b.WriteString("## Fuente\n[" + a.SourceName + "](" + a.URL + ")\n\n")

	if len(concepts) > 0 {
		b.WriteString("## Conexiones\n")
		for _, c := range concepts {
			b.WriteString("- [[" + c + "]]\n")
		}
		b.WriteString("\n")
	}
	return b.String()
}

// BuildElectron produces an Electron note aggregating atom backlinks.
func BuildElectron(name string, sources []atomRef) string {
	fm := frontmatter{
		Type:    "electron",
		Created: noteTime(time.Now()),
		Status:  "active",
		Tags:    []string{"ai", "concept"},
	}

	var b strings.Builder
	b.WriteString(fm.render())
	b.WriteString("\n# " + name + "\n\n")
	b.WriteString(fmt.Sprintf("## Definición\nConcepto recurrente en el seguimiento de IA — referenciado en %d nota(s).\n\n", len(sources)))
	if len(sources) > 0 {
		b.WriteString("## Fuentes\n")
		for _, s := range sources {
			b.WriteString("- [[" + s.Slug + "]] — " + s.Title + "\n")
		}
		b.WriteString("\n")
	}
	return b.String()
}

// BuildMolecule produces a Molecule note synthesizing a category/theme.
func BuildMolecule(title, summary string, refs []atomRef) string {
	fm := frontmatter{
		Type:    "molecule",
		Created: noteTime(time.Now()),
		Status:  "active",
		Tags:    []string{"synthesis", "ai"},
	}

	var b strings.Builder
	b.WriteString(fm.render())
	b.WriteString("\n# 🧪 " + title + "\n\n")
	if summary != "" {
		b.WriteString("## Central Idea\n" + summary + "\n\n")
	}
	b.WriteString("## Development\n\n")
	b.WriteString("## Conexiones\n")
	for _, s := range refs {
		b.WriteString("- [[" + s.Slug + "]] — " + s.Title + "\n")
	}
	b.WriteString("\n## Aplicación\n\n")
	return b.String()
}

// BuildRedirect rewrites a merged concept's note into a pointer at the concept
// it was folded into. The file stays in the vault so links already written to
// it — in generated notes and in whatever the user wrote by hand — still land
// somewhere useful.
func BuildRedirect(title, targetName, targetSlug string) string {
	fm := frontmatter{
		Type:    "electron",
		Created: noteTime(time.Now()),
		Status:  "merged",
		Tags:    []string{"ai", "concept"},
	}
	var b strings.Builder
	b.WriteString(fm.render())
	b.WriteString("\n# " + title + "\n\n")
	b.WriteString("Merged into [[" + targetSlug + "]] — " + targetName + ".\n")
	return b.String()
}

func excerpt(s string, n int) string {
	r := []rune(strings.TrimSpace(s))
	if len(r) <= n {
		return string(r)
	}
	return string(r[:n]) + " …"
}
