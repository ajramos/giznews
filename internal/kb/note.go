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
	b.WriteString("type: " + f.Type + "\n")
	b.WriteString("created: " + f.Created + "\n")
	if f.Status != "" {
		b.WriteString("status: " + f.Status + "\n")
	}
	if f.Source != "" {
		b.WriteString("source: " + strings.ReplaceAll(f.Source, "\n", " ") + "\n")
	}
	if f.URL != "" {
		b.WriteString("url: " + f.URL + "\n")
	}
	if f.Rating != "" {
		b.WriteString("rating: " + f.Rating + "\n")
	}
	if f.Category != "" {
		b.WriteString("category: " + f.Category + "\n")
	}
	if len(f.Tags) > 0 {
		b.WriteString("tags:\n")
		for _, t := range f.Tags {
			b.WriteString("  - " + t + "\n")
		}
	}
	b.WriteString("---\n")
	return b.String()
}

// atomRef is a link from an electron back to an atom.
type atomRef struct {
	Slug  string
	Title string
}

// BuildAtom produces the Atom note for an article.
// concepts are the electron slugs this atom links out to.
func BuildAtom(a *db.Article, concepts []string) string {
	now := time.Now().Format("2006-01-02 15:04")
	fm := frontmatter{
		Type:     "atom",
		Created:  now,
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
	now := time.Now().Format("2006-01-02 15:04")
	fm := frontmatter{
		Type:    "electron",
		Created: now,
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
	now := time.Now().Format("2006-01-02 15:04")
	fm := frontmatter{
		Type:    "molecule",
		Created: now,
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

func excerpt(s string, n int) string {
	r := []rune(strings.TrimSpace(s))
	if len(r) <= n {
		return string(r)
	}
	return string(r[:n]) + " …"
}
