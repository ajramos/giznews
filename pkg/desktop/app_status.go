package desktop

import (
	"context"

	"github.com/ajramos/giznews/internal/db"
)

// Status reports backend health for the UI.
func (a *App) Status(ctx context.Context) (*StatusDTO, error) {
	unread, err := db.NewArticleRepo(a.db).Count(ctx, db.StatusUnread)
	if err != nil {
		return nil, err
	}
	total, err := db.NewArticleRepo(a.db).Count(ctx, "")
	if err != nil {
		return nil, err
	}
	notes, err := db.NewKBRepo(a.db).Count(ctx, "")
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
		TotalNotes:      notes,
	}, nil
}
