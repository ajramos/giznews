package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/ajramos/giznews/internal/config"

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

	var err error
	svc := digest.NewService(d, digest.Options{
		Model:               cfg.LLM.Model,
		Days:                7,
		MaxArticlesPerTheme: cfg.Digest.MaxArticlesPerTheme,
		UseLLM:              prov != nil,
	}, prov, logger)

	// A stored digest can be exported, re-read or mailed later without asking
	// the model to write it a second time.
	date := flagValue(args, "date")
	var dig *digest.Digest
	if date != "" {
		dig, err = digest.Load(ctx, d, date)
		if err != nil {
			logger.Fatalf("digest %s: %v — nothing was written for that day", date, err)
		}
	} else {
		dig, err = svc.Generate(ctx)
		if err != nil {
			logger.Fatalf("digest: %v", err)
		}
		if err := digest.Save(ctx, d, dig); err != nil {
			logger.Printf("digest: could not store it: %v", err)
		}
	}

	if out := flagValue(args, "out"); out != "" {
		if err := exportDigest(dig, out, flagValue(args, "format")); err != nil {
			logger.Fatalf("digest --out: %v", err)
		}
		fmt.Printf("digest %s written to %s\n", dig.Date, out)
	}
	if hasFlag(args, "send") {
		// Sending is the one thing here that leaves the machine, so it happens
		// only when asked, and only when somebody configured where to.
		if err := digest.Send(smtpFromConfig(cfg), "AI digest · "+dig.Date, digest.HTML(dig)); err != nil {
			logger.Fatalf("digest --send: %v", err)
		}
		fmt.Printf("digest %s sent to %s\n", dig.Date, strings.Join(cfg.Digest.SMTP.To, ", "))
	}
	if flagValue(args, "out") != "" || hasFlag(args, "send") {
		return
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

// exportDigest writes a digest to a file. The format follows the extension
// unless it is given, so `--out digest.html` means what it looks like.
func exportDigest(d *digest.Digest, path, format string) error {
	if format == "" {
		switch {
		case strings.HasSuffix(strings.ToLower(path), ".html"), strings.HasSuffix(strings.ToLower(path), ".htm"):
			format = "html"
		default:
			format = "md"
		}
	}
	var body string
	switch strings.ToLower(format) {
	case "md", "markdown":
		body = digest.Markdown(d)
	case "html":
		body = digest.HTML(d)
	default:
		return fmt.Errorf("unknown format %q (expected md or html)", format)
	}
	if dir := filepath.Dir(path); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	return os.WriteFile(path, []byte(body), 0o644)
}

// smtpFromConfig maps the config file's shape onto the sender's.
func smtpFromConfig(cfg *config.Config) digest.SMTP {
	s := cfg.Digest.SMTP
	return digest.SMTP{
		Host: s.Host, Port: s.Port, From: s.From, To: s.To,
		Username: s.Username, Password: s.Password, StartTLS: s.StartTLS,
	}
}
