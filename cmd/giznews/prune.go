package main

import (
	"fmt"
	"log"
	"strconv"

	"github.com/ajramos/giznews/internal/prune"
)

// runPrune reclaims what an archive of read news does not need.
func runPrune(args []string, logger *log.Logger) {
	cfg, d, ctx := loadAndOpenDB(args, logger)
	defer d.Close()

	opts := prune.Options{
		BodyDays: cfg.Prune.BodyDays,
		RowDays:  cfg.Prune.RowDays,
	}
	if v := flagValue(args, "older-than"); v != "" {
		days, err := parseDays(v)
		if err != nil {
			logger.Fatalf("prune: --older-than %q: %v", v, err)
		}
		opts.BodyDays = days
	}
	if v := flagValue(args, "delete-after"); v != "" {
		days, err := parseDays(v)
		if err != nil {
			logger.Fatalf("prune: --delete-after %q: %v", v, err)
		}
		opts.RowDays = days
	}

	if hasFlag(args, "dry-run") {
		plan, err := prune.Preview(ctx, d, opts)
		if err != nil {
			logger.Fatalf("prune --dry-run: %v", err)
		}
		printPrunePlan(plan)
		fmt.Println("\ndry run — nothing was deleted")
		return
	}

	res, err := prune.Apply(ctx, d, opts, logger)
	if err != nil {
		logger.Fatalf("prune: %v", err)
	}
	printPrunePlan(&res.Plan)
	fmt.Printf("\ndatabase: %s → %s (%s reclaimed)\n",
		humanBytes(res.SizeBefore), humanBytes(res.SizeAfter), humanBytes(res.SizeBefore-res.SizeAfter))
}

func printPrunePlan(p *prune.Plan) {
	fmt.Printf("database is %s\n\n", humanBytes(p.SizeBefore))
	fmt.Printf("  %5d article(s) older than %d days lose their body   ~%s\n", p.Bodies, p.BodyDays, humanBytes(p.BodyBytes))
	fmt.Printf("  %5d article(s) older than %d days go entirely       ~%s\n", p.Rows, p.RowDays, humanBytes(p.RowBytes))
	fmt.Printf("\nkept, whatever their age: %d starred, %d still unread, %d with a note in the vault\n",
		p.KeptStarred, p.KeptUnread, p.KeptInVault)
}

// parseDays reads "180d", "180" or "6m" — the shapes someone types when they
// mean an age.
func parseDays(v string) (int, error) {
	if v == "" {
		return 0, fmt.Errorf("empty")
	}
	unit := v[len(v)-1]
	number := v
	multiplier := 1
	switch unit {
	case 'd':
		number = v[:len(v)-1]
	case 'w':
		number, multiplier = v[:len(v)-1], 7
	case 'm':
		number, multiplier = v[:len(v)-1], 30
	case 'y':
		number, multiplier = v[:len(v)-1], 365
	}
	n, err := strconv.Atoi(number)
	if err != nil || n <= 0 {
		return 0, fmt.Errorf("expected something like 180d, 6m or 1y")
	}
	return n * multiplier, nil
}

func humanBytes(n int64) string {
	switch {
	case n < 0:
		return "0 B"
	case n < 1024:
		return fmt.Sprintf("%d B", n)
	case n < 1024*1024:
		return fmt.Sprintf("%.1f KB", float64(n)/1024)
	case n < 1024*1024*1024:
		return fmt.Sprintf("%.1f MB", float64(n)/(1024*1024))
	}
	return fmt.Sprintf("%.2f GB", float64(n)/(1024*1024*1024))
}
