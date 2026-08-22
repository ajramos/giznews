package digest

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/ajramos/giznews/internal/db"
)

// A digest is generated once and read later, possibly by a different program:
// the desktop app writes it, the CLI exports it, and neither should have to
// know how the other stored it.

// storedTheme is a theme as it sits in the database. The desktop app writes the
// key "theme" and this package's own type calls it "name"; both are read, so a
// digest exported today can be one the app generated months ago.
type storedTheme struct {
	Name     string        `json:"name,omitempty"`
	Theme    string        `json:"theme,omitempty"`
	Summary  string        `json:"summary,omitempty"`
	Articles []*db.Article `json:"articles"`
}

// Save persists a digest under its date, so it can be exported, re-read or
// mailed later without generating it again.
func Save(ctx context.Context, database *db.DB, d *Digest) error {
	themes := make([]storedTheme, 0, len(d.Themes))
	for _, th := range d.Themes {
		themes = append(themes, storedTheme{Name: th.Name, Summary: th.Summary, Articles: th.Articles})
	}
	blob, err := json.Marshal(themes)
	if err != nil {
		return fmt.Errorf("digest: encode themes: %w", err)
	}
	return db.NewDigestRepo(database).Save(ctx, d.Date, d.Overview, string(blob))
}

// Load reads a stored digest by date. A date nobody wrote a digest for is
// db.ErrNotFound, never an empty digest: exporting one has to fail loudly
// rather than write an empty file.
func Load(ctx context.Context, database *db.DB, date string) (*Digest, error) {
	row, err := db.NewDigestRepo(database).Get(ctx, date)
	if err != nil {
		return nil, err
	}
	var stored []storedTheme
	if row.Themes != "" {
		if err := json.Unmarshal([]byte(row.Themes), &stored); err != nil {
			return nil, fmt.Errorf("digest %s: stored themes are unreadable: %w", date, err)
		}
	}
	d := &Digest{Date: row.Date, Overview: row.Overview}
	for _, th := range stored {
		name := th.Name
		if name == "" {
			name = th.Theme
		}
		d.Themes = append(d.Themes, &Theme{Name: name, Summary: th.Summary, Articles: th.Articles})
	}
	return d, nil
}
