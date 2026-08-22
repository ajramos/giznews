// Package prune reclaims the space an archive of read news does not need.
//
// Nothing in giznews ever deleted an article. Archiving is logical, which is
// the right default — you can always go back — and it is exactly why the file
// only grows: every article ever fetched keeps its full extracted body, its
// embedding and its row in the search index, forever. A feed of a few hundred
// items a day turns into a multi-gigabyte SQLite file that is slower at
// everything, on a machine where nobody asked to keep last year's press
// releases.
//
// So this is deliberately staged from cheapest to most destructive: bodies
// first, which is most of the space and almost none of the meaning, then the
// rows themselves much later.
package prune

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/ajramos/giznews/internal/db"
)

// Options says how old is old enough. Both defaults are conservative: half a
// year before an article loses its body, a full year before it loses its row.
type Options struct {
	BodyDays int // clear bodies older than this (0 = 180)
	RowDays  int // delete rows older than this (0 = 365)
}

func (o Options) bodyDays() int {
	if o.BodyDays > 0 {
		return o.BodyDays
	}
	return 180
}

func (o Options) rowDays() int {
	if o.RowDays > 0 {
		return o.RowDays
	}
	return 365
}

// Plan is what a prune would do.
type Plan struct {
	BodyDays  int   `json:"body_days"`
	RowDays   int   `json:"row_days"`
	Bodies    int   `json:"bodies"`
	BodyBytes int64 `json:"body_bytes"`
	Rows      int   `json:"rows"`
	RowBytes  int64 `json:"row_bytes"`
	// Kept counts the old articles that are staying, and why. A prune that
	// spares nothing is a prune worth looking at twice.
	KeptStarred int   `json:"kept_starred"`
	KeptUnread  int   `json:"kept_unread"`
	KeptInVault int   `json:"kept_in_vault"`
	SizeBefore  int64 `json:"size_before"`
}

// Result is what it did.
type Result struct {
	Plan
	SizeAfter int64 `json:"size_after"`
}

// Preview works out what would go, writing nothing.
func Preview(ctx context.Context, d *db.DB, opts Options) (*Plan, error) {
	plan := &Plan{BodyDays: opts.bodyDays(), RowDays: opts.rowDays()}

	size, err := dbSize(ctx, d)
	if err != nil {
		return nil, err
	}
	plan.SizeBefore = size

	bodies, err := candidates(ctx, d, bodyCandidates, cutoff(opts.bodyDays()))
	if err != nil {
		return nil, err
	}
	rows, err := candidates(ctx, d, rowCandidates, cutoff(opts.rowDays()))
	if err != nil {
		return nil, err
	}
	// An article old enough to be deleted is also old enough to lose its body,
	// so it appears in both sets. Counting it twice would promise space that
	// does not exist: what the first stage really takes is the difference.
	going := make(map[int64]bool, len(rows))
	for _, c := range rows {
		going[c.id] = true
		plan.Rows++
		plan.RowBytes += c.size
	}
	for _, c := range bodies {
		if going[c.id] {
			continue
		}
		plan.Bodies++
		plan.BodyBytes += c.size
	}

	kept, err := keptCounts(ctx, d, cutoff(opts.bodyDays()))
	if err != nil {
		return nil, err
	}
	plan.KeptStarred, plan.KeptUnread, plan.KeptInVault = kept[0], kept[1], kept[2]
	return plan, nil
}

