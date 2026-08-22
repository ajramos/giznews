package db

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
)

// ErrNotFound is returned when a lookup key does not exist.
var ErrNotFound = errors.New("not found")

// SourceRepo provides CRUD over the sources table.
type SourceRepo struct {
	db *DB
}

// NewSourceRepo creates a source repository.
func NewSourceRepo(db *DB) *SourceRepo {
	return &SourceRepo{db: db}
}

// Create inserts a new source and returns it with its ID.
func (r *SourceRepo) Create(ctx context.Context, ns NewSource) (*Source, error) {
	now := Now()
	res, err := r.db.sql.ExecContext(ctx, `
		INSERT INTO sources (name, type, url, params, group_name, enabled, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		ns.Name, string(ns.Type), ns.URL, ns.Params, ns.Group, boolToInt(ns.Enabled), now, now)
	if err != nil {
		return nil, fmt.Errorf("insert source: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("source last insert id: %w", err)
	}
	return r.Get(ctx, id)
}

// sourceColumns is the columns a source row is read back with.
const sourceColumns = `
	id, name, type, url, params, group_name, enabled, last_fetch,
	last_error, last_ok, consecutive_failures, empty_cycles, created_at, updated_at`

// Get returns a source by id.
func (r *SourceRepo) Get(ctx context.Context, id int64) (*Source, error) {
	row := r.db.sql.QueryRowContext(ctx,
		"SELECT "+sourceColumns+" FROM sources WHERE id = ?", id)
	return scanSource(row)
}

// GetByName returns a source by its unique name.
func (r *SourceRepo) GetByName(ctx context.Context, name string) (*Source, error) {
	row := r.db.sql.QueryRowContext(ctx,
		"SELECT "+sourceColumns+" FROM sources WHERE name = ?", name)
	return scanSource(row)
}

// List returns all non-hidden sources, ordered by group then name.
func (r *SourceRepo) List(ctx context.Context) ([]*Source, error) {
	rows, err := r.db.sql.QueryContext(ctx,
		"SELECT "+sourceColumns+" FROM sources WHERE hidden = 0 ORDER BY group_name, name")
	if err != nil {
		return nil, fmt.Errorf("list sources: %w", err)
	}
	defer rows.Close()

	var out []*Source
	for rows.Next() {
		s, err := scanSource(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// SetHidden soft-deletes a source (keeps its articles for history).
func (r *SourceRepo) SetHidden(ctx context.Context, id int64, hidden bool) error {
	res, err := r.db.sql.ExecContext(ctx,
		"UPDATE sources SET hidden = ?, updated_at = ? WHERE id = ?", boolToInt(hidden), Now(), id)
	if err != nil {
		return fmt.Errorf("set source hidden: %w", err)
	}
	return checkAffected(res, "set source hidden")
}

// ListEnabled returns enabled sources.
func (r *SourceRepo) ListEnabled(ctx context.Context) ([]*Source, error) {
	all, err := r.List(ctx)
	if err != nil {
		return nil, err
	}
	var out []*Source
	for _, s := range all {
		if s.Enabled {
			out = append(out, s)
		}
	}
	return out, nil
}

// Update persists changes to an existing source.
func (r *SourceRepo) Update(ctx context.Context, s *Source) error {
	now := Now()
	res, err := r.db.sql.ExecContext(ctx, `
		UPDATE sources
		SET name = ?, type = ?, url = ?, params = ?, group_name = ?, enabled = ?, updated_at = ?
		WHERE id = ?`,
		s.Name, string(s.Type), s.URL, s.Params, s.Group, boolToInt(s.Enabled), now, s.ID)
	if err != nil {
		return fmt.Errorf("update source: %w", err)
	}
	return checkAffected(res, "update source")
}

// MarkSourceOK records a fetch that brought something in: the streak is over.
func (r *SourceRepo) MarkSourceOK(ctx context.Context, id int64) error {
	now := Now()
	_, err := r.db.sql.ExecContext(ctx, `
		UPDATE sources SET
			last_fetch = ?, last_ok = ?, last_error = '',
			consecutive_failures = 0, empty_cycles = 0, updated_at = ?
		WHERE id = ?`, now, now, now, id)
	if err != nil {
		return fmt.Errorf("mark source ok: %w", err)
	}
	return nil
}

// MarkSourceFailure records a failed fetch and returns how many have happened
// in a row, so the caller can say something the first time it crosses a
// threshold.
func (r *SourceRepo) MarkSourceFailure(ctx context.Context, id int64, message string) (int, error) {
	now := Now()
	_, err := r.db.sql.ExecContext(ctx, `
		UPDATE sources SET
			last_fetch = ?, last_error = ?,
			consecutive_failures = consecutive_failures + 1, empty_cycles = 0, updated_at = ?
		WHERE id = ?`, now, message, now, id)
	if err != nil {
		return 0, fmt.Errorf("mark source failure: %w", err)
	}
	return r.failures(ctx, id)
}

// MarkSourceEmpty records a fetch that succeeded but returned no items, and
// flags the source as suspect once it has been empty for suspectAfter cycles
// (suspectAfter <= 0 never flags it). It returns the running empty-cycle count
// so the caller can warn on the crossing.
func (r *SourceRepo) MarkSourceEmpty(ctx context.Context, id int64, suspectAfter int) (int, error) {
	now := Now()
	_, err := r.db.sql.ExecContext(ctx, `
		UPDATE sources SET
			last_fetch = ?, empty_cycles = empty_cycles + 1, consecutive_failures = 0,
			last_error = CASE WHEN ? > 0 AND empty_cycles + 1 >= ? THEN ? ELSE '' END,
			updated_at = ?
		WHERE id = ?`, now, suspectAfter, suspectAfter, "returned no items for several fetch cycles", now, id)
	if err != nil {
		return 0, fmt.Errorf("mark source empty: %w", err)
	}
	return r.emptyCycles(ctx, id)
}

func (r *SourceRepo) failures(ctx context.Context, id int64) (int, error) {
	var n int
	if err := r.db.sql.QueryRowContext(ctx,
		"SELECT consecutive_failures FROM sources WHERE id = ?", id).Scan(&n); err != nil {
		return 0, err
	}
	return n, nil
}

func (r *SourceRepo) emptyCycles(ctx context.Context, id int64) (int, error) {
	var n int
	if err := r.db.sql.QueryRowContext(ctx,
		"SELECT empty_cycles FROM sources WHERE id = ?", id).Scan(&n); err != nil {
		return 0, err
	}
	return n, nil
}

// SetEnabled toggles a source on/off.
func (r *SourceRepo) SetEnabled(ctx context.Context, id int64, enabled bool) error {
	res, err := r.db.sql.ExecContext(ctx,
		"UPDATE sources SET enabled = ?, updated_at = ? WHERE id = ?", boolToInt(enabled), Now(), id)
	if err != nil {
		return fmt.Errorf("set enabled: %w", err)
	}
	return checkAffected(res, "set enabled")
}

// Delete removes a source. Articles from it are kept (orphans) for history.
func (r *SourceRepo) Delete(ctx context.Context, id int64) error {
	res, err := r.db.sql.ExecContext(ctx, "DELETE FROM sources WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("delete source: %w", err)
	}
	return checkAffected(res, "delete source")
}

type scanner interface {
	Scan(dest ...any) error
}

func scanSource(row scanner) (*Source, error) {
	var (
		s          Source
		enabled    int
		lastFetch  sql.NullString
		lastError  string
		lastOK     sql.NullString
		failures   int
		emptyCyles int
	)
	if err := row.Scan(&s.ID, &s.Name, &s.Type, &s.URL, &s.Params, &s.Group, &enabled, &lastFetch,
		&lastError, &lastOK, &failures, &emptyCyles, &s.CreatedAt, &s.UpdatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("scan source: %w", err)
	}
	s.Enabled = enabled != 0
	if lastFetch.Valid {
		s.LastFetch = lastFetch.String
	}
	s.LastError = lastError
	if lastOK.Valid {
		s.LastOK = lastOK.String
	}
	s.ConsecutiveFailures = failures
	s.EmptyCycles = emptyCyles
	return &s, nil
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

func checkAffected(res sql.Result, op string) error {
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// marshalTags/Entities keep JSON columns as raw JSON strings.
func marshalTags(tags []string) string {
	b, _ := json.Marshal(tags)
	return string(b)
}

func marshalEntities(entities []Entity) string {
	b, _ := json.Marshal(entities)
	return string(b)
}
