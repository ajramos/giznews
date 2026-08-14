package classify

import (
	"context"
	"fmt"
	"log"

	"github.com/ajramos/giznews/internal/db"
	"github.com/ajramos/giznews/internal/llm"
)

// Options configures a classification run.
type Options struct {
	Limit     int // max articles per run (0 = config default)
	BatchSize int // articles per LLM call (0 = config default)
	AgeDays   int // only articles fetched within this window
	UseLLM    bool
	Model     string
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
	res := &Result{}

	rules, err := CompileAll(ctx, db.NewRuleRepo(s.db))
	if err != nil {
		return nil, fmt.Errorf("compile rules: %w", err)
	}

	// Phase 1: deterministic rules.
	var llmBatch []*db.Article
	for _, a := range articles {
		if actions := MatchFirst(rules, a); actions != nil {
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

// runBatches classifies articles in LLM batches. It returns the number
// successfully classified and a list of per-batch errors; a mid-run failure
// does not discard the work already persisted.
func (s *Service) runBatches(ctx context.Context, articles []*db.Article) (int, []string) {
	classified := 0
	repo := db.NewArticleRepo(s.db)
	var errs []string

	for start := 0; start < len(articles); start += s.opts.BatchSize {
		if ctx.Err() != nil {
			errs = append(errs, ctx.Err().Error())
			break
		}
		end := start + s.opts.BatchSize
		if end > len(articles) {
			end = len(articles)
		}
		batch := articles[start:end]

		s.logger.Printf("classifying batch %d/%d (%d articles)", start/s.opts.BatchSize+1, (len(articles)+s.opts.BatchSize-1)/s.opts.BatchSize, len(batch))
		results, err := BatchClassify(ctx, s.prov, s.opts.Model, batch)
		if err != nil {
			errs = append(errs, fmt.Sprintf("LLM batch %d: %v", start/s.opts.BatchSize+1, err))
			// Stop: later batches would almost certainly fail too.
			break
		}
		for _, a := range batch {
			c := results[a.ID]
			if c == nil {
				continue
			}
			if err := repo.ApplyClassification(ctx, a.ID, c.Category, c.Summary, c.Tags, c.Entities, c.Importance); err != nil {
				errs = append(errs, fmt.Sprintf("#%d: %v", a.ID, err))
				continue
			}
			classified++
		}
	}
	return classified, errs
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
