// Package extract fetches the original article URL and extracts its readable
// body (readability → markdown), caching it in content_md. It is used both on
// demand (opening an article) and in batch during fetch.
package extract

import (
	"context"
	"fmt"
	"runtime"
	"sync"
	"time"

	"github.com/ajramos/giznews/internal/db"
	"github.com/ajramos/giznews/internal/sources"
	"github.com/go-shiori/go-readability"
)

// MinLength is the body-length threshold below which an article is considered
// to have no real content (HN feeds only carry titles + comment links).
const MinLength = 200

// Service extracts and stores article bodies.
type Service struct {
	db *db.DB
}

// NewService builds an extractor.
func NewService(database *db.DB) *Service {
	return &Service{db: database}
}

// ExtractArticle fetches art.URL, extracts the readable content, converts it to
// markdown and persists it. Marks the article as extracted on success.
func (s *Service) ExtractArticle(ctx context.Context, art *db.Article) error {
	if art == nil || art.URL == "" {
		return fmt.Errorf("extract: no url")
	}
	parsed, err := readability.FromURL(art.URL, 15*time.Second)
	if err != nil {
		return fmt.Errorf("extract %s: %w", art.URL, err)
	}
	md := sources.HTMLToMarkdown(parsed.Content)
	if len([]rune(md)) < MinLength {
		return fmt.Errorf("extract %s: no readable content", art.URL)
	}
	repo := db.NewArticleRepo(s.db)
	if err := repo.SetContent(ctx, art.ID, parsed.Content, md); err != nil {
		return err
	}
	return repo.SetExtracted(ctx, art.ID, true)
}

// ExtractPending extracts up to limit short, un-extracted articles using a
// worker pool. Returns how many succeeded.
func (s *Service) ExtractPending(ctx context.Context, limit, workers int) (int, error) {
	if limit <= 0 {
		return 0, nil
	}
	if workers <= 0 {
		workers = runtime.NumCPU()
		if workers > 4 {
			workers = 4
		}
	}
	pending, err := db.NewArticleRepo(s.db).ListPendingExtract(ctx, limit)
	if err != nil {
		return 0, err
	}
	if len(pending) == 0 {
		return 0, nil
	}

	jobCh := make(chan *db.Article)
	var wg sync.WaitGroup
	var mu sync.Mutex
	done := 0

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for art := range jobCh {
				if ctx.Err() != nil {
					return
				}
				if err := s.ExtractArticle(ctx, art); err != nil {
					continue // leave un-extracted; retried on a later run
				}
				mu.Lock()
				done++
				mu.Unlock()
			}
		}()
	}

	for _, art := range pending {
		select {
		case jobCh <- art:
		case <-ctx.Done():
			break
		}
	}
	close(jobCh)
	wg.Wait()
	return done, nil
}
