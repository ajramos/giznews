package desktop

import (
	"context"
	"fmt"

	"github.com/ajramos/giznews/internal/db"
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

func (a *App) SetArticleStatus(ctx context.Context, id int64, status string) error {
	return db.NewArticleRepo(a.db).SetStatus(ctx, id, db.ArticleStatus(status))
}

func (a *App) SetArticleImportance(ctx context.Context, id int64, importance int) error {
	return db.NewArticleRepo(a.db).SetImportance(ctx, id, importance)
}
