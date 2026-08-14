package desktop

import (
	"context"
	"fmt"

	"github.com/ajramos/giznews/internal/db"
)

func (a *App) ListSources(ctx context.Context) ([]*SourceDTO, error) {
	sources, err := db.NewSourceRepo(a.db).List(ctx)
	if err != nil {
		return nil, fmt.Errorf("list sources: %w", err)
	}
	out := make([]*SourceDTO, 0, len(sources))
	for _, s := range sources {
		out = append(out, toSourceDTO(s))
	}
	return out, nil
}

func (a *App) AddSource(ctx context.Context, name, srcType, url, group string) (*SourceDTO, error) {
	if group == "" {
		group = "general"
	}
	s, err := db.NewSourceRepo(a.db).Create(ctx, db.NewSource{
		Name: name, Type: db.SourceType(srcType), URL: url, Group: group, Enabled: true,
	})
	if err != nil {
		return nil, fmt.Errorf("add source: %w", err)
	}
	return toSourceDTO(s), nil
}

func (a *App) SetSourceEnabled(ctx context.Context, id int64, enabled bool) error {
	return db.NewSourceRepo(a.db).SetEnabled(ctx, id, enabled)
}

// DeleteSource soft-deletes a source from the registry. Its articles are kept
// for history; the deletion is fully reversible (re-adding the source restores
// its feed without losing anything).
func (a *App) DeleteSource(ctx context.Context, id int64) error {
	return db.NewSourceRepo(a.db).SetHidden(ctx, id, true)
}
