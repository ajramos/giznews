package desktop

import (
	"context"
	"testing"
)

func TestRulesCRUD(t *testing.T) {
	app := newTestApp(t)
	ctx := context.Background()

	actions := []RuleActionDTO{{Type: "category", Value: "industry"}, {Type: "importance", Value: "2"}}

	r, err := app.AddRule(ctx, "openai", "openai|gpt", actions, true)
	if err != nil {
		t.Fatal(err)
	}
	if r.ID == 0 || !r.Enabled || len(r.Actions) != 2 {
		t.Fatalf("rule = %+v", r)
	}

	all, err := app.ListRules(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 1 {
		t.Fatalf("rules = %+v", all)
	}

	if _, err := app.UpdateRule(ctx, r.ID, "openai2", "openai|gpt|chatgpt", []RuleActionDTO{{Type: "category", Value: "models"}}, true); err != nil {
		t.Fatal(err)
	}

	if err := app.SetRuleEnabled(ctx, r.ID, false); err != nil {
		t.Fatal(err)
	}
	// disabled rules move after enabled in List ordering; only one rule here.
	got, _ := app.ListRules(ctx)
	if got[0].Enabled {
		t.Fatalf("expected disabled, got %+v", got[0])
	}

	if err := app.DeleteRule(ctx, r.ID); err != nil {
		t.Fatal(err)
	}
	all, _ = app.ListRules(ctx)
	if len(all) != 0 {
		t.Fatalf("after delete = %+v", all)
	}
}

func TestAddRuleInvalidRegex(t *testing.T) {
	app := newTestApp(t)
	ctx := context.Background()
	if _, err := app.AddRule(ctx, "bad", "(unclosed", nil, true); err == nil {
		t.Fatal("expected error for invalid regex")
	}
}
