package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"

	"github.com/ajramos/giznews/internal/classify"
	"github.com/ajramos/giznews/internal/db"
)

// ruleFile is the on-disk format of a ruleset. Rules deserve to live in a file
// under version control: they are the cheapest part of the pipeline to get
// wrong, and a regex nobody can diff is a regex nobody reviews.
type ruleFile struct {
	Version int        `json:"version"`
	Rules   []fileRule `json:"rules"`
}

// fileRule is one rule as written in the file. Note is for whoever reads it —
// the database has nowhere to put it, so it stays in the file.
type fileRule struct {
	Name    string          `json:"name"`
	Note    string          `json:"note,omitempty"`
	Query   string          `json:"query"`
	Actions []db.RuleAction `json:"actions"`
	Enabled *bool           `json:"enabled,omitempty"` // absent = enabled
}

// runRules manages the deterministic prefilter: list, test, import, export.
func runRules(args []string, logger *log.Logger) {
	if len(args) == 0 {
		logger.Fatal("usage: giznews rules <list|test|import|export|add|rm|enable|disable> [args]")
	}
	cfg, d, ctx := loadAndOpenDB(args[1:], logger)
	defer d.Close()
	repo := db.NewRuleRepo(d)

	switch args[0] {
	case "list":
		rules, err := repo.List(ctx)
		if err != nil {
			logger.Fatalf("rules list: %v", err)
		}
		if len(rules) == 0 {
			fmt.Println("no rules yet — `giznews rules import docs/rules/noise.json` for a starter set")
			return
		}
		for _, r := range rules {
			state := "on "
			if !r.Enabled {
				state = "off"
			}
			fmt.Printf("%3d  %s  %-9s  %-34s  %s\n", r.ID, state, ruleEffect(r), truncate(r.Name, 34), truncate(r.Query, 52))
		}
		fmt.Printf("\n%d rule(s). First match wins, in this order.\n", len(rules))

	case "test":
		fs := flag.NewFlagSet("test", flag.ContinueOnError)
		named := fs.String("rule", "", "test a stored rule by name instead of a raw regex")
		limit := fs.Int("limit", 20, "how many matching articles to print")
		mustParse(fs, args[1:], logger)

		query := ""
		switch {
		case *named != "":
			rule, err := ruleByRef(ctx, repo, *named)
			if err != nil {
				logger.Fatalf("rules test: %v", err)
			}
			query = rule.Query
			fmt.Printf("rule %q: %s\n\n", rule.Name, rule.Query)
		default:
			query = fs.Arg(0)
			if query == "" {
				logger.Fatal(`usage: giznews rules test "<regex>" | --rule <name>`)
			}
		}

		svc := classify.NewService(d, classify.Options{}, nil, logger)
		matched, total, err := svc.TestQuery(ctx, query, *limit)
		if err != nil {
			logger.Fatalf("rules test: %v", err)
		}
		for _, a := range matched {
			category := a.Category
			if category == "" {
				category = "-"
			}
			fmt.Printf("  ★%d %-9s %-8s %s\n", a.Importance, truncate(category, 9), a.Status, truncate(a.Title, 62))
		}
		if total == 0 {
			fmt.Println("nothing in the database matches — the rule would sit idle")
			return
		}
		fmt.Printf("\n%d article(s) match", total)
		if total > len(matched) {
			fmt.Printf(" (%d shown)", len(matched))
		}
		fmt.Println(" — check every one of them before letting a rule archive it")

	case "import":
		// --dry-run may sit anywhere, so it is picked out by hand rather than by
		// a flag set: Go's flag parsing stops at the first positional argument,
		// so `rules import file.json --dry-run` would otherwise import for real.
		dryRun := hasFlag(args[1:], "dry-run")
		path := firstNonFlag(args[1:])
		if path == "" {
			logger.Fatal("usage: giznews rules import <file.json> [--dry-run]")
		}
		created, updated, unchanged, err := importRules(ctx, repo, path, dryRun)
		if err != nil {
			logger.Fatalf("rules import: %v", err)
		}
		verb := "imported"
		if dryRun {
			verb = "would import"
		}
		fmt.Printf("%s: %d new, %d updated, %d unchanged\n", verb, created, updated, unchanged)
		if !dryRun {
			fmt.Println("run `giznews classify --dry-run` before the next classify to see what they claim")
		}

	case "export":
		rules, err := repo.List(ctx)
		if err != nil {
			logger.Fatalf("rules export: %v", err)
		}
		out := ruleFile{Version: 1}
		for _, r := range rules {
			enabled := r.Enabled
			out.Rules = append(out.Rules, fileRule{
				Name: r.Name, Query: r.Query, Actions: r.Actions, Enabled: &enabled,
			})
		}
		blob, err := json.MarshalIndent(out, "", "  ")
		if err != nil {
			logger.Fatalf("rules export: %v", err)
		}
		if path := firstNonFlag(args[1:]); path != "" {
			if err := os.WriteFile(path, append(blob, '\n'), 0o644); err != nil {
				logger.Fatalf("rules export: %v", err)
			}
			fmt.Printf("%d rule(s) written to %s\n", len(rules), path)
			return
		}
		fmt.Println(string(blob))

	case "add":
		fs := flag.NewFlagSet("add", flag.ContinueOnError)
		name := fs.String("name", "", "rule name")
		query := fs.String("query", "", "regex, matched against title + author + URL")
		category := fs.String("category", "", "category to apply")
		importance := fs.Int("importance", -1, "importance to apply (0-3)")
		tags := fs.String("tag", "", "comma-separated tags to apply")
		archive := fs.Bool("archive", false, "archive what matches")
		keep := fs.Bool("keep", false, "protect what matches from later rules")
		boost := fs.Int("boost", 0, "importance floor applied after the model (1-3)")
		disabled := fs.Bool("disabled", false, "create it switched off")
		mustParse(fs, args[1:], logger)
		if *name == "" || *query == "" {
			logger.Fatal(`usage: giznews rules add --name X --query "<regex>" [--archive|--keep|--category C|--importance N|--tag a,b]`)
		}
		var actions []db.RuleAction
		if *category != "" {
			actions = append(actions, db.RuleAction{Type: "category", Value: *category})
		}
		if *importance >= 0 {
			actions = append(actions, db.RuleAction{Type: "importance", Value: strconv.Itoa(*importance)})
		}
		for _, t := range strings.Split(*tags, ",") {
			if t = strings.TrimSpace(t); t != "" {
				actions = append(actions, db.RuleAction{Type: "tag", Value: t})
			}
		}
		if *archive {
			actions = append(actions, db.RuleAction{Type: "archive"})
		}
		if *keep {
			actions = append(actions, db.RuleAction{Type: "keep"})
		}
		if *boost > 0 {
			actions = append(actions, db.RuleAction{Type: "boost", Value: strconv.Itoa(*boost)})
		}
		nr := db.NewRule{Name: *name, Query: *query, Actions: actions, Enabled: !*disabled}
		if err := validateRule(nr); err != nil {
			logger.Fatalf("rules add: %v", err)
		}
		r, err := repo.Create(ctx, nr)
		if err != nil {
			logger.Fatalf("rules add: %v", err)
		}
		fmt.Printf("rule %d %q added (%s)\n", r.ID, r.Name, ruleEffect(r))

	case "rm":
		rule := mustRule(ctx, repo, args, logger, "rm")
		if err := repo.Delete(ctx, rule.ID); err != nil {
			logger.Fatalf("rules rm: %v", err)
		}
		fmt.Printf("rule %d %q removed\n", rule.ID, rule.Name)

	case "enable", "disable":
		rule := mustRule(ctx, repo, args, logger, args[0])
		on := args[0] == "enable"
		if err := repo.SetEnabled(ctx, rule.ID, on); err != nil {
			logger.Fatalf("rules %s: %v", args[0], err)
		}
		fmt.Printf("rule %d %q is now %s\n", rule.ID, rule.Name, map[bool]string{true: "on", false: "off"}[on])

	default:
		logger.Fatalf("unknown rules subcommand %q (expected list|test|import|export|add|rm|enable|disable)", args[0])
	}
	_ = cfg
}

