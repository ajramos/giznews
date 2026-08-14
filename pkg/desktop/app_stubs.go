package desktop

import (
	"context"
	"fmt"

	"github.com/ajramos/giznews/internal/db"
)

// Semantic search lands in its owning phase (phase 5).

func (a *App) Search(ctx context.Context, query string, limit int) ([]*SearchResultDTO, error) {
	return nil, fmt.Errorf("semantic search not implemented yet (phase 5)")
}

func (a *App) Status(ctx context.Context) (*StatusDTO, error) {
	unread, err := db.NewArticleRepo(a.db).Count(ctx, db.StatusUnread)
	if err != nil {
		return nil, err
	}
	total, err := db.NewArticleRepo(a.db).Count(ctx, "")
	if err != nil {
		return nil, err
	}
	return &StatusDTO{
		DBPath:          a.cfg.ResolveDBPath(),
		VaultPath:       a.cfg.ResolveVaultPath(),
		LLMProvider:     a.cfg.LLM.Provider,
		LLMEnabled:      a.cfg.LLM.Enabled,
		EmbeddingsModel: a.cfg.LLM.EmbeddingModel,
		UnreadArticles:  unread,
		TotalArticles:   total,
		TotalNotes:      0,
	}, nil
}