// Apply prunes, then reclaims the file.
func Apply(ctx context.Context, d *db.DB, opts Options, logger *log.Logger) (*Result, error) {
	plan, err := Preview(ctx, d, opts)
	if err != nil {
		return nil, err
	}
	res := &Result{Plan: *plan}

	// Rows first: dropping their bodies a moment before deleting them would be
	// work nobody needs.
	deleted, err := ids(ctx, d, rowCandidates, cutoff(opts.rowDays()))
	if err != nil {
		return nil, err
	}
	if len(deleted) > 0 {
		if err := deleteArticles(ctx, d, deleted); err != nil {
			return nil, err
		}
		if logger != nil {
			logger.Printf("prune: deleted %d article(s) older than %d days", len(deleted), opts.rowDays())
		}
	}

	stripped, err := ids(ctx, d, bodyCandidates, cutoff(opts.bodyDays()))
	if err != nil {
		return nil, err
	}
	if len(stripped) > 0 {
		if err := dropBodies(ctx, d, stripped); err != nil {
			return nil, err
		}
		if logger != nil {
			logger.Printf("prune: dropped the body of %d article(s) older than %d days", len(stripped), opts.bodyDays())
		}
	}

	if len(deleted)+len(stripped) > 0 {
		if err := vacuum(ctx, d); err != nil {
			return nil, err
		}
	}
	after, err := dbSize(ctx, d)
	if err != nil {
		return nil, err
	}
	res.SizeAfter = after
	return res, nil
}

// protection is what a prune may never touch, whatever its age.
//
// Starred is an explicit "keep this". Unread has not been read yet, and
// deleting something before it was seen is the one thing a reader would never
// forgive. An article with a note in the vault is the source of something
// written down, so its row has to stay reachable.
//
// The last clause is about stories: copies are pruned as a unit. Deleting the
// anchor of a story while its members remain would strand them — they point at
// a row that is gone, and `storyAnchor` would never surface them again — so if
// any copy is protected, or recent, the whole story is.
const protection = `
	WITH keyed AS (
		SELECT a.id, COALESCE(NULLIF(a.story_id, 0), a.id) AS story_key,
		       a.starred, a.status, a.fetched_at
		FROM articles a
	),
	held AS (
		SELECT DISTINCT k.story_key
		FROM keyed k
		WHERE k.starred = 1
		   OR k.status = 'unread'
		   OR k.fetched_at >= :cutoff
		   OR EXISTS (SELECT 1 FROM ingests i
		              WHERE i.ref_type = 'article' AND i.ref_id = CAST(k.id AS TEXT))
	)`

// bodyCandidates are old articles that still carry an extracted body.
const bodyCandidates = protection + `
	SELECT a.id, LENGTH(a.content_md) + LENGTH(a.content_html)
	FROM articles a JOIN keyed k ON k.id = a.id
	WHERE k.story_key NOT IN (SELECT story_key FROM held)
	  AND (LENGTH(a.content_md) > 0 OR LENGTH(a.content_html) > 0)`

// rowCandidates are articles old enough to go entirely.
const rowCandidates = protection + `
	SELECT a.id, LENGTH(a.content_md) + LENGTH(a.content_html)
	     + LENGTH(COALESCE(a.summary, '')) + LENGTH(COALESCE(a.title, ''))
	     + LENGTH(COALESCE(a.embedding, ''))
	FROM articles a JOIN keyed k ON k.id = a.id
	WHERE k.story_key NOT IN (SELECT story_key FROM held)`

func cutoff(days int) string {
	return time.Now().UTC().AddDate(0, 0, -days).Format(time.RFC3339)
}

// candidate is one article a stage would act on, and what it weighs.
type candidate struct {
	id   int64
	size int64
}

