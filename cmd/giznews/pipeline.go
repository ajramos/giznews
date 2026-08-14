package main

import (
	"fmt"
	"log"

	"github.com/ajramos/giznews/internal/classify"
	"github.com/ajramos/giznews/internal/digest"
	"github.com/ajramos/giznews/internal/llm"
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
		BatchSize: cfg.Classify.BatchSize,
		AgeDays:   14,
		UseLLM:    cfg.Classify.UseLLM && prov != nil,
		Model:     cfg.LLM.Model,
	}, prov, logger)

	res, err := svc.ClassifyAll(ctx)
	if err != nil {
		logger.Fatalf("classify: %v", err)
	}
	fmt.Printf("classified %d articles (rules: %d, llm: %d, skipped-no-llm: %d)\n",
		res.Classified, res.ByRules, res.ByLLM, res.SkippedNoLLM)
	for _, e := range res.Errors {
		fmt.Printf("  ! %s\n", e)
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
