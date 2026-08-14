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
	Published  string   `json:"published,omitempty"`
	FetchedAt  string   `json:"fetched_at"`
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

// ListArticlesOptions mirrors db.ListOptions in JSON-friendly form.
type ListArticlesOptions struct {
	Status        string `json:"status,omitempty"`
	Category      string `json:"category,omitempty"`
	SourceID      int64  `json:"source_id,omitempty"`
	Group         string `json:"group,omitempty"`
	ImportanceMin int    `json:"importance_min,omitempty"`
	Query         string `json:"query,omitempty"`
	Limit         int    `json:"limit,omitempty"`
	Offset        int    `json:"offset,omitempty"`
}

func toArticleDTO(a *db.Article) *ArticleDTO {
	if a == nil {
		return nil
	}
	return &ArticleDTO{
		ID: a.ID, SourceID: a.SourceID, SourceName: a.SourceName, URL: a.URL,
		Title: a.Title, Author: a.Author, ContentMD: a.ContentMD, Summary: a.Summary,
		Category: a.Category, Tags: a.Tags, Importance: a.Importance,
		Status: string(a.Status), Published: a.Published, FetchedAt: a.FetchedAt,
	}
}

func toSourceDTO(s *db.Source) *SourceDTO {
	if s == nil {
		return nil
	}
	return &SourceDTO{
		ID: s.ID, Name: s.Name, Type: string(s.Type), URL: s.URL,
		Group: s.Group, Enabled: s.Enabled, LastFetch: s.LastFetch,
	}
}
