package classify

import (
	"context"
	"fmt"

	"github.com/ajramos/giznews/internal/db"
)

// Plan is what a classification run would do, worked out without doing it.
//
// The prefilter is cheap to write and expensive to get wrong: a regex that
// archives more than it should does it silently, to a queue nobody is reading.
// A plan says which rule claims which article, and how many are left for the
// model, before any of it is applied.
type Plan struct {
	Pending  int        `json:"pending"`
	ByRules  int        `json:"by_rules"`
	Archived int        `json:"archived"`
	Kept     int        `json:"kept"`
	ToLLM    int        `json:"to_llm"`
	Batches  int        `json:"batches"`
	Rules    []RulePlan `json:"rules"`
	// Unmatched is a sample of what would reach the model, so a queue full of
	// something the rules could have caught is visible.
	Unmatched []string `json:"unmatched"`
}

// RulePlan is one rule and what it would claim.
type RulePlan struct {
	Name    string   `json:"name"`
	Query   string   `json:"query"`
	Matches int      `json:"matches"`
	Effect  string   `json:"effect"` // archive | keep | classify
	Sample  []string `json:"sample"`
}

// maxSample is how many titles a plan shows per rule: enough to recognise a
// regex that went wide, short enough to read.
const maxSample = 5

// Preview reports what ClassifyAll would do with the queue as it stands. It
// writes nothing.
func (s *Service) Preview(ctx context.Context) (*Plan, error) {
	articles, err := db.NewArticleRepo(s.db).ListUnclassified(ctx, s.opts.Limit, s.opts.AgeDays)
	if err != nil {
		return nil, fmt.Errorf("list unclassified: %w", err)
	}
	rules, err := CompileAll(ctx, db.NewRuleRepo(s.db))
	if err != nil {
		return nil, fmt.Errorf("compile rules: %w", err)
	}

	plan := &Plan{Pending: len(articles)}
	byName := map[string]*RulePlan{}
	for _, r := range rules {
		rp := &RulePlan{Name: r.Name, Query: r.Query, Effect: effectOf(r.RuleActions)}
		byName[r.Name] = rp
		plan.Rules = append(plan.Rules, *rp)
	}

	for _, a := range articles {
		match := MatchFirstRule(rules, a)
		if match == nil {
			plan.ToLLM++
			if len(plan.Unmatched) < maxSample {
				plan.Unmatched = append(plan.Unmatched, a.Title)
			}
			continue
		}
		rp := byName[match.Name]
		if rp != nil {
			rp.Matches++
			if len(rp.Sample) < maxSample {
				rp.Sample = append(rp.Sample, a.Title)
			}
		}
		switch {
		case match.Keep:
			plan.Kept++
			plan.ToLLM++ // protected: the model still sees it
		case match.Archive:
			plan.Archived++
			plan.ByRules++
		default:
			plan.ByRules++
		}
	}

	// Rules were copied before counting so the order is the matching order.
	for i := range plan.Rules {
		if rp := byName[plan.Rules[i].Name]; rp != nil {
			plan.Rules[i] = *rp
		}
	}
	if plan.ToLLM > 0 && s.opts.BatchSize > 0 {
		plan.Batches = (plan.ToLLM + s.opts.BatchSize - 1) / s.opts.BatchSize
	}
	return plan, nil
}

// effectOf names what a rule does, for a reader scanning a list of them.
func effectOf(a *RuleActions) string {
	switch {
	case a.Keep:
		return "keep"
	case a.Archive:
		return "archive"
	default:
		return "classify"
	}
}

// TestQuery reports which stored articles a query would match, whether they
// have been classified or not. It is how a rule is written safely: see what it
// catches before it is allowed to act.
func (s *Service) TestQuery(ctx context.Context, query string, limit int) ([]*db.Article, int, error) {
	rule, err := CompileRule(&db.Rule{Name: "test", Query: query})
	if err != nil {
		return nil, 0, err
	}
	articles, err := db.NewArticleRepo(s.db).List(ctx, db.ListOptions{Limit: 5000})
	if err != nil {
		return nil, 0, err
	}
	var matched []*db.Article
	for _, a := range articles {
		if rule.Match(a) {
			matched = append(matched, a)
		}
	}
	total := len(matched)
	if limit > 0 && len(matched) > limit {
		matched = matched[:limit]
	}
	return matched, total, nil
}
