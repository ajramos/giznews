package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"regexp"
	"strings"
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

var conceptSuffixRe = regexp.MustCompile(`-concept(-\d+)?$`)

// CanonKey collapses a slug to the key that identifies a concept across
// spellings: "open-ai" and "openai" share one, and so do "gpt-5" and "gpt5".
// The "-concept" suffix the slug allocator adds when an atom already owns the
// plain slug is stripped first, so it never splits a concept in two.
func CanonKey(slug string) string {
	return strings.ReplaceAll(conceptSuffixRe.ReplaceAllString(slug, ""), "-", "")
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
		INSERT INTO concepts (slug, name, canon_key, mentions, first_seen, last_seen)
		VALUES (?, ?, ?, 0, ?, ?)
		ON CONFLICT (slug) DO UPDATE SET
			name = CASE WHEN excluded.name = concepts.slug THEN concepts.name ELSE excluded.name END,
			last_seen = excluded.last_seen`,
		slug, name, CanonKey(slug), now, now); err != nil {
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
		SELECT n.id, n.note_type, n.title, n.slug, n.path, n.frontmatter, n.content, n.tags, n.wikilinks, n.content_hash, n.created_at, n.updated_at
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

// HasMention reports whether a note is already counted towards a concept.
func (r *ConceptRepo) HasMention(ctx context.Context, slug string, noteID int64) (bool, error) {
	var n int
	err := r.db.sql.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM concept_mentions WHERE concept_slug = ? AND note_id = ?", slug, noteID).Scan(&n)
	if err != nil {
		return false, fmt.Errorf("concept mention exists: %w", err)
	}
	return n > 0, nil
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

// Rename sets a concept's display name.
func (r *ConceptRepo) Rename(ctx context.Context, slug, name string) error {
	res, err := r.db.sql.ExecContext(ctx,
		"UPDATE concepts SET name = ? WHERE slug = ?", name, slug)
	if err != nil {
		return fmt.Errorf("rename concept: %w", err)
	}
	return checkAffected(res, "rename concept")
}

// RawNamed returns the concepts still carrying their slug as their name — the
// ones that were first seen through a lowercase tag and never got a better one.
func (r *ConceptRepo) RawNamed(ctx context.Context, limit int) ([]*Concept, error) {
	if limit <= 0 {
		limit = 500
	}
	return r.list(ctx, `
		SELECT slug, name, note_id, mentions, first_seen, last_seen FROM concepts
		WHERE name = slug ORDER BY mentions DESC LIMIT ?`, limit)
}

// Resolve maps a freshly derived concept slug to the slug that already
// represents it: an explicit alias wins, then an existing concept sharing its
// canonical key. Unknown concepts resolve to themselves.
func (r *ConceptRepo) Resolve(ctx context.Context, slug string) (string, error) {
	var canonical string
	err := r.db.sql.QueryRowContext(ctx,
		"SELECT canonical FROM concept_aliases WHERE alias = ?", slug).Scan(&canonical)
	if err == nil && canonical != "" {
		return canonical, nil
	}
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return "", fmt.Errorf("resolve alias: %w", err)
	}

	var existing string
	err = r.db.sql.QueryRowContext(ctx,
		"SELECT slug FROM concepts WHERE canon_key = ? ORDER BY mentions DESC LIMIT 1", CanonKey(slug)).Scan(&existing)
	if err == nil && existing != "" {
		return existing, nil
	}
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return "", fmt.Errorf("resolve concept: %w", err)
	}
	return slug, nil
}

// Alias records that one slug should redirect to another from now on.
func (r *ConceptRepo) Alias(ctx context.Context, alias, canonical string) error {
	if alias == "" || canonical == "" || alias == canonical {
		return fmt.Errorf("concept alias: need two different slugs")
	}
	_, err := r.db.sql.ExecContext(ctx, `
		INSERT INTO concept_aliases (alias, canonical, created_at) VALUES (?, ?, ?)
		ON CONFLICT (alias) DO UPDATE SET canonical = excluded.canonical`,
		alias, canonical, Now())
	if err != nil {
		return fmt.Errorf("record alias: %w", err)
	}
	return nil
}

// Aliases returns the slugs redirecting to the given concept.
func (r *ConceptRepo) Aliases(ctx context.Context, canonical string) ([]string, error) {
	rows, err := r.db.sql.QueryContext(ctx,
		"SELECT alias FROM concept_aliases WHERE canonical = ? ORDER BY alias", canonical)
	if err != nil {
		return nil, fmt.Errorf("list aliases: %w", err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var a string
		if err := rows.Scan(&a); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// Merge folds one concept into another: every mention and every incoming link
// moves to the target, the source becomes an alias of it, and the notes that
// linked to the source are returned so the caller can rewrite their markdown.
func (r *ConceptRepo) Merge(ctx context.Context, from, to string) ([]int64, error) {
	if from == "" || to == "" || from == to {
		return nil, fmt.Errorf("concept merge: need two different slugs")
	}
	source, err := r.Get(ctx, from)
	if err != nil {
		return nil, err
	}
	if _, err := r.Get(ctx, to); errors.Is(err, ErrNotFound) {
		if _, err := r.Touch(ctx, to, source.Name, 0); err != nil {
			return nil, err
		}
	} else if err != nil {
		return nil, err
	}

	rows, err := r.db.sql.QueryContext(ctx, "SELECT from_note FROM kb_links WHERE to_slug = ?", from)
	if err != nil {
		return nil, fmt.Errorf("merge: list linking notes: %w", err)
	}
	var affected []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return nil, err
		}
		affected = append(affected, id)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}

	for _, stmt := range []struct {
		query string
		args  []any
	}{
		{`INSERT OR IGNORE INTO kb_links (from_note, to_slug, kind, created_at)
		  SELECT from_note, ?, kind, created_at FROM kb_links WHERE to_slug = ?`, []any{to, from}},
		{"DELETE FROM kb_links WHERE to_slug = ?", []any{from}},
		{`INSERT OR IGNORE INTO concept_mentions (concept_slug, note_id, created_at)
		  SELECT ?, note_id, created_at FROM concept_mentions WHERE concept_slug = ?`, []any{to, from}},
		{"DELETE FROM concept_mentions WHERE concept_slug = ?", []any{from}},
		{"DELETE FROM concepts WHERE slug = ?", []any{from}},
		{`UPDATE concepts SET mentions =
			(SELECT COUNT(*) FROM concept_mentions WHERE concept_slug = ?), last_seen = ?
		  WHERE slug = ?`, []any{to, Now(), to}},
	} {
		if _, err := r.db.sql.ExecContext(ctx, stmt.query, stmt.args...); err != nil {
			return nil, fmt.Errorf("merge concepts: %w", err)
		}
	}

	if err := r.Alias(ctx, from, to); err != nil {
		return nil, err
	}
	// Aliases of the source must follow it, or they would resolve to a concept
	// that no longer exists.
	if _, err := r.db.sql.ExecContext(ctx,
		"UPDATE concept_aliases SET canonical = ? WHERE canonical = ? AND alias != ?", to, from, to); err != nil {
		return nil, fmt.Errorf("repoint aliases: %w", err)
	}
	return affected, nil
}
