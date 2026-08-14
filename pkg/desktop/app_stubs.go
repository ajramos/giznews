package desktop

import (
	"context"
	"fmt"

	"github.com/ajramos/giznews/internal/db"
)

// Pipeline, knowledge-graph and search methods land in their owning phases.
// They exist now to satisfy the API contract and prove the plumbing.

func (a *App) Fetch(ctx context.Context) (*FetchResult, error) {
	return nil, fmt.Errorf("fetch not implemented yet (phase 2)")
}

func (a *App) Digest(ctx context.Context) (*DigestDTO, error) {
	return nil, fmt.Errorf("digest not implemented yet (phase 3)")
}

func (a *App) ListNotes(ctx context.Context, noteType string) ([]*NoteDTO, error) {
	return nil, fmt.Errorf("knowledge graph not implemented yet (phase 4)")
}

func (a *App) GetNote(ctx context.Context, id int64) (*NoteDTO, error) {
	return nil, fmt.Errorf("knowledge graph not implemented yet (phase 4)")
}

func (a *App) GraphNeighbors(ctx context.Context, id int64) ([]*NoteDTO, error) {
	return nil, fmt.Errorf("knowledge graph not implemented yet (phase 4)")
}

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
