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
	// ContentHash is the SHA-256 of the file giznews last wrote for this note.
	// A file that no longer hashes to it has been edited by the user.
	ContentHash string `json:"content_hash,omitempty"`
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
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
	ContentHash string
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
		INSERT INTO kb_notes (note_type, title, slug, path, frontmatter, content, tags, wikilinks, content_hash, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		string(nn.Type), nn.Title, nn.Slug, nn.Path, nn.Frontmatter, nn.Content,
		marshalStrings(nn.Tags), marshalStrings(nn.Wikilinks), nn.ContentHash, now, now)
	if err != nil {
		return nil, fmt.Errorf("insert kb note: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("kb note last insert id: %w", err)
	}
	if err := r.SetLinks(ctx, id, nn.Wikilinks); err != nil {
		return nil, err
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
	return scanKBNotes(rows)
}

// Update persists content and metadata for an existing note. The embedding is
// dropped because it describes the previous content; the next search index run
// recomputes it.
func (r *KBRepo) Update(ctx context.Context, n *KBNote) error {
	res, err := r.db.sql.ExecContext(ctx, `
		UPDATE kb_notes SET
			title = ?, frontmatter = ?, content = ?, tags = ?, wikilinks = ?,
			embedding = NULL, updated_at = ?
		WHERE id = ?`,
		n.Title, n.Frontmatter, n.Content, marshalStrings(n.Tags), marshalStrings(n.Wikilinks), Now(), n.ID)
	if err != nil {
		return fmt.Errorf("update kb note: %w", err)
	}
	if err := checkAffected(res, "update kb note"); err != nil {
		return err
	}
	return r.SetLinks(ctx, n.ID, n.Wikilinks)
}

// SetLinks replaces a note's outgoing edges in kb_links. The JSON column stays
// the note's own record of its links; kb_links is the queryable graph, and both
// are written together so they cannot drift.
func (r *KBRepo) SetLinks(ctx context.Context, noteID int64, slugs []string) error {
	if _, err := r.db.sql.ExecContext(ctx, "DELETE FROM kb_links WHERE from_note = ?", noteID); err != nil {
		return fmt.Errorf("clear kb links: %w", err)
	}
	now := Now()
	for _, slug := range slugs {
		if slug == "" {
			continue
		}
		if _, err := r.db.sql.ExecContext(ctx, `
			INSERT OR IGNORE INTO kb_links (from_note, to_slug, kind, created_at)
			VALUES (?, ?, 'wikilink', ?)`, noteID, slug, now); err != nil {
			return fmt.Errorf("insert kb link: %w", err)
		}
	}
	return nil
}

// MarkFresh bumps a note's updated_at without touching its content. It is how
// a note that turned out to need no rewrite stops being reported as stale —
// Update would also drop the embedding and rewrite the links for nothing.
func (r *KBRepo) MarkFresh(ctx context.Context, id int64) error {
	res, err := r.db.sql.ExecContext(ctx,
		"UPDATE kb_notes SET updated_at = ? WHERE id = ?", Now(), id)
	if err != nil {
		return fmt.Errorf("mark note fresh: %w", err)
	}
	return checkAffected(res, "mark note fresh")
}

// SetContentHash records the fingerprint of the file written for a note.
func (r *KBRepo) SetContentHash(ctx context.Context, id int64, hash string) error {
	res, err := r.db.sql.ExecContext(ctx,
		"UPDATE kb_notes SET content_hash = ? WHERE id = ?", hash, id)
	if err != nil {
		return fmt.Errorf("set content hash: %w", err)
	}
	return checkAffected(res, "set content hash")
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

// Incoming returns the notes linking to the given slug (incoming graph edges),
// newest first.
func (r *KBRepo) Incoming(ctx context.Context, slug string, limit int) ([]*KBNote, error) {
	if limit <= 0 {
		limit = 500
	}
	rows, err := r.db.sql.QueryContext(ctx, `
		SELECT n.id, n.note_type, n.title, n.slug, n.path, n.frontmatter, n.content, n.tags, n.wikilinks, n.content_hash, n.created_at, n.updated_at
		FROM kb_links l
		JOIN kb_notes n ON n.id = l.from_note
		WHERE l.to_slug = ?
		ORDER BY n.created_at DESC LIMIT ?`, slug, limit)
	if err != nil {
		return nil, fmt.Errorf("kb incoming: %w", err)
	}
	defer rows.Close()
	return scanKBNotes(rows)
}

// CoMentioned returns notes that link to at least one slug the given note also
// links to. Two atoms citing the same concept are neighbours in the graph even
// when neither links to the other.
func (r *KBRepo) CoMentioned(ctx context.Context, noteID int64, limit int) ([]*KBNote, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := r.db.sql.QueryContext(ctx, `
		SELECT DISTINCT n.id, n.note_type, n.title, n.slug, n.path, n.frontmatter, n.content, n.tags, n.wikilinks, n.content_hash, n.created_at, n.updated_at
		FROM kb_links mine
		JOIN kb_links theirs ON theirs.to_slug = mine.to_slug AND theirs.from_note != mine.from_note
		JOIN kb_notes n ON n.id = theirs.from_note
		WHERE mine.from_note = ?
		ORDER BY n.created_at DESC LIMIT ?`, noteID, limit)
	if err != nil {
		return nil, fmt.Errorf("kb co-mentioned: %w", err)
	}
	defer rows.Close()
	return scanKBNotes(rows)
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
		SELECT id, note_type, title, slug, path, frontmatter, content, tags, wikilinks, content_hash, created_at, updated_at
		FROM kb_notes
		WHERE ( `+strings.Join(clauses, " OR ")+` ) AND id != ?
		ORDER BY created_at DESC LIMIT ?`, args...)
	if err != nil {
		return nil, fmt.Errorf("kb shared tags: %w", err)
	}
	defer rows.Close()
	return scanKBNotes(rows)
}

// Categories returns the distinct categories atoms declare in their
// frontmatter, alphabetically.
func (r *KBRepo) Categories(ctx context.Context) ([]string, error) {
	rows, err := r.db.sql.QueryContext(ctx, `
		SELECT DISTINCT json_extract(frontmatter, '$.category') AS category
		FROM kb_notes
		WHERE note_type = 'atom' AND category IS NOT NULL AND category != ''
		ORDER BY category`)
	if err != nil {
		return nil, fmt.Errorf("kb categories: %w", err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var c string
		if err := rows.Scan(&c); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// CreatedOn returns the notes created on the given YYYY-MM-DD day.
func (r *KBRepo) CreatedOn(ctx context.Context, day string, limit int) ([]*KBNote, error) {
	if limit <= 0 {
		limit = 200
	}
	rows, err := r.db.sql.QueryContext(ctx, kbNoteColumns+`
		WHERE substr(created_at, 1, 10) = ?
		ORDER BY created_at DESC LIMIT ?`, day, limit)
	if err != nil {
		return nil, fmt.Errorf("kb created on: %w", err)
	}
	defer rows.Close()
	return scanKBNotes(rows)
}

// ByCategory returns notes whose frontmatter declares the given category.
func (r *KBRepo) ByCategory(ctx context.Context, category string, limit int) ([]*KBNote, error) {
	rows, err := r.db.sql.QueryContext(ctx, `
		SELECT id, note_type, title, slug, path, frontmatter, content, tags, wikilinks, content_hash, created_at, updated_at
		FROM kb_notes
		WHERE frontmatter LIKE ? AND note_type = 'atom'
		ORDER BY created_at DESC LIMIT ?`, `%"category":"`+category+`"%`, limit)
	if err != nil {
		return nil, fmt.Errorf("kb by category: %w", err)
	}
	defer rows.Close()
	return scanKBNotes(rows)
}

// scanKBNotes drains a note result set selecting the kbNoteColumns columns.
func scanKBNotes(rows *sql.Rows) ([]*KBNote, error) {
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
	SELECT id, note_type, title, slug, path, frontmatter, content, tags, wikilinks, content_hash, created_at, updated_at
	FROM kb_notes`

func scanKBNote(row scanner) (*KBNote, error) {
	var (
		n        KBNote
		tagsRaw  string
		linksRaw string
	)
	if err := row.Scan(&n.ID, &n.Type, &n.Title, &n.Slug, &n.Path, &n.Frontmatter, &n.Content, &tagsRaw, &linksRaw, &n.ContentHash, &n.CreatedAt, &n.UpdatedAt); err != nil {
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
