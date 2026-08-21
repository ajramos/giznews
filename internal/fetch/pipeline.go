package fetch

import (
	"context"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/ajramos/giznews/internal/db"
	"github.com/ajramos/giznews/internal/extract"
	"github.com/ajramos/giznews/internal/sources"
)

// dedupHammingThreshold is the max Hamming distance for two articles to be
// considered the same document. It catches a feed republishing a piece
// verbatim; two newsrooms writing the same story is a different question, and
// SameStory answers it.
const dedupHammingThreshold = 3

// Result reports what a fetch run did.
type Result struct {
	NewArticles int `json:"new_articles"`
	Updated     int `json:"updated"`
	// Grouped counts copies filed under a story that was already here — a
	// second outlet running the same piece.
	Grouped        int      `json:"grouped"`
	Duplicates     int      `json:"duplicates"`
	SourcesFetched int      `json:"sources_fetched"`
	SourcesFailed  int      `json:"sources_failed"`
	Extracted      int      `json:"extracted"`
	Errors         []string `json:"errors,omitempty"`
	ElapsedMs      int64    `json:"elapsed_ms"`
}

// Service runs the fetch pipeline: fetch → normalize → dedupe → persist, and
// optionally extracts full article bodies in batch.
type Service struct {
	db     *db.DB
	man    *sources.Manager
	logger *log.Logger

	extractor      *extract.Service
	extractLimit   int
	extractWorkers int

	maxAge time.Duration // 0 = unlimited

	// The dedup caches map a fingerprint to the article that carries it, not
	// merely to "seen": a copy has to know which story it belongs to.
	mu         sync.RWMutex
	recentHash map[uint64]int64
	knownURLs  map[string]int64
	// Headlines are matched by their words, indexed by token so a new item is
	// only compared against the few articles that share one.
	titles     map[int64]titleEntry
	titleIndex map[string][]int64
}

// titleEntry is one recent headline and the story it belongs to.
type titleEntry struct {
	tokens []string
	anchor int64
}

// NewService builds a fetch pipeline service.
func NewService(database *db.DB, man *sources.Manager, logger *log.Logger) (*Service, error) {
	s := &Service{
		db:         database,
		man:        man,
		logger:     logger,
		recentHash: map[uint64]int64{},
		knownURLs:  map[string]int64{},
		titles:     map[int64]titleEntry{},
		titleIndex: map[string][]int64{},
	}
	if err := s.warmDedup(); err != nil {
		return nil, fmt.Errorf("warm dedup cache: %w", err)
	}
	return s, nil
}

// SetMaxAge drops feed items published more than the given number of days ago.
// A non-positive value disables the filter (keep everything).
func (s *Service) SetMaxAge(days int) {
	if days <= 0 {
		s.maxAge = 0
		return
	}
	s.maxAge = time.Duration(days) * 24 * time.Hour
}

// warmDedup preloads recent article fingerprints so cross-source duplicates
// within the window are caught even before the pipeline inserts them.
func (s *Service) warmDedup() error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cutoff := time.Now().Add(-30 * 24 * time.Hour).UTC().Format(time.RFC3339)
	// The anchor is what a new copy joins, so a story already three deep does
	// not fork into a second one.
	rows, err := s.db.SQL().QueryContext(ctx, `
		SELECT id, url, title, simhash, CASE WHEN story_id = 0 THEN id ELSE story_id END
		FROM articles WHERE fetched_at >= ?`, cutoff)
	if err != nil {
		return err
	}
	defer rows.Close()

	s.mu.Lock()
	defer s.mu.Unlock()
	for rows.Next() {
		var (
			id     int64
			u      string
			title  string
			h      int64
			anchor int64
		)
		if err := rows.Scan(&id, &u, &title, &h, &anchor); err != nil {
			return err
		}
		if h != 0 {
			s.recentHash[uint64(h)] = anchor
		}
		if u != "" {
			s.knownURLs[NormalizeURL(u)] = id
		}
		s.rememberTitle(id, title, anchor)
	}
	return rows.Err()
}

