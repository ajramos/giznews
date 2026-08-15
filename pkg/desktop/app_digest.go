package desktop

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/ajramos/giznews/internal/db"
)

// DigestMeta is a light summary of a stored digest (for the history list).
type DigestMeta struct {
	Date     string `json:"date"`
	Overview string `json:"overview"`
}

// ListDigests returns the saved digests, newest first.
func (a *App) ListDigests(ctx context.Context) ([]*DigestMeta, error) {
	rows, err := db.NewDigestRepo(a.db).List(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]*DigestMeta, 0, len(rows))
	for _, r := range rows {
		out = append(out, &DigestMeta{Date: r.Date, Overview: r.Overview})
	}
	return out, nil
}

// GetDigest returns a previously stored digest by date.
func (a *App) GetDigest(ctx context.Context, date string) (*DigestDTO, error) {
	row, err := db.NewDigestRepo(a.db).Get(ctx, date)
	if err != nil {
		if errors.Is(err, db.ErrNotFound) {
			return nil, nil
		}
		return nil, err
	}
	var themes []*DigestThemeDTO
	if row.Themes != "" {
		if err := json.Unmarshal([]byte(row.Themes), &themes); err != nil {
			return nil, err
		}
	}
	return &DigestDTO{Date: row.Date, Overview: row.Overview, Themes: themes}, nil
}
