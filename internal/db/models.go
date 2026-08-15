package db

// SourceType identifies the feed adapter used for a source.
type SourceType string

const (
	SourceRSS        SourceType = "rss"
	SourceHackerNews SourceType = "hackernews"
	SourceArxiv      SourceType = "arxiv"
	SourceGmail      SourceType = "gmail"
	SourceManual     SourceType = "manual"
)

// Source is a news feed tracked by giznews.
type Source struct {
	ID        int64      `json:"id"`
	Name      string     `json:"name"`
	Type      SourceType `json:"type"`
	URL       string     `json:"url,omitempty"`
	Params    string     `json:"params,omitempty"` // JSON map, e.g. gmail query
	Group     string     `json:"group,omitempty"`
	Enabled   bool       `json:"enabled"`
	LastFetch string     `json:"last_fetch,omitempty"` // RFC3339
	CreatedAt string     `json:"created_at"`
	UpdatedAt string     `json:"updated_at"`
}

// ArticleStatus is the triage state of an article.
type ArticleStatus string

const (
	StatusUnread   ArticleStatus = "unread"
	StatusRead     ArticleStatus = "read"
	StatusArchived ArticleStatus = "archived"
	StatusStarred  ArticleStatus = "starred"
)

// Entity is a named entity extracted from an article (person, org, product…).
type Entity struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

// Article is a normalized news item.
type Article struct {
	ID          int64         `json:"id"`
	SourceID    int64         `json:"source_id"`
	SourceName  string        `json:"source_name,omitempty"` // joined, not stored
	GUID        string        `json:"guid"`
	URL         string        `json:"url"`
	Title       string        `json:"title"`
	Author      string        `json:"author,omitempty"`
	ContentHTML string        `json:"content_html,omitempty"`
	ContentMD   string        `json:"content_md,omitempty"`
	Summary     string        `json:"summary,omitempty"`
	Category    string        `json:"category,omitempty"`
	Tags        []string      `json:"tags"`
	Entities    []Entity      `json:"entities"`
	Importance  int           `json:"importance"` // 0..3
	SimHash     uint64        `json:"simhash,omitempty"`
	Status      ArticleStatus `json:"status"`
	Published   string        `json:"published,omitempty"` // RFC3339
	FetchedAt   string        `json:"fetched_at"`
	UpdatedAt   string        `json:"updated_at"`
}

// NewSource is the input when creating or updating a source.
type NewSource struct {
	Name    string
	Type    SourceType
	URL     string
	Params  string
	Group   string
	Enabled bool
}
