package db

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

// NoteType is the Zettelkasten note category (mirrors chronicles folders).
type NoteType string

const (
	NoteInbox    NoteType = "inbox"
	NoteAtom     NoteType = "atom"
	NoteElectron NoteType = "electron"
	NoteMolecule NoteType = "molecule"
)

// KBNote is a knowledge-graph note stored in the vault.
type KBNote struct {
	ID          int64    `json:"id"`
	Type        NoteType `json:"type"`
	Title       string   `json:"title"`
	Slug        string   `json:"slug"`
	Path        string   `json:"path"`
	Frontmatter string   `json:"frontmatter,omitempty"` // JSON map
	Content     string   `json:"content"`
	Tags        []string `json:"tags"`
	Wikilinks   []string `json:"wikilinks"`
	CreatedAt   string   `json:"created_at"`
	UpdatedAt   string   `json:"updated_at"`
}

// NewKBNote is the input to insert a note.
type NewKBNote struct {
	Type        NoteType
	Title       string
	Slug        string
	Path        string
	Frontmatter string
	Content     string
	Tags        []string
	Wikilinks   []string
}

// KBRepo provides persistence for knowledge-graph notes.
type KBRepo struct {
	db *DB
}

// NewKBRepo creates a note repository.
func NewKBRepo(db *DB) *KBRepo {
	return &KBRepo{db: db}
}

