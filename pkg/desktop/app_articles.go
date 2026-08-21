package desktop

import (
	"context"
	"fmt"

	"github.com/ajramos/giznews/internal/db"
	"github.com/ajramos/giznews/internal/extract"
)

// contentMinLength is the threshold below which an article is considered to
// have no real body (HN feeds only carry titles + comment links). Kept as an
// alias of extract.MinLength for readability at call sites.
const contentMinLength = extract.MinLength

func (a *App) ListArticles(ctx context.Context, opts ListArticlesOptions) ([]*ArticleDTO, error) {	limit, offset := opts.Limit, opts.Offset
	if limit <= 0 {
		limit = 200
	}
	articles, err := db.NewArticleRepo(a.db).List(ctx, db.ListOptions{
		Status:          db.ArticleStatus(opts.Status),
		Unarchived:      opts.Unarchived,
		Starred:         opts.Starred,
		Category:        opts.Category,
		SourceID:        opts.SourceID,
		Group:           opts.Group,
		ImportanceMin:   opts.ImportanceMin,
		ImportanceExact: opts.ImportanceExact,
		Unclassified:    opts.Unclassified,
		Summarized:      opts.Summarized,
		Query:           opts.Query,
		Limit:           limit,
		Offset:          offset,
	})
	if err != nil {
		return nil, fmt.Errorf("list articles: %w", err)
	}
	out := make([]*ArticleDTO, 0, len(articles))
	for _, art := range articles {
		dto := toArticleDTO(art)
		// Keep the list payload light: full bodies are fetched on demand via
		// GetArticle when the reader opens an article.
		dto.ContentMD = ""
		out = append(out, dto)
	}
	return out, nil
}

// ListInbox returns the articles awaiting processing (no knowledge note yet) —
// the vault "inbox" stage of the Zettelkasten flow.
func (a *App) ListInbox(ctx context.Context, limit int) ([]*ArticleDTO, error) {
	pending, err := db.NewArticleRepo(a.db).ListPending(ctx, limit)
	if err != nil {
		return nil, fmt.Errorf("list inbox: %w", err)
	}
	out := make([]*ArticleDTO, 0, len(pending))
	for _, art := range pending {
		dto := toArticleDTO(art)
		dto.ContentMD = ""
		out = append(out, dto)
	}
	return out, nil
}

func (a *App) GetArticle(ctx context.Context, id int64) (*ArticleDTO, error) {
	art, err := db.NewArticleRepo(a.db).Get(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("get article: %w", err)
	}
	return toArticleDTO(art), nil
}

// GetArticleContent returns an article ready to read: if its stored body is
// too short, it fetches the original URL and extracts the readable content
// (readability → markdown), caching it for next time. Extraction happens on
// demand so opening an article is fast on first read and instant afterwards.
func (a *App) GetArticleContent(ctx context.Context, id int64) (*ArticleDTO, error) {
	repo := db.NewArticleRepo(a.db)
	art, err := repo.Get(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("get article: %w", err)
	}

	if len([]rune(art.ContentMD)) < extract.MinLength && art.URL != "" {
		if err := extract.NewService(a.db).ExtractArticle(ctx, art); err != nil {
			// Return what we have; extraction failure is non-fatal.
			updated, _ := repo.Get(ctx, id)
			if updated != nil {
				return toArticleDTO(updated), nil
			}
		}
		art, err = repo.Get(ctx, id)
		if err != nil {
			return nil, err
		}
	}
	// Clean stored content on read so previously-extracted articles also get
	// the HTML-fence unwrapping without a re-extraction.
	art.ContentMD = extract.CleanMarkdown(art.ContentMD)
	return toArticleDTO(art), nil
}

func (a *App) SetArticleStatus(ctx context.Context, id int64, status string) error {
	return db.NewArticleRepo(a.db).SetStatus(ctx, id, db.ArticleStatus(status), db.ActorUser)
}

func (a *App) SetArticleStarred(ctx context.Context, id int64, starred bool) error {
	return db.NewArticleRepo(a.db).SetStarred(ctx, id, starred)
}

func (a *App) SetArticleImportance(ctx context.Context, id int64, importance int) error {
	return db.NewArticleRepo(a.db).SetImportance(ctx, id, importance)
}
