package main

import (
	"context"
	"fmt"
	"log"

	"github.com/ajramos/giznews/internal/classify"
	"github.com/ajramos/giznews/internal/digest"
	"github.com/ajramos/giznews/internal/llm"
	"github.com/ajramos/giznews/internal/pipeline"
)

// buildProvider creates the LLM provider from config (nil when disabled).
func runClassify(args []string, logger *log.Logger) {
	cfg, d, ctx := loadAndOpenDB(args, logger)
	defer d.Close()

	prov, err := llm.NewProvider(llm.Options{
		Provider: cfg.LLM.Provider,
		Model:    cfg.LLM.Model,
		Endpoint: cfg.LLM.Endpoint,
		Region:   cfg.LLM.Region,
		APIKey:   cfg.LLM.APIKey,
		Timeout:  cfg.LLMTimeout(),
	})
	if err != nil {
		logger.Fatalf("llm: %v", err)
	}
	if !cfg.LLM.Enabled {
		prov = nil
	}

	svc := classify.NewService(d, classify.Options{
		BatchSize:       cfg.Classify.BatchSize,
		AgeDays:         14,
		CoverageSources: cfg.Classify.CoverageSources,
		CoverageFloor:   cfg.Classify.CoverageFloor,
		Learn:           cfg.Classify.Learn.Enabled,
		MaxDelta:        cfg.Classify.Learn.MaxDelta,
		UseLLM:          cfg.Classify.UseLLM && prov != nil,
		Model:           cfg.LLM.Model,
	}, prov, logger)

	if hasFlag(args, "dry-run") {
		plan, err := svc.Preview(ctx)
		if err != nil {
			logger.Fatalf("classify --dry-run: %v", err)
		}
		printClassifyPlan(plan)
		return
	}

	var res *classify.Result
	if err := pipeline.WithLock(ctx, d, logger, func(ctx context.Context) error {
		var err error
		res, err = svc.ClassifyAll(ctx)
		return err
	}); err != nil {
		logger.Fatalf("classify: %v", err)
	}
	fmt.Printf("classified %d articles (rules: %d, llm: %d, skipped-no-llm: %d)\n",
		res.Classified, res.ByRules, res.ByLLM, res.SkippedNoLLM)
	if res.ByCoverage > 0 {
		fmt.Printf("  %d story(ies) met the coverage floor\n", res.ByCoverage)
	}
	if res.Boosted > 0 {
		fmt.Printf("  %d article(s) raised to their floor\n", res.Boosted)
	}
	if res.Adjusted > 0 {
		fmt.Printf("  %d article(s) moved by what your reading taught it\n", res.Adjusted)
	}
	for _, e := range res.Errors {
		fmt.Printf("  ! %s\n", e)
	}
}

// printClassifyPlan renders a dry run: which rule claims what, and how much is
// left for the model.
func printClassifyPlan(p *classify.Plan) {
	fmt.Printf("dry run — nothing was classified, archived or written\n")
	fmt.Printf("%d article(s) waiting\n\n", p.Pending)

	if len(p.Rules) == 0 {
		fmt.Println("no rules — everything goes to the model")
	}
	for _, r := range p.Rules {
		fmt.Printf("  %-9s %-34s %3d article(s)\n", r.Effect, truncate(r.Name, 34), r.Matches)
		for _, title := range r.Sample {
			fmt.Printf("           · %s\n", truncate(title, 62))
		}
	}

	fmt.Printf("\n%d resolved by rules, %d of them archived — these never reach the model,\n", p.ByRules, p.Archived)
	fmt.Printf("and never get a summary or entities either.\n")
	if p.Kept > 0 {
		fmt.Printf("%d protected by a keep rule: they go to the model untouched.\n", p.Kept)
	}
	if p.Boosted > 0 {
		fmt.Printf("%d boosted: classified by the model, then raised to their floor.\n", p.Boosted)
	}
	if p.Covered > 0 {
		fmt.Printf("%d raised by coverage: enough outlets ran the same story.\n", p.Covered)
	}
	if p.Learned > 0 {
		fmt.Printf("%d would be moved by what your reading taught it (%d up, %d down).\n",
			p.Learned, p.LearnedUp, p.Learned-p.LearnedUp)
	}
	fmt.Printf("\n%d would go to the model, in %d batch(es):\n", p.ToLLM, p.Batches)
	for _, title := range p.Unmatched {
		fmt.Printf("  · %s\n", truncate(title, 62))
	}
}

func runDigest(args []string, logger *log.Logger) {
	cfg, d, ctx := loadAndOpenDB(args, logger)
	defer d.Close()

	var prov llm.Provider
	if cfg.LLM.Enabled {
		var err error
		prov, err = llm.NewProvider(llm.Options{
			Provider: cfg.LLM.Provider,
			Model:    cfg.LLM.Model,
			Endpoint: cfg.LLM.Endpoint,
			Region:   cfg.LLM.Region,
			APIKey:   cfg.LLM.APIKey,
			Timeout:  cfg.LLMTimeout(),
		})
		if err != nil {
			logger.Fatalf("llm: %v", err)
		}
	}

	svc := digest.NewService(d, digest.Options{
		Model:               cfg.LLM.Model,
		Days:                7,
		MaxArticlesPerTheme: cfg.Digest.MaxArticlesPerTheme,
		UseLLM:              prov != nil,
	}, prov, logger)

	dig, err := svc.Generate(ctx)
	if err != nil {
		logger.Fatalf("digest: %v", err)
	}

	fmt.Printf("\n=== AI Digest %s ===\n\n", dig.Date)
	if dig.Overview != "" {
		fmt.Printf("%s\n\n", dig.Overview)
	}
	for _, th := range dig.Themes {
		fmt.Printf("## %s\n", th.Name)
		if th.Summary != "" {
			fmt.Printf("%s\n", th.Summary)
		}
		for _, a := range th.Articles {
			stars := "☆☆☆"
			if a.Importance >= 1 {
				stars = "★☆☆"
			}
			if a.Importance >= 2 {
				stars = "★★☆"
			}
			if a.Importance >= 3 {
				stars = "★★★"
			}
			fmt.Printf("  [%s] %s — %s\n", stars, a.Title, a.SourceName)
		}
		fmt.Println()
	}
}
