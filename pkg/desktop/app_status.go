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

	var pending int
	if err := a.db.SQL().QueryRowContext(ctx,
		"SELECT COUNT(*) FROM articles WHERE status != 'archived' AND classified = 0").Scan(&pending); err != nil {
		return nil, err
	}

	// A source is flagged once it has failed, or come up empty, enough times to
	// be worth telling the reader about — the same threshold fetch warns at. A
	// threshold of 0 turns the status-bar warning off.
	warn := a.cfg.Fetch.SourceWarnAfter
	unhealthy := 0
	if warn > 0 {
		if err := a.db.SQL().QueryRowContext(ctx,
			`SELECT COUNT(*) FROM sources
			 WHERE enabled = 1 AND hidden = 0 AND (
			   consecutive_failures >= ? OR empty_cycles >= ?)`, warn, warn).Scan(&unhealthy); err != nil {
			return nil, err
		}
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
		DBPath:           a.cfg.ResolveDBPath(),
		VaultPath:        a.cfg.ResolveVaultPath(),
		LLMProvider:      a.cfg.LLM.Provider,
		LLMEnabled:       a.cfg.LLM.Enabled,
		LLMReachable:     reachable,
		EmbeddingsModel:  a.cfg.LLM.EmbeddingModel,
		UnreadArticles:   unread,
		TotalArticles:    total,
		TotalNotes:       notes,
		PendingClassify:  pending,
		UnhealthySources: unhealthy,
	}, nil
}