// importRules applies a ruleset file, matching stored rules by name so the file
// stays the source of truth and importing it twice changes nothing.
func importRules(ctx context.Context, repo *db.RuleRepo, path string, dryRun bool) (created, updated, unchanged int, err error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return 0, 0, 0, err
	}
	var file ruleFile
	if err := json.Unmarshal(raw, &file); err != nil {
		// A bare array of rules is accepted too: it is what `jq` hands back.
		if err2 := json.Unmarshal(raw, &file.Rules); err2 != nil {
			return 0, 0, 0, fmt.Errorf("parse %s: %w", path, err)
		}
	}
	if len(file.Rules) == 0 {
		return 0, 0, 0, fmt.Errorf("%s holds no rules", path)
	}

	existing, err := repo.List(ctx)
	if err != nil {
		return 0, 0, 0, err
	}
	byName := make(map[string]*db.Rule, len(existing))
	for _, r := range existing {
		byName[r.Name] = r
	}

	for _, fr := range file.Rules {
		enabled := fr.Enabled == nil || *fr.Enabled
		nr := db.NewRule{Name: fr.Name, Query: fr.Query, Actions: fr.Actions, Enabled: enabled}
		if err := validateRule(nr); err != nil {
			return created, updated, unchanged, err
		}
		old, ok := byName[fr.Name]
		if !ok {
			if !dryRun {
				if _, err := repo.Create(ctx, nr); err != nil {
					return created, updated, unchanged, err
				}
			}
			created++
			continue
		}
		if sameRule(old, nr) {
			unchanged++
			continue
		}
		if !dryRun {
			if err := repo.Update(ctx, old.ID, nr); err != nil {
				return created, updated, unchanged, err
			}
		}
		updated++
	}
	return created, updated, unchanged, nil
}

