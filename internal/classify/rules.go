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
	// Keep protects an article: the rule fires, nothing is applied, and the
	// article goes to the model anyway. Rules are first-match-wins, so a broad
	// noise rule ("anything about crypto") would otherwise archive the one
	// article about crypto that mattered. A keep rule placed before it says
	// what the noise rules are not allowed to touch.
	Keep bool
	// Boost is an importance floor applied *after* the model has classified the
	// article, not instead of it. Marking something important with an ordinary
	// importance action would resolve it and skip the model, which is exactly
	// backwards: the articles worth three stars are the ones most worth having
	// a summary and entities for. A boost says "whatever the model decides,
	// this is at least an N" and leaves the rest of the pipeline alone.
	Boost int
}

// compiledRule is a rule with its matcher compiled. It embeds the parsed
// actions so fields are reachable directly.
type compiledRule struct {
	*RuleActions
	Name  string
	Query string
	re    *regexp.Regexp
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
		case "keep":
			ra.Keep = true
		case "boost":
			if n, err := strconv.Atoi(act.Value); err == nil && n > 0 && n <= 3 {
				ra.Boost = n
			}
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
	return &compiledRule{RuleActions: ra, Name: r.Name, Query: r.Query, re: re}, nil
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

// Decision is what the whole rule chain decided about one article.
//
// Rules are first-match-wins, but a boost is not a claim on an article — it is
// something said about it. So the chain is read twice: the highest boost any
// rule gives it, and separately the first rule that actually wants to do
// something. An article somebody boosted is never archived: an explicit "this
// matters" outranks a pattern that says "this usually does not".
type Decision struct {
	Floor     int    // highest importance floor a boost rule gives it
	BoostedBy string // the rule that set that floor
	Action    *RuleActions
	ActionBy  string
}

// ToLLM reports whether the article still has to be classified by the model.
func (d Decision) ToLLM() bool {
	return d.Floor > 0 || d.Action == nil || d.Action.Keep
}

// Decide runs the whole chain over one article.
func Decide(rules []*compiledRule, a *db.Article) Decision {
	var d Decision
	for _, r := range rules {
		if !r.Match(a) {
			continue
		}
		if r.Boost > 0 {
			if r.Boost > d.Floor {
				d.Floor, d.BoostedBy = r.Boost, r.Name
			}
			continue // a boost annotates; it does not claim the article
		}
		if d.Action == nil {
			d.Action, d.ActionBy = r.RuleActions, r.Name
		}
	}
	return d
}

// MatchFirst returns the actions of the first rule that fires for a, or nil.
func MatchFirst(rules []*compiledRule, a *db.Article) *RuleActions {
	if r := MatchFirstRule(rules, a); r != nil {
		return r.RuleActions
	}
	return nil
}

// MatchFirstRule returns the first rule that fires for a, or nil. Callers that
// only need the outcome use MatchFirst; a preview needs to say which rule it
// was.
func MatchFirstRule(rules []*compiledRule, a *db.Article) *CompiledRule {
	for _, r := range rules {
		if r.Match(a) {
			return r
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
