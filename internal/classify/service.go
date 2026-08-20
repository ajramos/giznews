package classify

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/ajramos/giznews/internal/db"
	"github.com/ajramos/giznews/internal/llm"
)

// Options configures a classification run.
type Options struct {
	Limit     int // max articles per run (0 = config default)
	BatchSize int // articles per LLM call (0 = config default)
	Concurrency int // parallel LLM batches (0 = sequential)
	AgeDays   int // only articles fetched within this window
	UseLLM    bool
	Model     string
	Language  string // ISO 639-1 for LLM-generated summaries/headlines
	// OnProgress, when set, reports phase progress ("rules" or "llm").
	OnProgress func(phase string, done, total int)
}

// Service classifies unread articles: rules first, then LLM batches.
type Service struct {
	db     *db.DB
	opts   Options
	prov   llm.Provider
	logger *log.Logger
}

// NewService builds a classifier. If UseLLM is set, provider must be non-nil
// (or the caller passes one that is constructed lazily).
func NewService(database *db.DB, opts Options, prov llm.Provider, logger *log.Logger) *Service {
	if opts.BatchSize <= 0 {
		opts.BatchSize = 20
	}
	if opts.AgeDays <= 0 {
		opts.AgeDays = 14
	}
	if opts.Limit <= 0 {
		opts.Limit = 200
	}
	return &Service{db: database, opts: opts, prov: prov, logger: logger}
}

// ClassifyAll classifies pending articles and returns a summary of what ran.
func (s *Service) ClassifyAll(ctx context.Context) (*Result, error) {
	repo := db.NewArticleRepo(s.db)
	articles, err := repo.ListUnclassified(ctx, s.opts.Limit, s.opts.AgeDays)
	if err != nil {
		return nil, fmt.Errorf("list unclassified: %w", err)
	}
	return s.classify(ctx, articles)
}

// ClassifyIDs classifies an explicit set of articles (bulk selection), letting
// the user prioritize specific items regardless of the unclassified queue.
func (s *Service) ClassifyIDs(ctx context.Context, ids []int64) (*Result, error) {
	repo := db.NewArticleRepo(s.db)
	articles, err := repo.GetByIDs(ctx, ids)
	if err != nil {
		return nil, fmt.Errorf("get articles by ids: %w", err)
	}
	return s.classify(ctx, articles)
}

// classify runs rules + LLM over the given articles and returns a summary.
func (s *Service) classify(ctx context.Context, articles []*db.Article) (*Result, error) {
	repo := db.NewArticleRepo(s.db)
	res := &Result{}

	rules, err := CompileAll(ctx, db.NewRuleRepo(s.db))
	if err != nil {
		return nil, fmt.Errorf("compile rules: %w", err)
	}

	// Phase 1: deterministic rules.
	var llmBatch []*db.Article
	for _, a := range articles {
		if actions := MatchFirst(rules, a); actions != nil && !actions.Keep {
			if err := s.apply(ctx, repo, a, actions); err != nil {
				res.Errors = append(res.Errors, fmt.Sprintf("#%d: %v", a.ID, err))
				continue
			}
			res.ByRules++
			res.Classified++
			continue
		}
		llmBatch = append(llmBatch, a)
	}

	s.progress("rules", len(articles), len(articles))

	// Phase 2: LLM.
	if s.opts.UseLLM && s.prov != nil && len(llmBatch) > 0 {
		classified, batchErrs := s.runBatches(ctx, llmBatch)
		res.Errors = append(res.Errors, batchErrs...)
		res.ByLLM = classified
		res.Classified += classified
	} else if len(llmBatch) > 0 {
		// No LLM available: mark them with a deterministic default so they are
		// not re-picked next run, but still get a baseline importance.
		for _, a := range llmBatch {
			imp := defaultImportance(a)
			if err := repo.ApplyClassification(ctx, a.ID, "general", "", nil, nil, imp); err != nil {
				res.Errors = append(res.Errors, fmt.Sprintf("#%d: %v", a.ID, err))
				continue
			}
			res.SkippedNoLLM++
			res.Classified++
		}
	}

	return res, nil
}

