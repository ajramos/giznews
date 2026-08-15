package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

// DigestRow is a stored daily digest.
type DigestRow struct {
	Date      string
	Overview  string
	Themes    string // raw JSON array of themes
	CreatedAt string
}

// DigestRepo persists generated digests (one per date).
type DigestRepo struct {
	db *DB
}

// NewDigestRepo builds a digest repository.
func NewDigestRepo(db *DB) *DigestRepo { return &DigestRepo{db: db} }

// Save upserts a digest for the given date.
func (r *DigestRepo) Save(ctx context.Context, date, overview, themesJSON string) error {
	_, err := r.db.sql.ExecContext(ctx, `
		INSERT INTO digests (date, overview, themes, created_at) VALUES (?, ?, ?, ?)
		ON CONFLICT(date) DO UPDATE SET overview = excluded.overview, themes = excluded.themes, created_at = excluded.created_at`,
		date, overview, themesJSON, Now())
	if err != nil {
		return fmt.Errorf("save digest: %w", err)
	}
	return nil
}

// Get returns a digest by date, or ErrNotFound.
func (r *DigestRepo) Get(ctx context.Context, date string) (*DigestRow, error) {
	row := r.db.sql.QueryRowContext(ctx,
		"SELECT date, overview, themes, created_at FROM digests WHERE date = ?", date)
	var d DigestRow
	if err := row.Scan(&d.Date, &d.Overview, &d.Themes, &d.CreatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get digest: %w", err)
	}
	return &d, nil
}

// List returns all digests, newest first.
func (r *DigestRepo) List(ctx context.Context) ([]*DigestRow, error) {
	rows, err := r.db.sql.QueryContext(ctx,
		"SELECT date, overview, themes, created_at FROM digests ORDER BY date DESC")
	if err != nil {
		return nil, fmt.Errorf("list digests: %w", err)
	}
	defer rows.Close()

	var out []*DigestRow
	for rows.Next() {
		var d DigestRow
		if err := rows.Scan(&d.Date, &d.Overview, &d.Themes, &d.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, &d)
	}
	return out, rows.Err()
}
