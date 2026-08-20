package db

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
)

// Theme is a cluster of notes that keep naming the same concepts together, and
// the Molecule note written for it.
//
// Seed is the concept the cluster was anchored on. It is what gives the theme
// continuity: the membership is recomputed from scratch on every build, and
// without a stored anchor a theme would land on a different slug — and so a
// different note — as soon as its concepts changed rank.
type Theme struct {
	Slug       string   `json:"slug"`
	Title      string   `json:"title"`
	Seed       string   `json:"seed"`
	Concepts   []string `json:"concepts"`
	Summary    string   `json:"summary,omitempty"`
	SummaryKey string   `json:"-"`
	NoteID     int64    `json:"note_id,omitempty"`
	FirstSeen  string   `json:"first_seen"`
	LastSeen   string   `json:"last_seen"`
}

// ThemeRepo persists the themes a build found.
type ThemeRepo struct {
	db *DB
}

// NewThemeRepo creates a theme repository.
func NewThemeRepo(db *DB) *ThemeRepo {
	return &ThemeRepo{db: db}
}

// Get returns one theme by slug.
func (r *ThemeRepo) Get(ctx context.Context, slug string) (*Theme, error) {
	row := r.db.sql.QueryRowContext(ctx, `
		SELECT slug, title, seed, concepts, summary, summary_key, note_id, created_at, updated_at
		FROM kb_themes WHERE slug = ?`, slug)
	t, err := scanTheme(row)
	if err != nil {
		return nil, err
	}
	return t, nil
}

// List returns the themes most recently seen first.
func (r *ThemeRepo) List(ctx context.Context, limit int) ([]*Theme, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := r.db.sql.QueryContext(ctx, `
		SELECT slug, title, seed, concepts, summary, summary_key, note_id, created_at, updated_at
		FROM kb_themes ORDER BY updated_at DESC, slug LIMIT ?`, limit)
	if err != nil {
		return nil, fmt.Errorf("list themes: %w", err)
	}
	defer rows.Close()
	var out []*Theme
	for rows.Next() {
		t, err := scanTheme(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// Save writes a theme, keeping the date it was first seen.
func (r *ThemeRepo) Save(ctx context.Context, t *Theme) error {
	if t.Slug == "" {
		return fmt.Errorf("theme: empty slug")
	}
	concepts, err := json.Marshal(t.Concepts)
	if err != nil {
		return fmt.Errorf("theme concepts: %w", err)
	}
	now := Now()
	var noteID any
	if t.NoteID != 0 {
		noteID = t.NoteID
	}
	_, err = r.db.sql.ExecContext(ctx, `
		INSERT INTO kb_themes (slug, title, seed, concepts, summary, summary_key, note_id, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(slug) DO UPDATE SET
			title = excluded.title,
			seed = excluded.seed,
			concepts = excluded.concepts,
			summary = excluded.summary,
			summary_key = excluded.summary_key,
			note_id = COALESCE(excluded.note_id, kb_themes.note_id),
			updated_at = excluded.updated_at`,
		t.Slug, t.Title, t.Seed, string(concepts), t.Summary, t.SummaryKey, noteID, now, now)
	if err != nil {
		return fmt.Errorf("save theme %q: %w", t.Slug, err)
	}
	return nil
}

func scanTheme(row scanner) (*Theme, error) {
	var (
		t        Theme
		concepts string
		noteID   sql.NullInt64
	)
	err := row.Scan(&t.Slug, &t.Title, &t.Seed, &concepts, &t.Summary, &t.SummaryKey, &noteID, &t.FirstSeen, &t.LastSeen)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("scan theme: %w", err)
	}
	_ = json.Unmarshal([]byte(concepts), &t.Concepts)
	t.NoteID = noteID.Int64
	return &t, nil
}
