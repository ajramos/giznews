package db

import (
	"context"
	"fmt"
)

// WatchHit is an article a watch rule caught.
type WatchHit struct {
	ArticleID int64    `json:"article_id"`
	Rule      string   `json:"rule"`
	Seen      bool     `json:"seen"`
	CreatedAt string   `json:"created_at"`
	Article   *Article `json:"article,omitempty"`
}

// WatchRepo persists what the watches caught.
type WatchRepo struct {
	db *DB
}

// NewWatchRepo creates a watch repository.
func NewWatchRepo(db *DB) *WatchRepo {
	return &WatchRepo{db: db}
}

// Record notes that a watch caught an article, and reports whether this is the
// first time. Being told twice about the same article is worse than not being
// told at all, so the second call changes nothing and returns false.
func (r *WatchRepo) Record(ctx context.Context, articleID int64, rule string) (bool, error) {
	res, err := r.db.sql.ExecContext(ctx, `
		INSERT INTO watch_hits (article_id, rule, seen, created_at) VALUES (?, ?, 0, ?)
		ON CONFLICT(article_id) DO NOTHING`, articleID, rule, Now())
	if err != nil {
		return false, fmt.Errorf("record watch hit: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("record watch hit: %w", err)
	}
	return n > 0, nil
}

// List returns recent hits, newest first, with their articles. onlyUnseen
// narrows it to what the reader has not been shown yet.
func (r *WatchRepo) List(ctx context.Context, onlyUnseen bool, limit int) ([]*WatchHit, error) {
	if limit <= 0 {
		limit = 50
	}
	where := ""
	if onlyUnseen {
		where = " AND w.seen = 0"
	}
	rows, err := r.db.sql.QueryContext(ctx, `
		SELECT w.article_id, w.rule, w.seen, w.created_at, `+articleColumns+`
		FROM watch_hits w
		JOIN articles a ON a.id = w.article_id
		LEFT JOIN sources s ON s.id = a.source_id
		WHERE 1 = 1`+where+`
		ORDER BY w.created_at DESC, w.article_id DESC
		LIMIT ?`, limit)
	if err != nil {
		return nil, fmt.Errorf("list watch hits: %w", err)
	}
	defer rows.Close()

	var out []*WatchHit
	for rows.Next() {
		hit, err := scanWatchHit(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, hit)
	}
	return out, rows.Err()
}

// MarkSeen records that the reader has been shown these hits, so nothing
// announces them again.
func (r *WatchRepo) MarkSeen(ctx context.Context, ids []int64) error {
	for _, id := range ids {
		if _, err := r.db.sql.ExecContext(ctx,
			"UPDATE watch_hits SET seen = 1 WHERE article_id = ?", id); err != nil {
			return fmt.Errorf("mark watch hit seen: %w", err)
		}
	}
	return nil
}

// Since returns the hits recorded on or after a day, for the digest and the
// vault's daily note.
func (r *WatchRepo) Since(ctx context.Context, day string, limit int) ([]*WatchHit, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := r.db.sql.QueryContext(ctx, `
		SELECT w.article_id, w.rule, w.seen, w.created_at, `+articleColumns+`
		FROM watch_hits w
		JOIN articles a ON a.id = w.article_id
		LEFT JOIN sources s ON s.id = a.source_id
		WHERE w.created_at >= ?
		ORDER BY w.created_at DESC, w.article_id DESC
		LIMIT ?`, day, limit)
	if err != nil {
		return nil, fmt.Errorf("watch hits since %s: %w", day, err)
	}
	defer rows.Close()
	var out []*WatchHit
	for rows.Next() {
		hit, err := scanWatchHit(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, hit)
	}
	return out, rows.Err()
}

func scanWatchHit(row scanner) (*WatchHit, error) {
	var (
		hit     WatchHit
		seen    int
		a       Article
		tagsRaw string
		entRaw  string
		simHash int64
		starred int
	)
	if err := row.Scan(
		&hit.ArticleID, &hit.Rule, &seen, &hit.CreatedAt,
		&a.ID, &a.SourceID, &a.SourceName, &a.GUID, &a.URL, &a.Title, &a.Author,
		&a.ContentHTML, &a.ContentMD, &a.Summary, &a.Category, &tagsRaw, &entRaw,
		&a.Importance, &simHash, &a.Status, &starred, &a.Published, &a.FetchedAt, &a.UpdatedAt,
		&a.StoryID, &a.StorySize,
	); err != nil {
		return nil, fmt.Errorf("scan watch hit: %w", err)
	}
	a.SimHash = uint64(simHash)
	a.Starred = starred != 0
	hit.Seen = seen != 0
	hit.Article = &a
	return &hit, nil
}
