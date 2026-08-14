package classify

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/ajramos/giznews/internal/db"
)

// RuleActions holds the parsed actions of a rule.
type RuleActions struct {
	Category   string
	Tags       []string
	Importance int // 0..3; 0 when unset
	Archive    bool
}

// compiledRule is a rule with its matcher compiled. It embeds the parsed
// actions so fields are reachable directly.
type compiledRule struct {
	*RuleActions
	re *regexp.Regexp
}

// CompiledRule is the exported compiled rule with a Match method.
type CompiledRule = compiledRule

// ParseRule validates a db.Rule and produces its actions.
func ParseRule(r *db.Rule) (*RuleActions, error) {
	if _, err := regexp.Compile("(?i)" + r.Query); err != nil {
		return nil, fmt.Errorf("rule %q: invalid query: %w", r.Name, err)
	}
	ra := &RuleActions{}
	for _, act := range r.Actions {
		switch act.Type {
		case "category":
			ra.Category = act.Value
		case "tag":
			if strings.TrimSpace(act.Value) != "" {
				ra.Tags = append(ra.Tags, strings.TrimSpace(act.Value))
			}
		case "importance":
			if n, err := strconv.Atoi(act.Value); err == nil && n >= 0 && n <= 3 {
				ra.Importance = n
			}
		case "archive":
			ra.Archive = true
		}
	}
	return ra, nil
}

// CompileRule parses a rule and compiles its matcher.
func CompileRule(r *db.Rule) (*CompiledRule, error) {
	if _, err := regexp.Compile("(?i)" + r.Query); err != nil {
		return nil, fmt.Errorf("rule %q: invalid query: %w", r.Name, err)
	}
	ra, err := ParseRule(r)
	if err != nil {
		return nil, err
	}
	re, _ := regexp.Compile("(?i)" + r.Query)
	return &compiledRule{RuleActions: ra, re: re}, nil
}

// CompileAll precompiles every enabled rule; broken rules are skipped.
func CompileAll(ctx context.Context, repo *db.RuleRepo) ([]*compiledRule, error) {
	rules, err := repo.ListEnabled(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]*compiledRule, 0, len(rules))
	for _, r := range rules {
		cr, err := CompileRule(r)
		if err != nil {
			continue
		}
		out = append(out, cr)
	}
	return out, nil
}

// MatchFirst returns the actions of the first rule that fires for a, or nil.
func MatchFirst(rules []*compiledRule, a *db.Article) *RuleActions {
	for _, r := range rules {
		if r.Match(a) {
			return r.RuleActions
		}
	}
	return nil
}

func (r *compiledRule) Match(a *db.Article) bool {
	if r.re == nil {
		return false
	}
	hay := a.Title + "\n" + a.Author + "\n" + a.URL
	return r.re.MatchString(hay)
}

// defaultImportance gives a deterministic importance for articles that only
// matched rules and the rule did not set one.
func defaultImportance(a *db.Article) int {
	t := strings.ToLower(a.Title)
	for _, kw := range []string{"openai", "anthropic", "gpt", "gemini", "deepmind", "meta ai", "claude"} {
		if strings.Contains(t, kw) {
			return 2
		}
	}
	return 1
}
