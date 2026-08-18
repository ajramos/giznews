package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

// Concept is a recurring idea, entity or topic seen in the news stream. It
// exists independently of the notes that mention it, so mentions accumulate
// across build runs: a concept named once a day for a week is as significant as
// one named seven times in a single run, and only the persisted count can tell.
//
// NoteID is the Electron note the concept was promoted to, or 0 while it is
// still only a dangling wikilink.
type Concept struct {
	Slug      string `json:"slug"`
	Name      string `json:"name"`
	NoteID    int64  `json:"note_id,omitempty"`
	Mentions  int    `json:"mentions"`
	FirstSeen string `json:"first_seen"`
	LastSeen  string `json:"last_seen"`
}

// ConceptRepo persists concepts and the notes mentioning them.
type ConceptRepo struct {
	db *DB
}

// NewConceptRepo creates a concept repository.
func NewConceptRepo(db *DB) *ConceptRepo {
	return &ConceptRepo{db: db}
}

// Touch records that noteID mentions the concept, creating it on first sight,
// and returns its current state. Passing noteID 0 registers the concept without
// a mention. Mentions are idempotent per note, so re-running a build never
// inflates the count.
func (r *ConceptRepo) Touch(ctx context.Context, slug, name string, noteID int64) (*Concept, error) {
	if slug == "" {
		return nil, fmt.Errorf("concept: empty slug")
	}
	if name == "" {
		name = slug
	}
	now := Now()
	// A later mention may only carry the slug as its name (the display name was
	// lost); never let that overwrite a real name.
	if _, err := r.db.sql.ExecContext(ctx, `
		INSERT INTO concepts (slug, name, mentions, first_seen, last_seen)
		VALUES (?, ?, 0, ?, ?)
		ON CONFLICT (slug) DO UPDATE SET
			name = CASE WHEN excluded.name = concepts.slug THEN concepts.name ELSE excluded.name END,
			last_seen = excluded.last_seen`,
		slug, name, now, now); err != nil {
		return nil, fmt.Errorf("upsert concept: %w", err)
	}

	if noteID != 0 {
		if _, err := r.db.sql.ExecContext(ctx, `
			INSERT OR IGNORE INTO concept_mentions (concept_slug, note_id, created_at)
			VALUES (?, ?, ?)`, slug, noteID, now); err != nil {
			return nil, fmt.Errorf("record concept mention: %w", err)
		}
		if _, err := r.db.sql.ExecContext(ctx, `
			UPDATE concepts SET mentions =
				(SELECT COUNT(*) FROM concept_mentions WHERE concept_slug = ?)
			WHERE slug = ?`, slug, slug); err != nil {
			return nil, fmt.Errorf("recount concept mentions: %w", err)
		}
	}
	return r.Get(ctx, slug)
}

// Get returns one concept, or ErrNotFound.
func (r *ConceptRepo) Get(ctx context.Context, slug string) (*Concept, error) {
	var (
		c      Concept
		noteID sql.NullInt64
	)
	err := r.db.sql.QueryRowContext(ctx,
		"SELECT slug, name, note_id, mentions, first_seen, last_seen FROM concepts WHERE slug = ?", slug).
		Scan(&c.Slug, &c.Name, &noteID, &c.Mentions, &c.FirstSeen, &c.LastSeen)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get concept: %w", err)
	}
	c.NoteID = noteID.Int64
	return &c, nil
}

// Promote links a concept to the Electron note that now represents it.
func (r *ConceptRepo) Promote(ctx context.Context, slug string, noteID int64) error {
	res, err := r.db.sql.ExecContext(ctx,
		"UPDATE concepts SET note_id = ?, last_seen = ? WHERE slug = ?", noteID, Now(), slug)
	if err != nil {
		return fmt.Errorf("promote concept: %w", err)
	}
	return checkAffected(res, "promote concept")
}

// MentionedBy returns the notes that mention the concept, newest first.
func (r *ConceptRepo) MentionedBy(ctx context.Context, slug string, limit int) ([]*KBNote, error) {
	if limit <= 0 {
		limit = 500
	}
	rows, err := r.db.sql.QueryContext(ctx, `
		SELECT n.id, n.note_type, n.title, n.slug, n.path, n.frontmatter, n.content, n.tags, n.wikilinks, n.created_at, n.updated_at
		FROM concept_mentions m
		JOIN kb_notes n ON n.id = m.note_id
		WHERE m.concept_slug = ?
		ORDER BY n.created_at DESC LIMIT ?`, slug, limit)
	if err != nil {
		return nil, fmt.Errorf("concept mentions: %w", err)
	}
	defer rows.Close()
	return scanKBNotes(rows)
}

// Top returns the most-mentioned concepts, promoted or not.
func (r *ConceptRepo) Top(ctx context.Context, limit int) ([]*Concept, error) {
	if limit <= 0 {
		limit = 50
	}
	return r.list(ctx, "SELECT slug, name, note_id, mentions, first_seen, last_seen FROM concepts ORDER BY mentions DESC, last_seen DESC LIMIT ?", limit)
}

// Dangling returns concepts that notes link to but no Electron represents yet —
// the promotion queue, and the dead links a reader sees in Obsidian.
func (r *ConceptRepo) Dangling(ctx context.Context, limit int) ([]*Concept, error) {
	if limit <= 0 {
		limit = 50
	}
	return r.list(ctx, `
		SELECT slug, name, note_id, mentions, first_seen, last_seen FROM concepts
		WHERE note_id IS NULL AND mentions > 0
		ORDER BY mentions DESC, last_seen DESC LIMIT ?`, limit)
}

func (r *ConceptRepo) list(ctx context.Context, query string, args ...any) ([]*Concept, error) {
	rows, err := r.db.sql.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list concepts: %w", err)
	}
	defer rows.Close()
	var out []*Concept
	for rows.Next() {
		var (
			c      Concept
			noteID sql.NullInt64
		)
		if err := rows.Scan(&c.Slug, &c.Name, &noteID, &c.Mentions, &c.FirstSeen, &c.LastSeen); err != nil {
			return nil, fmt.Errorf("scan concept: %w", err)
		}
		c.NoteID = noteID.Int64
		out = append(out, &c)
	}
	return out, rows.Err()
}
