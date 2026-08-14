package db

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
)

// RuleAction is one deterministic action applied when a rule matches.
type RuleAction struct {
	// Type: "category" | "tag" | "importance" | "archive".
	Type string `json:"type"`
	// Value is the target for category/importance (string or int), a tag for
	// "tag", or empty for "archive".
	Value string `json:"value"`
}

// Rule is a deterministic classification rule (the ⚡ prefilter).
type Rule struct {
	ID        int64        `json:"id"`
	Name      string       `json:"name"`
	Query     string       `json:"query"`   // regex matched against title+author+url
	Actions   []RuleAction `json:"actions"` // applied in order on match
	Enabled   bool         `json:"enabled"`
	CreatedAt string       `json:"created_at"`
	UpdatedAt string       `json:"updated_at"`
}

// NewRule is the input when creating a rule.
type NewRule struct {
	Name    string
	Query   string
	Actions []RuleAction
	Enabled bool
}

// RuleRepo provides CRUD over the rules table.
type RuleRepo struct {
	db *DB
}

// NewRuleRepo creates a rules repository.
func NewRuleRepo(db *DB) *RuleRepo {
	return &RuleRepo{db: db}
}

// Create inserts a new rule.
func (r *RuleRepo) Create(ctx context.Context, nr NewRule) (*Rule, error) {
	now := Now()
	actions, _ := json.Marshal(nr.Actions)
	res, err := r.db.sql.ExecContext(ctx, `
		INSERT INTO rules (name, query, actions, enabled, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)`,
		nr.Name, nr.Query, string(actions), boolToInt(nr.Enabled), now, now)
	if err != nil {
		return nil, fmt.Errorf("insert rule: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("rule last insert id: %w", err)
	}
	return r.Get(ctx, id)
}

// Get returns a rule by id.
func (r *RuleRepo) Get(ctx context.Context, id int64) (*Rule, error) {
	row := r.db.sql.QueryRowContext(ctx,
		"SELECT id, name, query, actions, enabled, created_at, updated_at FROM rules WHERE id = ?", id)
	return scanRule(row)
}

// List returns all rules, enabled first.
func (r *RuleRepo) List(ctx context.Context) ([]*Rule, error) {
	rows, err := r.db.sql.QueryContext(ctx, `
		SELECT id, name, query, actions, enabled, created_at, updated_at
		FROM rules ORDER BY enabled DESC, id`)
	if err != nil {
		return nil, fmt.Errorf("list rules: %w", err)
	}
	defer rows.Close()
	var out []*Rule
	for rows.Next() {
		rule, err := scanRule(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, rule)
	}
	return out, rows.Err()
}

// ListEnabled returns enabled rules only.
func (r *RuleRepo) ListEnabled(ctx context.Context) ([]*Rule, error) {
	all, err := r.List(ctx)
	if err != nil {
		return nil, err
	}
	var out []*Rule
	for _, rule := range all {
		if rule.Enabled {
			out = append(out, rule)
		}
	}
	return out, nil
}

// SetEnabled toggles a rule.
func (r *RuleRepo) SetEnabled(ctx context.Context, id int64, enabled bool) error {
	res, err := r.db.sql.ExecContext(ctx,
		"UPDATE rules SET enabled = ?, updated_at = ? WHERE id = ?", boolToInt(enabled), Now(), id)
	if err != nil {
		return fmt.Errorf("set rule enabled: %w", err)
	}
	return checkAffected(res, "set rule enabled")
}

// Delete removes a rule.
func (r *RuleRepo) Delete(ctx context.Context, id int64) error {
	res, err := r.db.sql.ExecContext(ctx, "DELETE FROM rules WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("delete rule: %w", err)
	}
	return checkAffected(res, "delete rule")
}

func scanRule(row scanner) (*Rule, error) {
	var (
		rule       Rule
		actionsRaw string
		enabled    int
	)
	if err := row.Scan(&rule.ID, &rule.Name, &rule.Query, &actionsRaw, &enabled, &rule.CreatedAt, &rule.UpdatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("scan rule: %w", err)
	}
	rule.Enabled = enabled != 0
	_ = json.Unmarshal([]byte(actionsRaw), &rule.Actions)
	if rule.Actions == nil {
		rule.Actions = []RuleAction{}
	}
	return &rule, nil
}