// runBatches classifies articles in LLM batches (optionally in parallel). It
// returns the number successfully classified and a list of per-batch errors.
// A mid-run batch failure is logged and skipped — it never discards the work
// already persisted nor aborts the remaining batches.
func (s *Service) runBatches(ctx context.Context, articles []*db.Article) (int, []string) {
	repo := db.NewArticleRepo(s.db)
	totalBatches := (len(articles) + s.opts.BatchSize - 1) / s.opts.BatchSize
	if totalBatches <= 0 {
		return 0, nil
	}

	type batchJob struct {
		idx  int // 0-based
		from int
		to   int
	}
	jobs := make([]batchJob, 0, totalBatches)
	for start := 0; start < len(articles); start += s.opts.BatchSize {
		end := start + s.opts.BatchSize
		if end > len(articles) {
			end = len(articles)
		}
		jobs = append(jobs, batchJob{idx: len(jobs), from: start, to: end})
	}

	workers := s.opts.Concurrency
	if workers <= 0 {
		workers = 1
	}
	if workers > len(jobs) {
		workers = len(jobs)
	}

	var (
		mu           sync.Mutex
		classified   int
		articlesDone int
		errs         []string
	)

	ch := make(chan batchJob)
	var wg sync.WaitGroup
	wg.Add(workers)
	for w := 0; w < workers; w++ {
		go func() {
			defer wg.Done()
			for j := range ch {
				if ctx.Err() != nil {
					continue
				}
				batch := articles[j.from:j.to]
				start := time.Now()
				s.logger.Printf("classifying batch %d/%d (%d articles)", j.idx+1, totalBatches, len(batch))

				results, err := BatchClassify(ctx, s.prov, s.opts.Model, s.opts.Language, batch)
				if err != nil {
					mu.Lock()
					errs = append(errs, fmt.Sprintf("LLM batch %d: %v", j.idx+1, err))
					mu.Unlock()
					s.logger.Printf("batch %d/%d failed: %v", j.idx+1, totalBatches, err)
				} else {
					n := 0
					for _, a := range batch {
						c := results[a.ID]
						if c == nil {
							continue
						}
						if err := repo.ApplyClassification(ctx, a.ID, c.Category, c.Summary, c.Tags, c.Entities, c.Importance); err != nil {
							mu.Lock()
							errs = append(errs, fmt.Sprintf("#%d: %v", a.ID, err))
							mu.Unlock()
							continue
						}
						n++
					}
					mu.Lock()
					classified += n
					mu.Unlock()
					s.logger.Printf("batch %d/%d: %d classified in %v", j.idx+1, totalBatches, n, time.Since(start).Round(time.Millisecond))
				}

				mu.Lock()
				articlesDone += len(batch)
				done := articlesDone
				mu.Unlock()
				s.progress("llm", done, len(articles))
			}
		}()
	}
	for _, j := range jobs {
		ch <- j
	}
	close(ch)
	wg.Wait()

	return classified, errs
}

// progress forwards a phase update to the optional callback.
func (s *Service) progress(phase string, done, total int) {
	if s.opts.OnProgress != nil {
		s.opts.OnProgress(phase, done, total)
	}
}

// apply persists rule-based classification for one article.
func (s *Service) apply(ctx context.Context, repo *db.ArticleRepo, a *db.Article, actions *RuleActions) error {
	imp := actions.Importance
	if imp == 0 {
		imp = defaultImportance(a)
	}
	status := a.Status
	if actions.Archive {
		status = db.StatusArchived
		if err := repo.SetStatus(ctx, a.ID, status); err != nil {
			return err
		}
	}
	category := actions.Category
	if category == "" {
		category = "general"
	}
	return repo.ApplyClassification(ctx, a.ID, category, a.Summary, actions.Tags, nil, imp)
}