func candidates(ctx context.Context, d *db.DB, query, at string) ([]candidate, error) {
	rows, err := d.SQL().QueryContext(ctx, sqlWithCutoff(query), at)
	if err != nil {
		return nil, fmt.Errorf("prune: select: %w", err)
	}
	defer rows.Close()
	var out []candidate
	for rows.Next() {
		var c candidate
		if err := rows.Scan(&c.id, &c.size); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func ids(ctx context.Context, d *db.DB, query, at string) ([]int64, error) {
	found, err := candidates(ctx, d, query, at)
	if err != nil {
		return nil, err
	}
	out := make([]int64, 0, len(found))
	for _, c := range found {
		out = append(out, c.id)
	}
	return out, nil
}

// sqlWithCutoff turns the named cutoff into the positional parameter the driver
// wants, in the two places it appears.
func sqlWithCutoff(query string) string {
	return strings.ReplaceAll(query, ":cutoff", "?")
}

// keptCounts reports what age alone would have taken, had it not been spared.
func keptCounts(ctx context.Context, d *db.DB, at string) ([3]int, error) {
	var out [3]int
	err := d.SQL().QueryRowContext(ctx, `
		SELECT
			SUM(CASE WHEN a.starred = 1 THEN 1 ELSE 0 END),
			SUM(CASE WHEN a.status = 'unread' THEN 1 ELSE 0 END),
			SUM(CASE WHEN EXISTS (SELECT 1 FROM ingests i
			         WHERE i.ref_type = 'article' AND i.ref_id = CAST(a.id AS TEXT)) THEN 1 ELSE 0 END)
		FROM articles a WHERE a.fetched_at < ?`, at).Scan(&out[0], &out[1], &out[2])
	if err != nil {
		return out, fmt.Errorf("prune: kept: %w", err)
	}
	return out, nil
}

// dropBodies clears the text and takes the article out of the search index.
// The extracted flag stays set on purpose: without it the extractor would see a
// short body and fetch the whole article again, undoing the prune on the next
// run and re-downloading a year of news to do it.
func dropBodies(ctx context.Context, d *db.DB, articles []int64) error {
	for _, chunk := range chunks(articles, 400) {
		list := placeholders(len(chunk))
		args := asArgs(chunk)
		if _, err := d.SQL().ExecContext(ctx,
			"UPDATE articles SET content_md = '', content_html = '', extracted = 1, updated_at = ? WHERE id IN "+list,
			append([]any{db.Now()}, args...)...); err != nil {
			return fmt.Errorf("prune: drop bodies: %w", err)
		}
		if err := forgetInIndex(ctx, d, chunk); err != nil {
			return err
		}
	}
	return nil
}

// deleteArticles removes the rows. article_events go with them by cascade; the
// search index has to be told.
func deleteArticles(ctx context.Context, d *db.DB, articles []int64) error {
	for _, chunk := range chunks(articles, 400) {
		if err := forgetInIndex(ctx, d, chunk); err != nil {
			return err
		}
		if _, err := d.SQL().ExecContext(ctx,
			"DELETE FROM articles WHERE id IN "+placeholders(len(chunk)), asArgs(chunk)...); err != nil {
			return fmt.Errorf("prune: delete: %w", err)
		}
	}
	return nil
}

// forgetInIndex drops the FTS rows, which hold their own copy of the text and
// would otherwise keep every byte this was meant to reclaim. What survives is
// put back by the next `search index`.
func forgetInIndex(ctx context.Context, d *db.DB, articles []int64) error {
	_, err := d.SQL().ExecContext(ctx,
		"DELETE FROM articles_fts WHERE article_id IN "+placeholders(len(articles)), asArgs(articles)...)
	if err != nil && !strings.Contains(err.Error(), "no such table") {
		return fmt.Errorf("prune: search index: %w", err)
	}
	return nil
}

func vacuum(ctx context.Context, d *db.DB) error {
	if _, err := d.SQL().ExecContext(ctx, "VACUUM"); err != nil {
		return fmt.Errorf("prune: vacuum: %w", err)
	}
	return nil
}

// dbSize is what the file actually occupies, asked of SQLite rather than of the
// filesystem so it works wherever the database lives.
func dbSize(ctx context.Context, d *db.DB) (int64, error) {
	var pages, pageSize int64
	if err := d.SQL().QueryRowContext(ctx, "PRAGMA page_count").Scan(&pages); err != nil {
		return 0, fmt.Errorf("prune: page count: %w", err)
	}
	if err := d.SQL().QueryRowContext(ctx, "PRAGMA page_size").Scan(&pageSize); err != nil {
		return 0, fmt.Errorf("prune: page size: %w", err)
	}
	return pages * pageSize, nil
}

func chunks(ids []int64, size int) [][]int64 {
	var out [][]int64
	for start := 0; start < len(ids); start += size {
		end := start + size
		if end > len(ids) {
			end = len(ids)
		}
		out = append(out, ids[start:end])
	}
	return out
}

func placeholders(n int) string {
	return "(" + strings.TrimSuffix(strings.Repeat("?,", n), ",") + ")"
}

func asArgs(ids []int64) []any {
	out := make([]any, 0, len(ids))
	for _, id := range ids {
		out = append(out, id)
	}
	return out
}
