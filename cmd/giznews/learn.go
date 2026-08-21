package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/ajramos/giznews/internal/db"
	"github.com/ajramos/giznews/internal/learn"
)

// runLearn reports what the reader's history has taught the app, and — unless
// this is a dry run — stores it for the next classification.
func runLearn(args []string, logger *log.Logger) {
	cfg, d, ctx := loadAndOpenDB(args, logger)
	defer d.Close()

	opts := learn.Options{
		WindowDays: cfg.Classify.Learn.WindowDays,
		MinSamples: cfg.Classify.Learn.MinSamples,
		MaxDelta:   cfg.Classify.Learn.MaxDelta,
	}
	signals, err := learn.Compute(ctx, d, opts)
	if err != nil {
		logger.Fatalf("learn: %v", err)
	}
	if len(signals) == 0 {
		fmt.Println("nothing to learn from yet — read, star and archive a few articles first")
		return
	}

	printSignals(signals, opts)

	if hasFlag(args, "dry-run") {
		fmt.Println("\ndry run — nothing was stored")
		return
	}
	if err := learn.Store(ctx, d, signals); err != nil {
		logger.Fatalf("learn: %v", err)
	}
	moving := 0
	for _, s := range signals {
		if s.Delta != 0 {
			moving++
		}
	}
	fmt.Printf("\nstored: %d signal(s), %d of them moving importance\n", len(signals), moving)
	if !cfg.Classify.Learn.Enabled {
		fmt.Println("classify.learn.enabled is false, so they are recorded but not applied")
	}
}

// printSignals renders the table. Everything is shown, including what is not
// allowed to move anything yet — the sample size is half the argument.
func printSignals(signals []learn.Signal, opts learn.Options) {
	fmt.Printf("what your reading says, over the last %d days (a source or tag needs %d articles to count)\n\n",
		windowOr(opts.WindowDays, 90), sampleOr(opts.MinSamples, 20))
	fmt.Printf("  %-6s %-28s %5s %7s %7s %7s  %s\n", "kind", "name", "n", "read", "dropped", "starred", "verdict")
	for _, s := range signals {
		verdict := "—"
		switch {
		case s.Delta > 0:
			verdict = fmt.Sprintf("+%d  you keep these", s.Delta)
		case s.Delta < 0:
			verdict = fmt.Sprintf("%d  you throw these away", s.Delta)
		case s.Samples < sampleOr(opts.MinSamples, 20):
			verdict = "not enough yet"
		}
		fmt.Printf("  %-6s %-28s %5d %6.0f%% %6.0f%% %6.0f%%  %s\n",
			s.Kind, truncate(s.Label, 28), s.Samples,
			s.ReadRate*100, s.DropRate*100, s.StarRate*100, verdict)
	}
	fmt.Println("\n  read% is shown but never acted on: the reader opens whatever the cursor lands on.")
}

func windowOr(v, fallback int) int {
	if v > 0 {
		return v
	}
	return fallback
}

func sampleOr(v, fallback int) int {
	if v > 0 {
		return v
	}
	return fallback
}

// suggestRules writes a ruleset for the sources the reader clearly does not
// want, so a habit strong enough to be a rule goes through the same review path
// as one written by hand — never applied on its own.
func suggestRules(args []string, signals []learn.Signal, logger *log.Logger) {
	var proposed []fileRule
	for _, s := range signals {
		if s.Kind != "source" || s.Delta >= 0 || s.Match == "" {
			continue
		}
		if s.DropRate < suggestDropRate {
			continue
		}
		proposed = append(proposed, fileRule{
			Name: "learned: " + strings.ToLower(s.Label),
			Note: fmt.Sprintf("You archived %.0f%% of %d article(s) from this source without opening them.",
				s.DropRate*100, s.Samples),
			Query:   s.Match,
			Actions: []db.RuleAction{{Type: "archive"}},
			Enabled: boolPtr(false), // suggestions arrive switched off, always
		})
	}
	if len(proposed) == 0 {
		fmt.Printf("nothing worth a rule yet — no source is archived unopened at least %.0f%% of the time\n",
			suggestDropRate*100)
		return
	}
	blob, err := json.MarshalIndent(ruleFile{Version: 1, Rules: proposed}, "", "  ")
	if err != nil {
		logger.Fatalf("rules suggest: %v", err)
	}
	path := firstNonFlag(args)
	if path == "" {
		fmt.Println(string(blob))
		fmt.Printf("\n# %d suggestion(s), switched off. To use them:\n"+
			"#   giznews rules suggest suggested.json   # write them to a file\n"+
			"#   giznews rules import suggested.json    # load them, still off\n"+
			"#   giznews rules test --rule \"<name>\"     # see what one would catch\n", len(proposed))
		return
	}
	if err := os.WriteFile(path, append(blob, '\n'), 0o644); err != nil {
		logger.Fatalf("rules suggest: %v", err)
	}
	fmt.Printf("%d suggestion(s) written to %s, switched off — review them, then `giznews rules import %s`\n",
		len(proposed), path, path)
}

// suggestDropRate is how much of a source has to go unread before proposing a
// rule for it. Higher than the bar for an adjustment: a rule is permanent and
// silent, an adjustment is a nudge.
const suggestDropRate = 0.85

func boolPtr(b bool) *bool { return &b }
