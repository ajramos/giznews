// Package pipeline runs the stages of giznews from one place, so a schedule and
// a person at a terminal are running exactly the same thing.
package pipeline

import (
	"context"
	"fmt"
	"log"

	"github.com/ajramos/giznews/internal/classify"
	"github.com/ajramos/giznews/internal/config"
	"github.com/ajramos/giznews/internal/db"
	"github.com/ajramos/giznews/internal/digest"
	"github.com/ajramos/giznews/internal/fetch"
	"github.com/ajramos/giznews/internal/kb"
	"github.com/ajramos/giznews/internal/llm"
	"github.com/ajramos/giznews/internal/search"
	"github.com/ajramos/giznews/internal/sources"
)

// Runner builds the services a stage needs and runs it, reporting what happened
// in one line — which is all a log of an unattended run should say.
type Runner struct {
	cfg    *config.Config
	db     *db.DB
	logger *log.Logger
}

// New creates a runner over an open database.
func New(cfg *config.Config, database *db.DB, logger *log.Logger) *Runner {
	return &Runner{cfg: cfg, db: database, logger: logger}
}

// provider builds the LLM client, or nil when the model is switched off or
// unreachable. A stage that needs it degrades; it never fails the run.
func (r *Runner) provider() llm.Provider {
	if !r.cfg.LLM.Enabled {
		return nil
	}
	prov, err := llm.NewProvider(llm.Options{
		Provider: r.cfg.LLM.Provider,
		Model:    r.cfg.LLM.Model,
		Endpoint: r.cfg.LLM.Endpoint,
		Region:   r.cfg.LLM.Region,
		APIKey:   r.cfg.LLM.APIKey,
		Timeout:  r.cfg.LLMTimeout(),
	})
	if err != nil {
		if r.logger != nil {
			r.logger.Printf("serve: llm unavailable, continuing without it: %v", err)
		}
		return nil
	}
	return prov
}

// Fetch pulls every enabled source and extracts what it can.
func (r *Runner) Fetch(ctx context.Context) (string, error) {
	man := sources.NewManager(
		r.cfg.ResolveGmailCredentialsPath(),
		r.cfg.ResolveGmailTokenPath(),
		r.cfg.Gmail.Queries,
		r.cfg.Gmail.MaxAge,
	)
	svc, err := fetch.NewService(r.db, man, r.logger)
	if err != nil {
		return "", err
	}
	if r.cfg.Extract.OnFetch {
		svc.SetExtraction(r.cfg.Extract.Limit, r.cfg.Extract.Concurrency)
	}
	res, err := svc.FetchAll(ctx)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%d new, %d joined a story, %d extracted, %d source(s) failed",
		res.NewArticles, res.Grouped, res.Extracted, res.SourcesFailed), nil
}

// Classify runs the prefilter and the model over what is waiting.
func (r *Runner) Classify(ctx context.Context) (string, error) {
	prov := r.provider()
	svc := classify.NewService(r.db, classify.Options{
		BatchSize:       r.cfg.Classify.BatchSize,
		Concurrency:     r.cfg.Classify.Concurrency,
		AgeDays:         14,
		CoverageSources: r.cfg.Classify.CoverageSources,
		CoverageFloor:   r.cfg.Classify.CoverageFloor,
		Learn:           r.cfg.Classify.Learn.Enabled,
		MaxDelta:        r.cfg.Classify.Learn.MaxDelta,
		UseLLM:          r.cfg.Classify.UseLLM && prov != nil,
		Model:           r.cfg.LLM.Model,
		Language:        r.cfg.LLM.Language,
	}, prov, r.logger)

	res, err := svc.ClassifyAll(ctx)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%d classified (%d by rules, %d by model)", res.Classified, res.ByRules, res.ByLLM), nil
}

// KB builds the graph: the reader's own notes, then atoms and electrons, then
// the themes, then the vault's entry points.
func (r *Runner) KB(ctx context.Context) (string, error) {
	svc, err := r.kbService()
	if err != nil {
		return "", err
	}
	res, err := svc.Build(ctx)
	if err != nil {
		return "", err
	}
	summary := fmt.Sprintf("%d atom(s), %d electron(s), %d theme(s)",
		res.AtomsCreated, res.ElectronsCreated, res.MoleculesCreated+res.MoleculesUpdated)
	if _, err := svc.BuildIndex(ctx); err != nil {
		return summary, fmt.Errorf("index: %w", err)
	}
	return summary, nil
}

// Digest writes the day's digest.
func (r *Runner) Digest(ctx context.Context) (string, error) {
	prov := r.provider()
	svc := digest.NewService(r.db, digest.Options{
		Model:               r.cfg.LLM.Model,
		Days:                7,
		MaxArticlesPerTheme: r.cfg.Digest.MaxArticlesPerTheme,
		UseLLM:              prov != nil,
		Language:            r.cfg.LLM.Language,
	}, prov, r.logger)
	dig, err := svc.Generate(ctx)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s, %d theme(s)", dig.Date, len(dig.Themes)), nil
}

// Index refreshes the search index and its embeddings.
func (r *Runner) Index(ctx context.Context) (string, error) {
	svc, err := search.NewService(r.db, r.provider(), search.Options{Model: r.cfg.LLM.EmbeddingModel}, r.logger)
	if err != nil {
		return "", err
	}
	res, err := svc.Index(ctx)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%d note(s) and %d article(s) indexed, %d embedded, %d failed",
		res.FTSNotes, res.FTSArticles, res.NotesEmbedded+res.ArticlesEmbedded, res.EmbeddingsFailed), nil
}

func (r *Runner) kbService() (*kb.Service, error) {
	prov := r.provider()
	return kb.NewService(r.db, r.cfg.ResolveVaultPath(), kb.Options{
		ImportanceThreshold: r.cfg.Classify.ImportanceThreshold,
		MinOccurrences:      r.cfg.KB.MinOccurrences,
		AgeDays:             r.cfg.KB.AgeDays,
		ThemeDays:           r.cfg.KB.ThemeDays,
		Limit:               r.cfg.KB.Limit,
		Model:               r.cfg.LLM.Model,
		UseLLM:              prov != nil,
		Language:            r.cfg.LLM.Language,
	}, prov, r.logger)
}
