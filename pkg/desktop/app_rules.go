package desktop

import (
	"context"
	"fmt"
	"regexp"

	"github.com/ajramos/giznews/internal/db"
)

// RuleActionDTO is one deterministic action applied when a rule matches.
type RuleActionDTO struct {
	Type  string `json:"type"`  // category | tag | importance | archive
	Value string `json:"value"` // category/importance value, tag name, or "" for archive
}

// RuleDTO is a deterministic classification rule (the ⚡ prefilter).
type RuleDTO struct {
	ID      int64           `json:"id"`
	Name    string          `json:"name"`
	Query   string          `json:"query"` // regex matched against title+author+url
	Actions []RuleActionDTO `json:"actions"`
	Enabled bool            `json:"enabled"`
}

// ListRules returns all rules, enabled first.
func (a *App) ListRules(ctx context.Context) ([]*RuleDTO, error) {
	rules, err := db.NewRuleRepo(a.db).List(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]*RuleDTO, 0, len(rules))
	for _, r := range rules {
		out = append(out, toRuleDTO(r))
	}
	return out, nil
}

// AddRule creates a new rule.
func (a *App) AddRule(ctx context.Context, name, query string, actions []RuleActionDTO, enabled bool) (*RuleDTO, error) {
	if err := validateRule(name, query); err != nil {
		return nil, err
	}
	r, err := db.NewRuleRepo(a.db).Create(ctx, db.NewRule{
		Name: name, Query: query, Actions: toDBActions(actions), Enabled: enabled,
	})
	if err != nil {
		return nil, err
	}
	return toRuleDTO(r), nil
}

// UpdateRule persists a rule's mutable fields.
func (a *App) UpdateRule(ctx context.Context, id int64, name, query string, actions []RuleActionDTO, enabled bool) (*RuleDTO, error) {
	if err := validateRule(name, query); err != nil {
		return nil, err
	}
	repo := db.NewRuleRepo(a.db)
	if err := repo.Update(ctx, id, db.NewRule{
		Name: name, Query: query, Actions: toDBActions(actions), Enabled: enabled,
	}); err != nil {
		return nil, err
	}
	r, err := repo.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	return toRuleDTO(r), nil
}

// SetRuleEnabled toggles a rule.
func (a *App) SetRuleEnabled(ctx context.Context, id int64, enabled bool) error {
	return db.NewRuleRepo(a.db).SetEnabled(ctx, id, enabled)
}

// DeleteRule removes a rule.
func (a *App) DeleteRule(ctx context.Context, id int64) error {
	return db.NewRuleRepo(a.db).Delete(ctx, id)
}

func validateRule(name, query string) error {
	if name == "" {
		return fmt.Errorf("rule name is required")
	}
	if query == "" {
		return fmt.Errorf("rule query (regex) is required")
	}
	if _, err := regexp.Compile("(?i)" + query); err != nil {
		return fmt.Errorf("invalid regex query: %w", err)
	}
	return nil
}

func toRuleDTO(r *db.Rule) *RuleDTO {
	d := &RuleDTO{ID: r.ID, Name: r.Name, Query: r.Query, Enabled: r.Enabled}
	for _, act := range r.Actions {
		d.Actions = append(d.Actions, RuleActionDTO{Type: act.Type, Value: act.Value})
	}
	return d
}

func toDBActions(actions []RuleActionDTO) []db.RuleAction {
	out := make([]db.RuleAction, 0, len(actions))
	for _, act := range actions {
		out = append(out, db.RuleAction{Type: act.Type, Value: act.Value})
	}
	return out
}
