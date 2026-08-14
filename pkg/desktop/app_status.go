package desktop

import (
	"context"
	"time"

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

	reachable := false
	if a.cfg.LLM.Enabled {
		prov, err := a.provider()
		if err == nil && prov != nil {
			pingCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
			reachable = prov.Ping(pingCtx) == nil
			cancel()
		}
	}

	return &StatusDTO{
		DBPath:          a.cfg.ResolveDBPath(),
		VaultPath:       a.cfg.ResolveVaultPath(),
		LLMProvider:     a.cfg.LLM.Provider,
		LLMEnabled:      a.cfg.LLM.Enabled,
		LLMReachable:    reachable,
		EmbeddingsModel: a.cfg.LLM.EmbeddingModel,
		UnreadArticles:  unread,
		TotalArticles:   total,
		TotalNotes:      notes,
	}, nil
}