// SetExtraction enables batch full-content extraction after each fetch.
// limit=0 disables it. workers is the pool size (0 = sane default).
func (s *Service) SetExtraction(limit, workers int) {
	s.extractor = extract.NewService(s.db)
	s.extractLimit = limit
	s.extractWorkers = workers
}

// FetchAll pulls every enabled source and persists new articles.
func (s *Service) FetchAll(ctx context.Context) (*Result, error) {
	start := time.Now()
	res := &Result{}

	sourcesList, err := db.NewSourceRepo(s.db).ListEnabled(ctx)
	if err != nil {
		return nil, fmt.Errorf("list enabled sources: %w", err)
	}

	// Fetch sources sequentially: deterministic, and each fetcher already does
	// its own network I/O. A later phase can parallelize per group.
	for _, src := range sourcesList {
		if ctx.Err() != nil {
			res.Errors = append(res.Errors, ctx.Err().Error())
			break
		}
		if s.logger != nil {
			s.logger.Printf("fetching %s (%s)", src.Name, src.Type)
		}
		items, err := s.man.Fetch(ctx, src)
		if err != nil {
			res.SourcesFailed++
			res.Errors = append(res.Errors, fmt.Sprintf("%s: %v", src.Name, err))
			if s.logger != nil {
				s.logger.Printf("source %s failed: %v", src.Name, err)
			}
			continue
		}
		res.SourcesFetched++

		now := db.Now()
		if err := db.NewSourceRepo(s.db).TouchFetch(ctx, src.ID, now); err != nil {
			res.Errors = append(res.Errors, fmt.Sprintf("%s: touch fetch: %v", src.Name, err))
		}

		inserted := 0
		for _, it := range items {
			n, err := s.ingest(ctx, src, it, now)
			if err != nil {
				res.Errors = append(res.Errors, fmt.Sprintf("%s/%s: %v", src.Name, it.Title, err))
				continue
			}
			switch n {
			case ingestNew:
				inserted++
			case ingestUpdated:
				res.Updated++
			case ingestGrouped:
				res.Grouped++
			case ingestSkipped:
				res.Duplicates++
			}
		}
		res.NewArticles += inserted
		if s.logger != nil {
			s.logger.Printf("source %s: %d new, %d updated, %d joined a story, %d skipped",
				src.Name, inserted, res.Updated, res.Grouped, res.Duplicates)
		}
	}

	res.ElapsedMs = time.Since(start).Milliseconds()

	// Batch extraction: enrich short-bodied articles (HN etc.) with their real
	// content so they are ready to read. Backfills existing short articles too.
	if s.extractor != nil && s.extractLimit > 0 && ctx.Err() == nil {
		done, err := s.extractor.ExtractPending(ctx, s.extractLimit, s.extractWorkers)
		if err != nil {
			res.Errors = append(res.Errors, fmt.Sprintf("extract: %v", err))
		} else if done > 0 && s.logger != nil {
			s.logger.Printf("extracted %d article bodies", done)
		}
		res.Extracted = done
	}
	return res, nil
}

type ingestKind int

const (
	ingestNew ingestKind = iota
	ingestUpdated
	ingestGrouped // another outlet's copy of a story already here
	ingestSkipped // the same article again, or too old to want
)

