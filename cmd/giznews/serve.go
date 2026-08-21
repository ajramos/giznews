package main

import (
	"context"
	"fmt"
	"log"
	"os/signal"
	"syscall"
	"time"

	"github.com/ajramos/giznews/internal/pipeline"
)

// runServe keeps the pipeline running on its own, or — with --once — runs it a
// single time for cron to call.
func runServe(args []string, logger *log.Logger) {
	cfg, d, _ := loadAndOpenDB(args, logger)
	defer d.Close()

	// A signal has to reach the middle of a stage, so the loop can stop between
	// two of them rather than in the middle of a write.
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	opts := pipeline.ServeOptions{
		FetchEvery:    cfg.Serve.FetchInterval(),
		ClassifyEvery: cfg.Serve.ClassifyInterval(),
		KBEvery:       cfg.Serve.KBInterval(),
		IndexEvery:    cfg.Serve.IndexInterval(),
		DigestAt:      cfg.Serve.DigestAt,
		Once:          hasFlag(args, "once"),
	}

	runner := pipeline.New(cfg, d, logger)
	if opts.Once {
		fmt.Println("running the pipeline once")
	} else {
		fmt.Printf("giznews is running — Ctrl-C to stop\n")
	}
	started := time.Now()
	if err := runner.Serve(ctx, opts); err != nil {
		logger.Fatalf("serve: %v", err)
	}
	if opts.Once {
		fmt.Printf("done in %s\n", time.Since(started).Round(time.Second))
	}
}
