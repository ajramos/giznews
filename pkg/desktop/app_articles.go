package desktop

import (
	"context"
	"fmt"
	"time"

	"github.com/ajramos/giznews/internal/db"
	"github.com/ajramos/giznews/internal/sources"
	"github.com/go-shiori/go-readability"
)

func (a *App) ListArticles(ctx context.Context, opts ListArticlesOptions) ([]*ArticleDTO, error) {
	limit, offset := opts.Limit, opts.Offset
	if limit <= 0 {
		limit = 200
	}
	articles, err := db.NewArticleRepo(a.db).List(ctx, db.ListOptions{
		Status:        db.ArticleStatus(opts.Status),
		Category:      opts.Category,
		SourceID:      opts.SourceID,
		Group:         opts.Group,
		ImportanceMin: opts.ImportanceMin,
		Query:         opts.Query,
		Limit:         limit,
		Offset:        offset,
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

func (a *App) GetArticle(ctx context.Context, id int64) (*ArticleDTO, error) {
	art, err := db.NewArticleRepo(a.db).Get(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("get article: %w", err)
	}
	return toArticleDTO(art), nil
}

// contentMinLength is the threshold below which an article is considered to
// have no real body (HN feeds only carry titles + comment links).
const contentMinLength = 200

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

	if len([]rune(art.ContentMD)) < contentMinLength && art.URL != "" {
		if err := a.extractAndStore(ctx, art); err != nil {
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
	return toArticleDTO(art), nil
}

func (a *App) extractAndStore(ctx context.Context, art *db.Article) error {
	parsed, err := readability.FromURL(art.URL, 15*time.Second)
	if err != nil {
		return fmt.Errorf("extract %s: %w", art.URL, err)
	}
	md := sources.HTMLToMarkdown(parsed.Content)
	if len([]rune(md)) < contentMinLength {
		return fmt.Errorf("extract %s: no readable content", art.URL)
	}
	return db.NewArticleRepo(a.db).SetContent(ctx, art.ID, parsed.Content, md)
}

func (a *App) SetArticleStatus(ctx context.Context, id int64, status string) error {
	return db.NewArticleRepo(a.db).SetStatus(ctx, id, db.ArticleStatus(status))
}

func (a *App) SetArticleImportance(ctx context.Context, id int64, importance int) error {
	return db.NewArticleRepo(a.db).SetImportance(ctx, id, importance)
}
