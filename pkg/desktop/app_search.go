package desktop

import (
	"context"
	"fmt"

	"github.com/ajramos/giznews/internal/search"
)

// searchService builds the search service from config. A nil provider is
// allowed (keyword-only).
func (a *App) searchService() (*search.Service, error) {
	prov, err := a.provider()
	if err != nil {
		return nil, err
	}
	return search.NewService(a.db, prov, search.Options{
		Model: a.cfg.LLM.EmbeddingModel,
	}, a.logger())
}

// IndexResultDTO is the desktop view of a search-index run.
type IndexResultDTO struct {
	NotesEmbedded    int `json:"notes_embedded"`
	ArticlesEmbedded int `json:"articles_embedded"`
	FTSNotes         int `json:"fts_notes"`
	FTSArticles      int `json:"fts_articles"`
	EmbeddingsFailed int `json:"embeddings_failed"`
}

// SearchIndex rebuilds the FTS index and computes missing embeddings.
func (a *App) SearchIndex(ctx context.Context) (*IndexResultDTO, error) {
	var result *IndexResultDTO
	err := a.trackJob(ctx, "Index search", "index", func(jctx context.Context, p *jobProgress) error {
		svc, err := a.searchService()
		if err != nil {
			return err
		}
		res, err := svc.Index(jctx)
		if err != nil {
			return err
		}
		result = &IndexResultDTO{
			NotesEmbedded:    res.NotesEmbedded,
			ArticlesEmbedded: res.ArticlesEmbedded,
			FTSNotes:         res.FTSNotes,
			FTSArticles:      res.FTSArticles,
			EmbeddingsFailed: res.EmbeddingsFailed,
		}
		p.Message(fmt.Sprintf("%d notes · %d articles embedded", res.NotesEmbedded, res.ArticlesEmbedded))
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

// Search runs hybrid retrieval over notes and articles.
func (a *App) Search(ctx context.Context, query string, limit int) ([]*SearchResultDTO, error) {
	svc, err := a.searchService()
	if err != nil {
		return nil, err
	}
	results, err := svc.Search(ctx, query, limit)
	if err != nil {
		return nil, err
	}
	out := make([]*SearchResultDTO, 0, len(results))
	for _, r := range results {
		out = append(out, &SearchResultDTO{
			Kind: r.Kind, ID: r.ID, Title: r.Title, Source: r.Source, Snippet: r.Snippet, Score: r.Score,
		})
	}
	return out, nil
}
