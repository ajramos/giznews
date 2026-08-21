package main

import (
	"context"
	"fmt"
	"log"

	"github.com/ajramos/giznews/internal/fetch"
	"github.com/ajramos/giznews/internal/sources"
)

// runFetch pulls new articles from every enabled source.
func runFetch(args []string, logger *log.Logger) {
	cfg, d, ctx := loadAndOpenDB(args, logger)
	defer d.Close()

	man := sources.NewManager(
		cfg.ResolveGmailCredentialsPath(),
		cfg.ResolveGmailTokenPath(),
		cfg.Gmail.Queries,
		cfg.Gmail.MaxAge,
	)
	svc, err := fetch.NewService(d, man, logger)
	if err != nil {
		logger.Fatalf("fetch service: %v", err)
	}
	if cfg.Extract.OnFetch {
		svc.SetExtraction(cfg.Extract.Limit, cfg.Extract.Concurrency)
	}

	res, err := svc.FetchAll(ctx)
	if err != nil {
		logger.Fatalf("fetch: %v", err)
	}

	fmt.Printf("fetch complete in %dms\n", res.ElapsedMs)
	fmt.Printf("  new: %d   updated: %d   joined a story: %d   skipped: %d   extracted: %d\n",
		res.NewArticles, res.Updated, res.Grouped, res.Duplicates, res.Extracted)
	fmt.Printf("  sources ok: %d   failed: %d\n", res.SourcesFetched, res.SourcesFailed)
	for _, e := range res.Errors {
		fmt.Printf("  ! %s\n", e)
	}
}

// runGmailAuth runs the interactive OAuth flow and saves the token.
func runGmailAuth(args []string, logger *log.Logger) {
	cfg, err := loadConfig(args)
	if err != nil {
		logger.Fatalf("config: %v", err)
	}
	auth := &sources.GmailAuth{
		CredentialsPath: cfg.ResolveGmailCredentialsPath(),
		TokenPath:       cfg.ResolveGmailTokenPath(),
		Logger:          logger,
	}
	ctx := context.Background()
	if err := auth.Authorize(ctx); err != nil {
		logger.Fatalf("gmail auth: %v", err)
	}
	fmt.Printf("authorized — token saved to %s\n", cfg.ResolveGmailTokenPath())
}
