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
// considered near-duplicates.
const dedupHammingThreshold = 3

// Result reports what a fetch run did.
type Result struct {
	NewArticles    int      `json:"new_articles"`
	Updated        int      `json:"updated"`
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
	db    *db.DB
	man   *sources.Manager
	logger *log.Logger

	extractor *extract.Service
	extractLimit    int
	extractWorkers  int

	mu         sync.RWMutex
	recentHash map[uint64]bool
	knownURLs  map[string]bool
}

// NewService builds a fetch pipeline service.
func NewService(database *db.DB, man *sources.Manager, logger *log.Logger) (*Service, error) {
	s := &Service{
		db:         database,
		man:        man,
		logger:     logger,
		recentHash: map[uint64]bool{},
		knownURLs:  map[string]bool{},
	}
	if err := s.warmDedup(); err != nil {
		return nil, fmt.Errorf("warm dedup cache: %w", err)
	}
	return s, nil
}

// warmDedup preloads recent article fingerprints so cross-source duplicates
// within the window are caught even before the pipeline inserts them.
func (s *Service) warmDedup() error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cutoff := time.Now().Add(-30 * 24 * time.Hour).UTC().Format(time.RFC3339)
	rows, err := s.db.SQL().QueryContext(ctx, `
		SELECT url, simhash FROM articles WHERE simhash != 0 AND fetched_at >= ?`, cutoff)
	if err != nil {
		return err
	}
	defer rows.Close()

	s.mu.Lock()
	defer s.mu.Unlock()
	for rows.Next() {
		var (
			u string
			h int64
		)
		if err := rows.Scan(&u, &h); err != nil {
			return err
		}
		if h != 0 {
			s.recentHash[uint64(h)] = true
		}
		if u != "" {
			s.knownURLs[NormalizeURL(u)] = true
		}
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
			case ingestDuplicate:
				res.Duplicates++
			}
		}
		res.NewArticles += inserted
		if s.logger != nil {
			s.logger.Printf("source %s: %d new, %d updated, %d dups",
				src.Name, inserted, res.Updated, res.Duplicates)
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
	ingestDuplicate
)

// ingest normalizes, de-duplicates and persists a single item.
func (s *Service) ingest(ctx context.Context, src *db.Source, it *sources.Item, now string) (ingestKind, error) {
	url := NormalizeURL(it.URL)
	if url == "" {
		return ingestDuplicate, nil // no usable URL → skip
	}

	// Cross-source dedup by URL.
	s.mu.RLock()
	seenURL := s.knownURLs[url]
	s.mu.RUnlock()
	if seenURL {
		return ingestDuplicate, nil
	}

	// Cross-source dedup by simhash over title + first bit of content.
	sim := SimHash(strings.TrimSpace(it.Title) + " " + truncate(it.ContentMD, 400))
	if sim != 0 {
		s.mu.RLock()
		dup := s.isNearDuplicate(sim)
		s.mu.RUnlock()
		if dup {
			return ingestDuplicate, nil
		}
	}

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

	id, created, err := db.NewArticleRepo(s.db).Upsert(ctx, na)
	if err != nil {
		return ingestNew, err
	}
	_ = id

	s.mu.Lock()
	if sim != 0 {
		s.recentHash[sim] = true
	}
	s.knownURLs[url] = true
	s.mu.Unlock()

	if created {
		return ingestNew, nil
	}
	return ingestUpdated, nil
}

// isNearDuplicate reports whether sim is within threshold of a cached hash.
// Caller holds the read lock.
func (s *Service) isNearDuplicate(sim uint64) bool {
	for h := range s.recentHash {
		if HammingDistance(sim, h) <= dedupHammingThreshold {
			return true
		}
	}
	return false
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
