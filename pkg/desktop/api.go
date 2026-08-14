package desktop

import (
	"context"

	"github.com/ajramos/giznews/internal/config"
	"github.com/ajramos/giznews/internal/db"
	"github.com/ajramos/giznews/internal/llm"
)

// API is the surface exposed to the desktop frontend. The Wails app wraps an
// implementation with startup/shutdown lifecycle; the CLI uses the same
// services directly.
type API interface {
	// Sources
	ListSources(ctx context.Context) ([]*SourceDTO, error)
	AddSource(ctx context.Context, name, srcType, url, group string) (*SourceDTO, error)
	SetSourceEnabled(ctx context.Context, id int64, enabled bool) error
	DeleteSource(ctx context.Context, id int64) error

	// Articles
	ListArticles(ctx context.Context, opts ListArticlesOptions) ([]*ArticleDTO, error)
	GetArticle(ctx context.Context, id int64) (*ArticleDTO, error)
	GetArticleContent(ctx context.Context, id int64) (*ArticleDTO, error)
	SetArticleStatus(ctx context.Context, id int64, status string) error
	SetArticleImportance(ctx context.Context, id int64, importance int) error

	// Pipeline
	Fetch(ctx context.Context) (*FetchResult, error)
	Classify(ctx context.Context, limit int) (*ClassifyResult, error)
	SummarizeArticle(ctx context.Context, id int64) (*ArticleDTO, error)
	Digest(ctx context.Context) (*DigestDTO, error)

	// Knowledge graph
	KBuild(ctx context.Context) (*KBResult, error)
	KSynthesize(ctx context.Context, category string) (*KBResult, error)
	ListNotes(ctx context.Context, noteType string) ([]*NoteDTO, error)
	GetNote(ctx context.Context, id int64) (*NoteDTO, error)
	GraphNeighbors(ctx context.Context, id int64) ([]*NoteDTO, error)

	// Search
	SearchIndex(ctx context.Context) (*IndexResultDTO, error)
	Search(ctx context.Context, query string, limit int) ([]*SearchResultDTO, error)

	// Meta
	Status(ctx context.Context) (*StatusDTO, error)
}

// FetchResult reports what a fetch run ingested.
type FetchResult struct {
	NewArticles    int   `json:"new_articles"`
	Updated        int   `json:"updated"`
	SourcesFetched int   `json:"sources_fetched"`
	SourcesFailed  int   `json:"sources_failed"`
	ElapsedMs      int64 `json:"elapsed_ms"`
}

// StatusDTO describes backend health for the UI (Ollama reachable, counts…).
type StatusDTO struct {
	DBPath          string `json:"db_path"`
	VaultPath       string `json:"vault_path"`
	LLMProvider     string `json:"llm_provider"`
	LLMEnabled      bool   `json:"llm_enabled"`
	LLMReachable    bool   `json:"llm_reachable"`
	EmbeddingsModel string `json:"embeddings_model"`
	UnreadArticles  int    `json:"unread_articles"`
	TotalArticles   int    `json:"total_articles"`
	TotalNotes      int    `json:"total_notes"`
}

// App implements API over the internal services and DB. The Wails layer holds
// one instance; it is also directly testable.
type App struct {
	cfg  *config.Config
	db   *db.DB
	prov llm.Provider // optional override (tests); lazily built from config
}

// NewApp builds the desktop API backend.
func NewApp(cfg *config.Config, database *db.DB) *App {
	return &App{cfg: cfg, db: database}
}

// SetProvider overrides the LLM provider (used by tests).
func (a *App) SetProvider(p llm.Provider) { a.prov = p }

// ensure App satisfies the API contract at compile time.
var _ API = (*App)(nil)