// Create inserts a note and returns it.
func (r *KBRepo) Create(ctx context.Context, nn NewKBNote) (*KBNote, error) {
	now := Now()
	res, err := r.db.sql.ExecContext(ctx, `
		INSERT INTO kb_notes (note_type, title, slug, path, frontmatter, content, tags, wikilinks, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		string(nn.Type), nn.Title, nn.Slug, nn.Path, nn.Frontmatter, nn.Content,
		marshalStrings(nn.Tags), marshalStrings(nn.Wikilinks), now, now)
	if err != nil {
		return nil, fmt.Errorf("insert kb note: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("kb note last insert id: %w", err)
	}
	return r.Get(ctx, id)
}

// Get returns a note by id.
func (r *KBRepo) Get(ctx context.Context, id int64) (*KBNote, error) {
	row := r.db.sql.QueryRowContext(ctx, kbNoteColumns+" WHERE id = ?", id)
	return scanKBNote(row)
}

// GetBySlug returns a note by its unique slug.
func (r *KBRepo) GetBySlug(ctx context.Context, slug string) (*KBNote, error) {
	row := r.db.sql.QueryRowContext(ctx, kbNoteColumns+" WHERE slug = ?", slug)
	return scanKBNote(row)
}

// List returns notes, optionally filtered by type, ordered by created desc.
func (r *KBRepo) List(ctx context.Context, noteType NoteType, limit int) ([]*KBNote, error) {
	q := kbNoteColumns
	var args []any
	if noteType != "" {
		q += " WHERE note_type = ?"
		args = append(args, string(noteType))
	}
	q += " ORDER BY created_at DESC"
	if limit > 0 {
		q += " LIMIT ?"
		args = append(args, limit)
	}
	rows, err := r.db.sql.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("list kb notes: %w", err)
	}
	defer rows.Close()
	var out []*KBNote
	for rows.Next() {
		n, err := scanKBNote(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, n)
	}
	return out, rows.Err()
}

// Update persists content and metadata for an existing note.
func (r *KBRepo) Update(ctx context.Context, n *KBNote) error {
	res, err := r.db.sql.ExecContext(ctx, `
		UPDATE kb_notes SET
			title = ?, frontmatter = ?, content = ?, tags = ?, wikilinks = ?, updated_at = ?
		WHERE id = ?`,
		n.Title, n.Frontmatter, n.Content, marshalStrings(n.Tags), marshalStrings(n.Wikilinks), Now(), n.ID)
	if err != nil {
		return fmt.Errorf("update kb note: %w", err)
	}
	return checkAffected(res, "update kb note")
}

// SetEmbedding stores the semantic-search vector for a note.
func (r *KBRepo) SetEmbedding(ctx context.Context, id int64, embedding []float32) error {
	blob, err := marshalFloats(embedding)
	if err != nil {
		return err
	}
	res, err := r.db.sql.ExecContext(ctx,
		"UPDATE kb_notes SET embedding = ?, updated_at = ? WHERE id = ?", blob, Now(), id)
	if err != nil {
		return fmt.Errorf("set embedding: %w", err)
	}
	return checkAffected(res, "set embedding")
}

// GetEmbedding returns the stored embedding for a note, or nil if none.
func (r *KBRepo) GetEmbedding(ctx context.Context, id int64) ([]float32, error) {
	var blob []byte
	err := r.db.sql.QueryRowContext(ctx, "SELECT embedding FROM kb_notes WHERE id = ?", id).Scan(&blob)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get embedding: %w", err)
	}
	return unmarshalFloats(blob)
}

// Count returns the total number of notes (optionally by type).
func (r *KBRepo) Count(ctx context.Context, noteType NoteType) (int, error) {
	q := "SELECT COUNT(*) FROM kb_notes"
	var args []any
	if noteType != "" {
		q += " WHERE note_type = ?"
		args = append(args, string(noteType))
	}
	var n int
	if err := r.db.sql.QueryRowContext(ctx, q, args...).Scan(&n); err != nil {
		return 0, fmt.Errorf("count kb notes: %w", err)
	}
	return n, nil
}

// Incoming returns notes whose wikilinks reference the given slug (incoming
// graph edges).
func (r *KBRepo) Incoming(ctx context.Context, slug string, limit int) ([]*KBNote, error) {
	rows, err := r.db.sql.QueryContext(ctx, `
		SELECT id, note_type, title, slug, path, frontmatter, content, tags, wikilinks, created_at, updated_at
		FROM kb_notes WHERE wikilinks LIKE ?
		ORDER BY created_at DESC LIMIT ?`, `%`+slug+`%`, limit)
	if err != nil {
		return nil, fmt.Errorf("kb incoming: %w", err)
	}
	defer rows.Close()
	var out []*KBNote
	for rows.Next() {
		n, err := scanKBNote(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, n)
	}
	return out, rows.Err()
}

// BySharedTag returns notes (excluding self) that share any of tags, capped at
// limit, newest first.
func (r *KBRepo) BySharedTag(ctx context.Context, selfID int64, tags []string, limit int) ([]*KBNote, error) {
	if len(tags) == 0 {
		return nil, nil
	}
	clauses := make([]string, 0, len(tags))
	args := make([]any, 0, len(tags)+1)
	for _, tag := range tags {
		clauses = append(clauses, "tags LIKE ?")
		args = append(args, "%\""+tag+"\"%")
	}
	args = append(args, selfID, limit)
	rows, err := r.db.sql.QueryContext(ctx, `
		SELECT id, note_type, title, slug, path, frontmatter, content, tags, wikilinks, created_at, updated_at
		FROM kb_notes
		WHERE ( `+strings.Join(clauses, " OR ")+` ) AND id != ?
		ORDER BY created_at DESC LIMIT ?`, args...)
	if err != nil {
		return nil, fmt.Errorf("kb shared tags: %w", err)
	}
	defer rows.Close()
	var out []*KBNote
	for rows.Next() {
		n, err := scanKBNote(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, n)
	}
	return out, rows.Err()
}

// ByCategory returns notes whose frontmatter declares the given category.
func (r *KBRepo) ByCategory(ctx context.Context, category string, limit int) ([]*KBNote, error) {
	rows, err := r.db.sql.QueryContext(ctx, `
		SELECT id, note_type, title, slug, path, frontmatter, content, tags, wikilinks, created_at, updated_at
		FROM kb_notes
		WHERE frontmatter LIKE ? AND note_type = 'atom'
		ORDER BY created_at DESC LIMIT ?`, `%"category":"`+category+`"%`, limit)
	if err != nil {
		return nil, fmt.Errorf("kb by category: %w", err)
	}
	defer rows.Close()
	var out []*KBNote
	for rows.Next() {
		n, err := scanKBNote(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, n)
	}
	return out, rows.Err()
}

const kbNoteColumns = `
	SELECT id, note_type, title, slug, path, frontmatter, content, tags, wikilinks, created_at, updated_at
	FROM kb_notes`

func scanKBNote(row scanner) (*KBNote, error) {
	var (
		n        KBNote
		tagsRaw  string
		linksRaw string
	)
	if err := row.Scan(&n.ID, &n.Type, &n.Title, &n.Slug, &n.Path, &n.Frontmatter, &n.Content, &tagsRaw, &linksRaw, &n.CreatedAt, &n.UpdatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("scan kb note: %w", err)
	}
	_ = json.Unmarshal([]byte(tagsRaw), &n.Tags)
	_ = json.Unmarshal([]byte(linksRaw), &n.Wikilinks)
	if n.Tags == nil {
		n.Tags = []string{}
	}
	if n.Wikilinks == nil {
		n.Wikilinks = []string{}
	}
	return &n, nil
}

func marshalStrings(s []string) string {
	if s == nil {
		s = []string{}
	}
	b, _ := json.Marshal(s)
	return string(b)
}

func marshalFloats(f []float32) ([]byte, error) {
	return json.Marshal(f)
}

func unmarshalFloats(b []byte) ([]float32, error) {
	if len(b) == 0 {
		return nil, nil
	}
	var out []float32
	if err := json.Unmarshal(b, &out); err != nil {
		return nil, fmt.Errorf("unmarshal floats: %w", err)
	}
	return out, nil
}
