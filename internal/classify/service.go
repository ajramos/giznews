package classify

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/ajramos/giznews/internal/db"
	"github.com/ajramos/giznews/internal/learn"
	"github.com/ajramos/giznews/internal/llm"
)

// Options configures a classification run.
type Options struct {
	Limit       int // max articles per run (0 = config default)
	BatchSize   int // articles per LLM call (0 = config default)
	Concurrency int // parallel LLM batches (0 = sequential)
	AgeDays     int // only articles fetched within this window
	// CoverageSources is how many outlets running the same story make it
	// important on its own; CoverageFloor is the importance it then gets at
	// least. 0 sources disables it.
	CoverageSources int
	CoverageFloor   int
	// Learn applies what `giznews learn` worked out from the reader's own
	// history; MaxDelta bounds how far it may move anything.
	Learn    bool
	MaxDelta int
	UseLLM   bool
	Model    string
	Language string // ISO 639-1 for LLM-generated summaries/headlines
	// RulesOnly applies the deterministic rules and stops, leaving every
	// article no rule resolved — keep, boost and unmatched — unclassified for
	// a later LLM run. Boost and coverage floors are not applied here: they are
	// a floor over what the model decides, and the model has not run.
	RulesOnly bool
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
	limit := s.opts.Limit
	if s.opts.RulesOnly {
		// Rules are a deterministic sweep, not an LLM batch: applying them to a
		// thousand articles costs as much as to two hundred. Take the whole
		// queue, whatever the caller passed as the per-run cap.
		n, err := repo.CountUnclassified(ctx)
		if err != nil {
			return nil, err
		}
		limit = n
	}
	articles, err := repo.ListUnclassified(ctx, limit, s.opts.AgeDays)
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
	floors := map[int64]int{} // article -> importance floor a boost rule gave it
	for _, a := range articles {
		d := Decide(rules, a)
		if d.WatchedBy != "" {
			// Recorded before anything else decides the article's fate: being
			// told about something you asked to be told about does not depend
			// on what the rest of the chain does with it.
			first, err := db.NewWatchRepo(s.db).Record(ctx, a.ID, d.WatchedBy)
			if err != nil {
				res.Errors = append(res.Errors, fmt.Sprintf("#%d: watch: %v", a.ID, err))
			} else if first {
				res.Watched++
			}
		}
		floor := d.Floor
		if c := s.coverageFloor(a); c > floor {
			floor = c
			res.ByCoverage++
		}
		if floor > 0 {
			floors[a.ID] = floor
		}
		if floor > 0 {
			llmBatch = append(llmBatch, a) // covered widely, or boosted: the model still sees it
			continue
		}
		if !d.ToLLM() {
			if d.Action.Archive {
				res.Archived++
			}
			if err := s.apply(ctx, repo, a, d.Action); err != nil {
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

	// A rules-only run stops here: the articles no rule resolved stay pending,
	// and no floor is applied — that happens once the model has classified them.
	if s.opts.RulesOnly {
		res.Pending = len(llmBatch)
		return res, nil
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

	// Importance is settled last, over whatever the model (or the fallback)
	// decided: what the reader has taught the app, then the floors a rule set.
	learned, err := s.adjustments(ctx)
	if err != nil {
		res.Errors = append(res.Errors, err.Error())
	}
	boosted, adjusted, err := s.settleImportance(ctx, repo, llmBatch, floors, learned)
	if err != nil {
		res.Errors = append(res.Errors, err.Error())
	}
	res.Boosted, res.Adjusted = boosted, adjusted

	return res, nil
}

// coverageFloor is the importance a story earns purely by how many outlets ran
// it. Six newsrooms deciding the same morning that something matters is a
// judgement no regex can fake and no summary can convey.
func (s *Service) coverageFloor(a *db.Article) int {
	if s.opts.CoverageSources <= 0 || s.opts.CoverageFloor <= 0 {
		return 0
	}
	if a.StorySize < s.opts.CoverageSources {
		return 0
	}
	return s.opts.CoverageFloor
}

// adjustments loads what the reader has taught the app. Nothing is applied
// until somebody runs `giznews learn`, and turning Learn off ignores it without
// throwing it away.
func (s *Service) adjustments(ctx context.Context) (learn.Adjustments, error) {
	if !s.opts.Learn {
		return nil, nil
	}
	return learn.Load(ctx, s.db)
}

// settleImportance is the last word on how important an article is, applied
// once the model has had its say.
//
// The order is the argument. What the reader has taught the app moves the
// model's number by at most one step, in either direction; then a floor from a
// rule or from coverage raises it if it is still too low. So a rule someone
// wrote on purpose always beats a habit inferred from history, and history
// beats nothing but the model's guess.
func (s *Service) settleImportance(ctx context.Context, repo *db.ArticleRepo, batch []*db.Article,
	floors map[int64]int, learned learn.Adjustments) (boosted, adjusted int, err error) {

	for _, a := range batch {
		floor := floors[a.ID]
		if floor == 0 && len(learned) == 0 {
			continue
		}
		current, err := repo.Get(ctx, a.ID)
		if err != nil {
			return boosted, adjusted, fmt.Errorf("importance #%d: %w", a.ID, err)
		}

		want := current.Importance
		if delta := learned.For(current.SourceID, current.Tags, s.opts.MaxDelta); delta != 0 {
			want = clampImportance(want + delta)
		}
		moved := want != current.Importance
		if want < floor {
			want = floor // a rule said so, and a rule outranks a habit
		}
		if want == current.Importance {
			continue
		}
		if err := repo.SetImportance(ctx, a.ID, want); err != nil {
			return boosted, adjusted, fmt.Errorf("importance #%d: %w", a.ID, err)
		}
		if moved {
			adjusted++
		}
		if floor > 0 && want == floor && current.Importance < floor {
			boosted++
		}
	}
	return boosted, adjusted, nil
}

func clampImportance(n int) int {
	switch {
	case n < 0:
		return 0
	case n > 3:
		return 3
	}
	return n
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
		// A rule archiving something is the pipeline's decision, not the
		// reader's: it must never come back as a preference.
		if err := repo.SetStatus(ctx, a.ID, status, db.ActorSystem); err != nil {
			return err
		}
	}
	category := actions.Category
	if category == "" {
		category = "general"
	}
	return repo.ApplyClassification(ctx, a.ID, category, a.Summary, actions.Tags, nil, imp)
}
