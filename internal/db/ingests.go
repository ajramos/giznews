package db

import (
	"context"
	"fmt"
)

// IngestRepo tracks which external objects have been consumed by a pipeline
// stage (e.g. which articles already produced knowledge notes). It powers
// idempotent re-runs.
type IngestRepo struct {
	db *DB
}

// NewIngestRepo creates an ingest repository.
func NewIngestRepo(db *DB) *IngestRepo {
	return &IngestRepo{db: db}
}

// Record marks ref as ingested, linking an optional note id and status.
func (r *IngestRepo) Record(ctx context.Context, refType, refID string, noteID int64, status string) error {
	if status == "" {
		status = "processed"
	}
	_, err := r.db.sql.ExecContext(ctx, `
		INSERT INTO ingests (ref_type, ref_id, note_id, status, created_at)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT (ref_type, ref_id) DO UPDATE SET
			note_id = excluded.note_id, status = excluded.status`,
		refType, refID, noteID, status, Now())
	if err != nil {
		return fmt.Errorf("record ingest: %w", err)
	}
	return nil
}

// Exists reports whether ref was already ingested.
func (r *IngestRepo) Exists(ctx context.Context, refType, refID string) (bool, error) {
	var n int
	err := r.db.sql.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM ingests WHERE ref_type = ? AND ref_id = ?", refType, refID).Scan(&n)
	if err != nil {
		return false, fmt.Errorf("ingest exists: %w", err)
	}
	return n > 0, nil
}

// NoteID returns the note linked to an ingested ref, or 0.
func (r *IngestRepo) NoteID(ctx context.Context, refType, refID string) (int64, error) {
	var noteID int64
	err := r.db.sql.QueryRowContext(ctx,
		"SELECT COALESCE(note_id, 0) FROM ingests WHERE ref_type = ? AND ref_id = ?", refType, refID).Scan(&noteID)
	if err != nil {
		return 0, err
	}
	return noteID, nil
}
