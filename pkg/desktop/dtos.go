// Package desktop exposes the front-end-agnostic JSON API that the Wails
// desktop app binds against. It is pure Go (no Wails/CGO dependency), mirrors
// giztui's pkg/desktop pattern, and is unit-tested on its own.
//
// It converts between internal DB models and small JSON DTOs so the frontend
// never sees raw schema types.
package desktop

import "github.com/ajramos/giznews/internal/db"

// SourceDTO is a JSON-friendly view of a news source.
type SourceDTO struct {
	ID        int64  `json:"id"`
	Name      string `json:"name"`
	Type      string `json:"type"`
	URL       string `json:"url,omitempty"`
	Group     string `json:"group,omitempty"`
	Enabled   bool   `json:"enabled"`
	LastFetch string `json:"last_fetch,omitempty"`
	// Source health: last error, last time it brought something in, and how
	// long the current problem has run. EmptyCycles/Suspect flag a feed that
	// keeps answering but brings in nothing.
	LastError           string `json:"last_error,omitempty"`
	LastOK              string `json:"last_ok,omitempty"`
	ConsecutiveFailures int    `json:"consecutive_failures,omitempty"`
	EmptyCycles         int    `json:"empty_cycles,omitempty"`
}

// ArticleDTO is a JSON-friendly view of a news article.
type ArticleDTO struct {
	ID         int64    `json:"id"`
	SourceID   int64    `json:"source_id"`
	SourceName string   `json:"source_name,omitempty"`
	URL        string   `json:"url"`
	Title      string   `json:"title"`
	Author     string   `json:"author,omitempty"`
	ContentMD  string   `json:"content_md,omitempty"`
	Summary    string   `json:"summary,omitempty"`
	Category   string   `json:"category,omitempty"`
	Tags       []string `json:"tags"`
	Importance int      `json:"importance"`
	Status     string   `json:"status"`
	Starred    bool     `json:"starred"`
	Published  string   `json:"published,omitempty"`
	FetchedAt  string   `json:"fetched_at"`
	// StorySize is how many outlets ran this story, and StorySources who they
	// are. 0/empty means nobody else covered it.
	StorySize    int      `json:"story_size,omitempty"`
	StorySources []string `json:"story_sources,omitempty"`
}

// NoteDTO is a knowledge-graph note.
type NoteDTO struct {
	ID        int64    `json:"id"`
	Type      string   `json:"type"` // atom | electron | molecule | inbox
	Title     string   `json:"title"`
	Slug      string   `json:"slug"`
	Content   string   `json:"content"`
	Tags      []string `json:"tags"`
	Wikilinks []string `json:"wikilinks"`
	CreatedAt string   `json:"created_at"`
	// Frontmatter metadata (present on atoms; empty on electrons/molecules).
	Category string `json:"category,omitempty"`
	Rating   int    `json:"rating,omitempty"`
	URL      string `json:"url,omitempty"`
	Source   string `json:"source,omitempty"`
}

// ConceptDTO is a recurring idea tracked by the knowledge graph. Promoted says
// whether it already has an Electron note; the rest are still dangling links.
type ConceptDTO struct {
	Slug      string `json:"slug"`
	Name      string `json:"name"`
	Mentions  int    `json:"mentions"`
	NoteID    int64  `json:"note_id,omitempty"`
	Promoted  bool   `json:"promoted"`
	FirstSeen string `json:"first_seen,omitempty"`
	LastSeen  string `json:"last_seen,omitempty"`
}

// MergeDTO reports what folding one concept into another changed.
type MergeDTO struct {
	NotesRelinked int  `json:"notes_relinked"`
	Mentions      int  `json:"mentions"`
	Redirected    bool `json:"redirected"`
}

// SearchResultDTO is one hit from hybrid search.
type SearchResultDTO struct {
	Kind    string  `json:"kind"` // article | note
	ID      int64   `json:"id"`
	Title   string  `json:"title"`
	Source  string  `json:"source,omitempty"`
	Snippet string  `json:"snippet"`
	Score   float64 `json:"score"`
}

// DigestThemeDTO is a theme group inside a digest.
type DigestThemeDTO struct {
	Theme    string        `json:"theme"`
	Summary  string        `json:"summary"`
	Articles []*ArticleDTO `json:"articles"`
}

// DigestDTO is the generated daily digest.
type DigestDTO struct {
	Date     string            `json:"date"`
	Overview string            `json:"overview"`
	Themes   []*DigestThemeDTO `json:"themes"`
}

// ClassifyResult reports what a classification run did.
type ClassifyResult struct {
	Classified   int      `json:"classified"`
	ByRules      int      `json:"by_rules"`
	Archived     int      `json:"archived"`
	ByLLM        int      `json:"by_llm"`
	SkippedNoLLM int      `json:"skipped_no_llm"`
	Batches      int      `json:"batches"`
	Pending      int      `json:"pending"`
	Errors       []string `json:"errors,omitempty"`
}

// ListArticlesOptions mirrors db.ListOptions in JSON-friendly form.
type ListArticlesOptions struct {
	Status          string `json:"status,omitempty"`
	Unarchived      bool   `json:"unarchived,omitempty"`
	Starred         *bool  `json:"starred,omitempty"`
	Category        string `json:"category,omitempty"`
	SourceID        int64  `json:"source_id,omitempty"`
	Group           string `json:"group,omitempty"`
	ImportanceMin   int    `json:"importance_min,omitempty"`
	ImportanceExact *int   `json:"importance_exact,omitempty"`
	Unclassified    bool   `json:"unclassified,omitempty"`
	Summarized      bool   `json:"summarized,omitempty"`
	Query           string `json:"query,omitempty"`
	Limit           int    `json:"limit,omitempty"`
	Offset          int    `json:"offset,omitempty"`
}

func toArticleDTO(a *db.Article) *ArticleDTO {
	if a == nil {
		return nil
	}
	return &ArticleDTO{
		ID: a.ID, SourceID: a.SourceID, SourceName: a.SourceName, URL: a.URL,
		Title: a.Title, Author: a.Author, ContentMD: a.ContentMD, Summary: a.Summary,
		Category: a.Category, Tags: a.Tags, Importance: a.Importance,
		Status: string(a.Status), Starred: a.Starred, Published: a.Published, FetchedAt: a.FetchedAt,
		StorySize: a.StorySize, StorySources: a.StorySources,
	}
}

func toSourceDTO(s *db.Source) *SourceDTO {
	if s == nil {
		return nil
	}
	return &SourceDTO{
		ID: s.ID, Name: s.Name, Type: string(s.Type), URL: s.URL,
		Group: s.Group, Enabled: s.Enabled, LastFetch: s.LastFetch,
		LastError: s.LastError, LastOK: s.LastOK,
		ConsecutiveFailures: s.ConsecutiveFailures, EmptyCycles: s.EmptyCycles,
	}
}
