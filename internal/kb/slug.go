// Package kb builds and maintains the Zettelkasten knowledge graph: articles
// become Atom notes, recurring concepts become Electron notes, and curated
// syntheses become Molecules. Notes are written as Obsidian-compatible
// markdown into a dedicated vault and mirrored in SQLite for search + graph.
package kb

import (
	"regexp"
	"strings"
)

var slugCleanRe = regexp.MustCompile(`[^a-z0-9]+`)

// Slugify converts a title/concept into an Obsidian-compatible file slug.
func Slugify(title string) string {
	s := strings.ToLower(strings.TrimSpace(title))
	s = slugCleanRe.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")
	if len(s) > 80 {
		s = s[:80]
	}
	s = strings.Trim(s, "-")
	if s == "" {
		return "note"
	}
	return s
}

// stripBrackets removes [[...]] syntax from raw text (used for titles that
// accidentally contain wikilinks).
func stripBrackets(s string) string {
	s = strings.ReplaceAll(s, "[[", "")
	s = strings.ReplaceAll(s, "]]", "")
	return strings.TrimSpace(s)
}
