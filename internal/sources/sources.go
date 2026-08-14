// Package sources adapts external news providers (RSS, Hacker News, arXiv,
// Gmail newsletters) into a single normalized Item shape consumed by the fetch
// pipeline.
package sources

import (
	"context"
	"time"

	"github.com/ajramos/giznews/internal/db"
)

// Item is a normalized article as returned by a fetcher. It intentionally does
// not carry DB concerns; the pipeline decides persistence.
type Item struct {
	GUID        string    `json:"guid"`
	URL         string    `json:"url"`
	Title       string    `json:"title"`
	Author      string    `json:"author,omitempty"`
	ContentHTML string    `json:"content_html,omitempty"`
	ContentMD   string    `json:"content_md,omitempty"`
	Published   time.Time `json:"published"`
}

// Fetcher retrieves items from one provider. Implementations must be safe for
// concurrent use.
type Fetcher interface {
	Fetch(ctx context.Context) ([]*Item, error)
}

// Manager dispatches a db.Source to the right fetcher based on its type.
type Manager struct {
	cfg *configConfig
}

type configConfig struct {
	// Gmail settings needed by the gmail fetcher.
	CredentialsPath string
	TokenPath       string
	Queries         []string
	MaxAge          string
}

// NewManager builds a source Manager from the app config.
func NewManager(credsPath, tokenPath string, queries []string, maxAge string) *Manager {
	return &Manager{cfg: &configConfig{
		CredentialsPath: credsPath,
		TokenPath:       tokenPath,
		Queries:         queries,
		MaxAge:          maxAge,
	}}
}

// Fetch runs the fetcher matching src.Type.
func (m *Manager) Fetch(ctx context.Context, src *db.Source) ([]*Item, error) {
	switch src.Type {
	case db.SourceRSS:
		return NewRSSFetcher(src.URL, src.Params).Fetch(ctx)
	case db.SourceArxiv:
		return NewArxivFetcher(src.URL).Fetch(ctx)
	case db.SourceHackerNews:
		return NewHackerNewsFetcher(src.Params).Fetch(ctx)
	case db.SourceGmail:
		return NewGmailFetcher(m.cfg, src.Params).Fetch(ctx)
	default:
		return nil, &TypeError{Type: string(src.Type)}
	}
}

// TypeError reports an unknown source type.
type TypeError struct{ Type string }

func (e *TypeError) Error() string { return "unknown source type: " + e.Type }