// ingest normalizes, groups and persists a single item.
func (s *Service) ingest(ctx context.Context, src *db.Source, it *sources.Item, now string) (ingestKind, error) {
	url := NormalizeURL(it.URL)
	if url == "" {
		return ingestSkipped, nil // no usable URL → skip
	}

	// Drop stale archive items (e.g. a blog feed exposing its whole history).
	if s.maxAge > 0 && !it.Published.IsZero() && time.Since(it.Published) > s.maxAge {
		return ingestSkipped, nil
	}

	// The same URL from two feeds is the same article, not a story: nothing to
	// keep a second copy of.
	s.mu.RLock()
	seenURL := s.knownURLs[url]
	s.mu.RUnlock()
	if seenURL != 0 {
		return ingestSkipped, nil
	}

	// A different URL with the same fingerprint is another outlet running the
	// same story. That copy is kept and filed under the first one: how many
	// outlets picked a story up is the strongest signal the feed produces, and
	// dropping the copies used to throw it away.
	tokens := TitleTokens(it.Title)
	sim := SimHash(strings.TrimSpace(it.Title) + " " + truncate(it.ContentMD, 400))
	s.mu.RLock()
	anchor := s.storyOf(tokens)
	if anchor == 0 && sim != 0 {
		anchor = s.nearDuplicateOf(sim) // the same document, republished
	}
	s.mu.RUnlock()

	na := db.NewArticle{
		SourceID:    src.ID,
		GUID:        it.GUID,
		URL:         url,
		Title:       it.Title,
		Author:      it.Author,
		ContentHTML: it.ContentHTML,
		ContentMD:   it.ContentMD,
		Status:      db.StatusUnread,
		SimHash:     sim,
		Published:   formatTime(it.Published),
	}
	if na.Title == "" {
		na.Title = url
	}

	repo := db.NewArticleRepo(s.db)
	id, created, err := repo.Upsert(ctx, na)
	if err != nil {
		return ingestNew, err
	}
	if anchor != 0 && anchor != id {
		if err := repo.JoinStory(ctx, id, anchor); err != nil {
			return ingestNew, err
		}
	}

	story := anchor
	if story == 0 {
		story = id
	}
	s.mu.Lock()
	s.rememberTitle(id, it.Title, story)
	if sim != 0 {
		// A copy points at the story, not at itself, so the next outlet to run
		// it lands in the same one.
		if anchor != 0 {
			s.recentHash[sim] = anchor
		} else {
			s.recentHash[sim] = id
		}
	}
	s.knownURLs[url] = id
	s.mu.Unlock()

	switch {
	case anchor != 0 && anchor != id:
		return ingestGrouped, nil
	case created:
		return ingestNew, nil
	}
	return ingestUpdated, nil
}

// storyOf finds the story a headline belongs to: the closest recent headline
// that passes SameStory, or 0 when nobody has reported it yet. Only articles
// sharing a word are considered, so the cost is in the tokens, not in the
// month of history. Caller holds the read lock.
func (s *Service) storyOf(tokens []string) int64 {
	if len(tokens) == 0 {
		return 0
	}
	candidates := map[int64]bool{}
	for _, w := range tokens {
		for _, id := range s.titleIndex[w] {
			candidates[id] = true
		}
	}
	best, bestScore := int64(0), 0.0
	for id := range candidates {
		entry, ok := s.titles[id]
		if !ok || !SameStory(tokens, entry.tokens) {
			continue
		}
		// The closest match wins, so a headline near two stories does not join
		// whichever one the map happened to yield first.
		if score := TitleSimilarity(tokens, entry.tokens); score > bestScore {
			best, bestScore = entry.anchor, score
		}
	}
	return best
}

// rememberTitle indexes a headline against the story it belongs to. Caller
// holds the write lock (or owns the service, during warm-up).
func (s *Service) rememberTitle(id int64, title string, anchor int64) {
	tokens := TitleTokens(title)
	if len(tokens) == 0 {
		return
	}
	if _, seen := s.titles[id]; seen {
		s.titles[id] = titleEntry{tokens: tokens, anchor: anchor}
		return
	}
	s.titles[id] = titleEntry{tokens: tokens, anchor: anchor}
	indexed := map[string]bool{}
	for _, w := range tokens {
		if indexed[w] {
			continue
		}
		indexed[w] = true
		s.titleIndex[w] = append(s.titleIndex[w], id)
	}
}

// nearDuplicateOf returns the story an article belongs to, or 0 when it is the
// first to report it. Caller holds the read lock.
func (s *Service) nearDuplicateOf(sim uint64) int64 {
	for h, anchor := range s.recentHash {
		if HammingDistance(sim, h) <= dedupHammingThreshold {
			return anchor
		}
	}
	return 0
}

func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n])
}

func formatTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339)
}