// validateRule refuses a rule the classifier could not run: a regex that does
// not compile is skipped silently at match time, which looks exactly like a
// rule that never fires.
func validateRule(nr db.NewRule) error {
	if strings.TrimSpace(nr.Name) == "" || strings.TrimSpace(nr.Query) == "" {
		return fmt.Errorf("rule %q: name and query are both required", nr.Name)
	}
	if _, err := classify.CompileRule(&db.Rule{Name: nr.Name, Query: nr.Query, Actions: nr.Actions}); err != nil {
		return err
	}
	return nil
}

func sameRule(old *db.Rule, nr db.NewRule) bool {
	if old.Query != nr.Query || old.Enabled != nr.Enabled || len(old.Actions) != len(nr.Actions) {
		return false
	}
	for i, a := range old.Actions {
		if a.Type != nr.Actions[i].Type || a.Value != nr.Actions[i].Value {
			return false
		}
	}
	return true
}

// ruleEffect names what a rule does with what it matches.
func ruleEffect(r *db.Rule) string {
	actions, err := classify.ParseRule(r)
	if err != nil {
		return "broken"
	}
	switch {
	case actions.Boost > 0:
		return fmt.Sprintf("boost ★%d", actions.Boost)
	case actions.Keep:
		return "keep"
	case actions.Archive:
		return "archive"
	default:
		return "classify"
	}
}

// ruleByRef finds a rule by id or by name, so neither has to be looked up first.
func ruleByRef(ctx context.Context, repo *db.RuleRepo, ref string) (*db.Rule, error) {
	if id, err := strconv.ParseInt(ref, 10, 64); err == nil {
		return repo.Get(ctx, id)
	}
	rules, err := repo.List(ctx)
	if err != nil {
		return nil, err
	}
	for _, r := range rules {
		if strings.EqualFold(r.Name, ref) {
			return r, nil
		}
	}
	return nil, fmt.Errorf("no rule called %q", ref)
}

func mustRule(ctx context.Context, repo *db.RuleRepo, args []string, logger *log.Logger, verb string) *db.Rule {
	ref := firstNonFlag(args[1:])
	if ref == "" {
		logger.Fatalf("usage: giznews rules %s <id|name>", verb)
	}
	rule, err := ruleByRef(ctx, repo, ref)
	if err != nil {
		if errors.Is(err, db.ErrNotFound) {
			logger.Fatalf("rules %s: no rule %q", verb, ref)
		}
		logger.Fatalf("rules %s: %v", verb, err)
	}
	return rule
}

// mustParse parses a subcommand's flags, with --config taken out first: it is
// a global that may sit anywhere, and a flag set that does not declare it would
// reject the whole line.
func mustParse(fs *flag.FlagSet, args []string, logger *log.Logger) {
	fs.SetOutput(os.Stderr)
	if err := fs.Parse(stripConfig(args)); err != nil {
		logger.Fatalf("rules %s: %v", fs.Name(), err)
	}
}

// stripConfig removes the global --config flag and its value from a list of
// subcommand arguments.
func stripConfig(args []string) []string {
	out := make([]string, 0, len(args))
	for i := 0; i < len(args); i++ {
		a := args[i]
		if a == "-config" || a == "--config" {
			i++ // and its value
			continue
		}
		if strings.HasPrefix(a, "--config=") || strings.HasPrefix(a, "-config=") {
			continue
		}
		out = append(out, a)
	}
	return out
}

// firstNonFlag returns the first bare argument, for subcommands that take one
// reference and no flags of their own.
func firstNonFlag(args []string) string {
	for _, a := range stripConfig(args) {
		if !strings.HasPrefix(a, "-") {
			return a
		}
	}
	return ""
}
