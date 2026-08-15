package db

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

// ArticleRepo provides persistence for articles.
type ArticleRepo struct {
	db *DB
}

// NewArticleRepo creates an article repository.
func NewArticleRepo(db *DB) *ArticleRepo {
	return &ArticleRepo{db: db}
}

const articleColumns = `
	a.id, a.source_id, s.name, a.guid, a.url, a.title, a.author,
	a.content_html, a.content_md, a.summary, a.category, a.tags, a.entities,
	a.importance, a.simhash, a.status, a.published, a.fetched_at, a.updated_at`

const articleFrom = `
	FROM articles a
	LEFT JOIN sources s ON s.id = a.source_id`

// NewArticle is the input to insert an article.
type NewArticle struct {
	SourceID    int64
	GUID        string
	URL         string
	Title       string
	Author      string
	ContentHTML string
	ContentMD   string
	Summary     string
	Category    string
	Tags        []string
	Entities    []Entity
	Importance  int
	SimHash     uint64
	Status      ArticleStatus
	Published   string
}

// Upsert inserts an article or, on (source_id, guid) conflict, updates the
// mutable fields. It returns the article row id and whether it was newly
// inserted.
func (r *ArticleRepo) Upsert(ctx context.Context, na NewArticle) (id int64, created bool, err error) {
	now := Now()
	if na.Status == "" {
		na.Status = StatusUnread
	}

	// INSERT OR IGNORE gives a deterministic signal on first insert; an
	// existing row reports 0 rows affected, so we fall back to an update.
	res, err := r.db.sql.ExecContext(ctx, `
		INSERT OR IGNORE INTO articles (
			source_id, guid, url, title, author, content_html, content_md,
			summary, category, tags, entities, importance, simhash, status,
			published, fetched_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		na.SourceID, na.GUID, na.URL, na.Title, na.Author, na.ContentHTML, na.ContentMD,
		na.Summary, na.Category, marshalTags(na.Tags), marshalEntities(na.Entities),
		na.Importance, int64(na.SimHash), string(na.Status), na.Published, now, now)
	if err != nil {
		return 0, false, fmt.Errorf("insert article: %w", err)
	}
	if affected, _ := res.RowsAffected(); affected == 1 {
		id, err = res.LastInsertId()
		if err != nil {
			return 0, false, fmt.Errorf("article last insert id: %w", err)
		}
		return id, true, nil
	}

	// Existing row: refresh mutable fields and re-read the id.
	_, err = r.db.sql.ExecContext(ctx, `
		UPDATE articles SET
			url = ?, title = ?, author = ?, content_html = ?, content_md = ?,
			summary = ?, category = ?, tags = ?, entities = ?, importance = ?,
			simhash = ?, published = ?, updated_at = ?
		WHERE source_id = ? AND guid = ?`,
		na.URL, na.Title, na.Author, na.ContentHTML, na.ContentMD,
		na.Summary, na.Category, marshalTags(na.Tags), marshalEntities(na.Entities),
		na.Importance, int64(na.SimHash), na.Published, now, na.SourceID, na.GUID)
	if err != nil {
		return 0, false, fmt.Errorf("update article: %w", err)
	}
	err = r.db.sql.QueryRowContext(ctx,
		"SELECT id FROM articles WHERE source_id = ? AND guid = ?", na.SourceID, na.GUID).Scan(&id)
	if err != nil {
		return 0, false, fmt.Errorf("re-read article id: %w", err)
	}
	return id, false, nil
}

// Get returns an article by id.
func (r *ArticleRepo) Get(ctx context.Context, id int64) (*Article, error) {
	row := r.db.sql.QueryRowContext(ctx,
		"SELECT "+articleColumns+articleFrom+" WHERE a.id = ?", id)
	return scanArticle(row)
}

// GetByIDs returns articles by their ids, preserving the input order. Unknown
// ids are skipped.
func (r *ArticleRepo) GetByIDs(ctx context.Context, ids []int64) ([]*Article, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	placeholders := make([]string, len(ids))
	args := make([]any, len(ids))
	for i, id := range ids {
		placeholders[i] = "?"
		args[i] = id
	}
	rows, err := r.db.sql.QueryContext(ctx,
		"SELECT "+articleColumns+articleFrom+" WHERE a.id IN ("+strings.Join(placeholders, ",")+")", args...)
	if err != nil {
		return nil, fmt.Errorf("get articles by ids: %w", err)
	}
	defer rows.Close()

	byID := map[int64]*Article{}
	for rows.Next() {
		a, err := scanArticle(rows)
		if err != nil {
			return nil, err
		}
		byID[a.ID] = a
	}
	out := make([]*Article, 0, len(ids))
	for _, id := range ids {
		if a, ok := byID[id]; ok {
			out = append(out, a)
		}
	}
	return out, rows.Err()
}

// ListOptions filters the article list.
type ListOptions struct {
	Status        ArticleStatus // empty = all
	Category      string        // empty = all
	SourceID      int64         // 0 = all
	Group         string        // source group; empty = all
	ImportanceMin int           // only articles with importance >= this
	Unclassified  bool          // only articles not yet classified
	Query         string        // LIKE filter on title/author; empty = all
	Limit         int           // 0 = default 200
	Offset        int
}

// List returns articles matching the options, newest first.
func (r *ArticleRepo) List(ctx context.Context, opts ListOptions) ([]*Article, error) {
	var conds []string
	var args []any

	if opts.Status != "" {
		conds = append(conds, "a.status = ?")
		args = append(args, string(opts.Status))
	}
	if opts.Category != "" {
		conds = append(conds, "a.category = ?")
		args = append(args, opts.Category)
	}
	if opts.SourceID != 0 {
		conds = append(conds, "a.source_id = ?")
		args = append(args, opts.SourceID)
	}
	if opts.Group != "" {
		conds = append(conds, "s.group_name = ?")
		args = append(args, opts.Group)
	}
	if opts.ImportanceMin > 0 {
		conds = append(conds, "a.importance >= ?")
		args = append(args, opts.ImportanceMin)
	}
	if opts.Unclassified {
		conds = append(conds, "a.classified = 0")
	}
	if opts.Query != "" {
		conds = append(conds, "(a.title LIKE ? OR a.author LIKE ?)")
		q := "%" + opts.Query + "%"
		args = append(args, q, q)
	}

	where := ""
	if len(conds) > 0 {
		where = " WHERE " + strings.Join(conds, " AND ")
	}
	if opts.Limit <= 0 {
		opts.Limit = 200
	}
	query := "SELECT " + articleColumns + articleFrom + where +
		" ORDER BY a.published IS NULL, a.published DESC, a.fetched_at DESC LIMIT ? OFFSET ?"
	args = append(args, opts.Limit, opts.Offset)

	rows, err := r.db.sql.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list articles: %w", err)
	}
	defer rows.Close()

	var out []*Article
	for rows.Next() {
		a, err := scanArticle(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// SetStatus updates the triage status of an article.
func (r *ArticleRepo) SetStatus(ctx context.Context, id int64, status ArticleStatus) error {
	res, err := r.db.sql.ExecContext(ctx,
		"UPDATE articles SET status = ?, updated_at = ? WHERE id = ?", string(status), Now(), id)
	if err != nil {
		return fmt.Errorf("set article status: %w", err)
	}
	return checkAffected(res, "set article status")
}

// SetImportance updates the importance score of an article.
func (r *ArticleRepo) SetImportance(ctx context.Context, id int64, importance int) error {
	res, err := r.db.sql.ExecContext(ctx,
		"UPDATE articles SET importance = ?, updated_at = ? WHERE id = ?", importance, Now(), id)
	if err != nil {
		return fmt.Errorf("set article importance: %w", err)
	}
	return checkAffected(res, "set article importance")
}

// ExistsSimhash reports whether any article already carries the given simhash.
func (r *ArticleRepo) ExistsSimhash(ctx context.Context, h uint64) (bool, error) {
	var n int
	err := r.db.sql.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM articles WHERE simhash = ?", int64(h)).Scan(&n)
	if err != nil {
		return false, fmt.Errorf("count simhash: %w", err)
	}
	return n > 0, nil
}

// Count returns the number of articles matching an optional status.
func (r *ArticleRepo) Count(ctx context.Context, status ArticleStatus) (int, error) {
	var n int
	if status == "" {
		err := r.db.sql.QueryRowContext(ctx, "SELECT COUNT(*) FROM articles").Scan(&n)
		return n, err
	}
	err := r.db.sql.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM articles WHERE status = ?", string(status)).Scan(&n)
	if err != nil {
		return 0, fmt.Errorf("count articles: %w", err)
	}
	return n, nil
}

// ListUnclassified returns unread articles that have not been classified yet,
// limited to the last ageDays days. Both the fetch time and the publication
// date must be within the window (items with no publication date are kept), so
// archive dumps (a feed exposing its whole history) never flood the queue.
func (r *ArticleRepo) ListUnclassified(ctx context.Context, limit, ageDays int) ([]*Article, error) {
	if limit <= 0 {
		limit = 100
	}
	if ageDays <= 0 {
		ageDays = 14
	}
	cutoff := time.Now().Add(-time.Duration(ageDays) * 24 * time.Hour).UTC().Format(time.RFC3339)
	rows, err := r.db.sql.QueryContext(ctx, `
		SELECT `+articleColumns+articleFrom+`
		WHERE a.status != 'archived' AND a.classified = 0 AND a.fetched_at >= ?
		  AND (a.published IS NULL OR a.published = '' OR a.published >= ?)
		ORDER BY a.published IS NULL, a.published DESC
		LIMIT ?`, cutoff, cutoff, limit)
	if err != nil {
		return nil, fmt.Errorf("list unclassified: %w", err)
	}
	defer rows.Close()
	var out []*Article
	for rows.Next() {
		a, err := scanArticle(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// ApplyClassification persists the results of classification for one article
// and marks it as classified.
func (r *ArticleRepo) ApplyClassification(ctx context.Context, id int64, category, summary string, tags []string, entities []Entity, importance int) error {
	res, err := r.db.sql.ExecContext(ctx, `
		UPDATE articles SET
			category = ?, summary = ?, tags = ?, entities = ?, importance = ?,
			classified = 1, updated_at = ?
		WHERE id = ?`,
		category, summary, marshalTags(tags), marshalEntities(entities), importance, Now(), id)
	if err != nil {
		return fmt.Errorf("apply classification: %w", err)
	}
	return checkAffected(res, "apply classification")
}

// SetSummary stores a per-article summary without marking it classified
// (used by the interactive "summarize" action).
func (r *ArticleRepo) SetSummary(ctx context.Context, id int64, summary string) error {
	res, err := r.db.sql.ExecContext(ctx,
		"UPDATE articles SET summary = ?, updated_at = ? WHERE id = ?", summary, Now(), id)
	if err != nil {
		return fmt.Errorf("set summary: %w", err)
	}
	return checkAffected(res, "set summary")
}

// ListRecent returns non-archived articles fetched within the last sinceDays
// days, newest first. Used by the digest generator.
func (r *ArticleRepo) ListRecent(ctx context.Context, sinceDays, limit int) ([]*Article, error) {
	if sinceDays <= 0 {
		sinceDays = 7
	}
	if limit <= 0 {
		limit = 500
	}
	cutoff := time.Now().Add(-time.Duration(sinceDays) * 24 * time.Hour).UTC().Format(time.RFC3339)
	rows, err := r.db.sql.QueryContext(ctx, `
		SELECT `+articleColumns+articleFrom+`
		WHERE a.status != 'archived' AND a.fetched_at >= ?
		ORDER BY a.published IS NULL, a.published DESC
		LIMIT ?`, cutoff, limit)
	if err != nil {
		return nil, fmt.Errorf("list recent: %w", err)
	}
	defer rows.Close()
	var out []*Article
	for rows.Next() {
		a, err := scanArticle(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// ListForKB returns classified, non-archived articles at or above an
// importance threshold that have not yet been ingested into the knowledge
// graph (i.e. have no ingests row).
func (r *ArticleRepo) ListForKB(ctx context.Context, importanceMin, ageDays, limit int) ([]*Article, error) {
	if importanceMin <= 0 {
		importanceMin = 2
	}
	if ageDays <= 0 {
		ageDays = 30
	}
	if limit <= 0 {
		limit = 200
	}
	cutoff := time.Now().Add(-time.Duration(ageDays) * 24 * time.Hour).UTC().Format(time.RFC3339)
	rows, err := r.db.sql.QueryContext(ctx, `
		SELECT `+articleColumns+articleFrom+`
		WHERE a.status != 'archived' AND a.classified = 1 AND a.importance >= ?
		  AND a.fetched_at >= ?
		  AND NOT EXISTS (
			SELECT 1 FROM ingests i WHERE i.ref_type = 'article' AND i.ref_id = CAST(a.id AS TEXT)
		  )
		ORDER BY a.importance DESC, a.published IS NULL, a.published DESC
		LIMIT ?`, importanceMin, cutoff, limit)
	if err != nil {
		return nil, fmt.Errorf("list for kb: %w", err)
	}
	defer rows.Close()
	var out []*Article
	for rows.Next() {
		a, err := scanArticle(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// ListPending returns non-archived articles that have not yet been ingested
// into the knowledge graph (no ingests row). These are the vault "inbox":
// the raw material waiting to become Atom notes.
func (r *ArticleRepo) ListPending(ctx context.Context, limit int) ([]*Article, error) {
	if limit <= 0 {
		limit = 100
	}
	rows, err := r.db.sql.QueryContext(ctx, `
		SELECT `+articleColumns+articleFrom+`
		WHERE a.status != 'archived'
		  AND NOT EXISTS (
			SELECT 1 FROM ingests i WHERE i.ref_type = 'article' AND i.ref_id = CAST(a.id AS TEXT)
		  )
		ORDER BY a.importance DESC, a.published IS NULL, a.published DESC
		LIMIT ?`, limit)
	if err != nil {
		return nil, fmt.Errorf("list pending: %w", err)
	}
	defer rows.Close()
	var out []*Article
	for rows.Next() {
		a, err := scanArticle(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// CountUnclassified reports how many unread articles await classification.
func (r *ArticleRepo) CountUnclassified(ctx context.Context) (int, error) {
	var n int
	err := r.db.sql.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM articles WHERE status != 'archived' AND classified = 0").Scan(&n)
	if err != nil {
		return 0, fmt.Errorf("count unclassified: %w", err)
	}
	return n, nil
}

// SetArticleEmbedding stores the semantic-search vector for an article.
func (r *ArticleRepo) SetArticleEmbedding(ctx context.Context, id int64, embedding []float32) error {
	blob, err := marshalFloats(embedding)
	if err != nil {
		return err
	}
	res, err := r.db.sql.ExecContext(ctx,
		"UPDATE articles SET embedding = ?, updated_at = ? WHERE id = ?", blob, Now(), id)
	if err != nil {
		return fmt.Errorf("set article embedding: %w", err)
	}
	return checkAffected(res, "set article embedding")
}

// SetContent persists extracted article content (HTML + markdown).
func (r *ArticleRepo) SetContent(ctx context.Context, id int64, contentHTML, contentMD string) error {
	res, err := r.db.sql.ExecContext(ctx,
		"UPDATE articles SET content_html = ?, content_md = ?, updated_at = ? WHERE id = ?",
		contentHTML, contentMD, Now(), id)
	if err != nil {
		return fmt.Errorf("set article content: %w", err)
	}
	return checkAffected(res, "set article content")
}

// SetExtracted marks whether a full-content extraction attempt succeeded.
func (r *ArticleRepo) SetExtracted(ctx context.Context, id int64, ok bool) error {
	_, err := r.db.sql.ExecContext(ctx,
		"UPDATE articles SET extracted = ?, updated_at = ? WHERE id = ?", boolToInt(ok), Now(), id)
	if err != nil {
		return fmt.Errorf("set article extracted: %w", err)
	}
	return nil
}

// ListPendingExtract returns articles that lack a real body and have not been
// extracted yet, capped at limit, most important first.
func (r *ArticleRepo) ListPendingExtract(ctx context.Context, limit int) ([]*Article, error) {
	if limit <= 0 {
		limit = 20
	}
	rows, err := r.db.sql.QueryContext(ctx, `
		SELECT `+articleColumns+articleFrom+`
		WHERE a.extracted = 0 AND LENGTH(a.content_md) < 200 AND a.url != ''
		ORDER BY a.importance DESC, a.fetched_at DESC
		LIMIT ?`, limit)
	if err != nil {
		return nil, fmt.Errorf("list pending extract: %w", err)
	}
	defer rows.Close()
	var out []*Article
	for rows.Next() {
		a, err := scanArticle(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// GetArticleEmbedding returns the stored embedding for an article, or nil.
func (r *ArticleRepo) GetArticleEmbedding(ctx context.Context, id int64) ([]float32, error) {
	var blob []byte
	err := r.db.sql.QueryRowContext(ctx, "SELECT embedding FROM articles WHERE id = ?", id).Scan(&blob)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get article embedding: %w", err)
	}
	return unmarshalFloats(blob)
}

func scanArticle(row scanner) (*Article, error) {
	var (
		a       Article
		tagsRaw string
		entRaw  string
		simHash int64 // SQLite stores INTEGER as signed int64
	)
	if err := row.Scan(
		&a.ID, &a.SourceID, &a.SourceName, &a.GUID, &a.URL, &a.Title, &a.Author,
		&a.ContentHTML, &a.ContentMD, &a.Summary, &a.Category, &tagsRaw, &entRaw,
		&a.Importance, &simHash, &a.Status, &a.Published, &a.FetchedAt, &a.UpdatedAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("scan article: %w", err)
	}
	a.SimHash = uint64(simHash)
	_ = json.Unmarshal([]byte(tagsRaw), &a.Tags)
	_ = json.Unmarshal([]byte(entRaw), &a.Entities)
	if a.Tags == nil {
		a.Tags = []string{}
	}
	if a.Entities == nil {
		a.Entities = []Entity{}
	}
	return &a, nil
}
