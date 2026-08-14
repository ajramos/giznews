package classify

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/ajramos/giznews/internal/db"
)

func TestParseClassificationsFences(t *testing.T) {
	content := "```json\n[{\"id\":1,\"category\":\"models\",\"importance\":3,\"tags\":[\"llm\",\"scaling\"],\"entities\":[{\"name\":\"OpenAI\",\"type\":\"org\"}],\"summary\":\"Big release.\",\"headline\":\"x\"}]\n```"
	m, err := parseClassifications(content)
	if err != nil {
		t.Fatal(err)
	}
	c, ok := m[1]
	if !ok {
		t.Fatal("missing id 1")
	}
	if c.Category != "models" || c.Importance != 3 {
		t.Fatalf("c = %+v", c)
	}
	if len(c.Tags) != 2 || c.Tags[0] != "llm" {
		t.Fatalf("tags = %v", c.Tags)
	}
}

func TestParseClassificationsBadCategory(t *testing.T) {
	content := `[{"id":2,"category":"MODELEZ","importance":9,"tags":["X","x"," "],"summary":"s"}]`
	m, err := parseClassifications(content)
	if err != nil {
		t.Fatal(err)
	}
	c := m[2]
	if c.Category != "general" {
		t.Fatalf("category = %q", c.Category)
	}
	if c.Importance != 1 {
		t.Fatalf("importance = %d", c.Importance)
	}
	if len(c.Tags) != 1 {
		t.Fatalf("tags = %v (want deduped)", c.Tags)
	}
}

func TestParseClassificationsGarbage(t *testing.T) {
	if _, err := parseClassifications("sorry I cannot help"); err == nil {
		t.Fatal("expected error on garbage")
	}
}

func TestRuleMatching(t *testing.T) {
	rule := &db.Rule{
		Name:  "openai-news",
		Query: `openai`,
		Actions: []db.RuleAction{
			{Type: "category", Value: "industry"},
			{Type: "importance", Value: "2"},
			{Type: "tag", Value: "OpenAI"},
		},
		Enabled: true,
	}
	cr, err := CompileRule(rule)
	if err != nil {
		t.Fatal(err)
	}

	art := &db.Article{ID: 1, Title: "OpenAI announces new model", Author: "a", URL: "https://x.com"}
	if !cr.Match(art) {
		t.Fatal("expected match")
	}
	if cr.Category != "industry" || cr.Importance != 2 {
		t.Fatalf("actions = %+v", cr.RuleActions)
	}

	other := &db.Article{ID: 2, Title: "Banana prices", URL: "https://x.com"}
	if cr.Match(other) {
		t.Fatal("expected no match")
	}
}

func TestRuleBadQuery(t *testing.T) {
	rule := &db.Rule{Name: "bad", Query: "([unclosed"}
	if _, err := ParseRule(rule); err == nil {
		t.Fatal("expected error for invalid regex")
	}
}

func TestRulesFirstThenLLM(t *testing.T) {
	d, err := db.Open(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	ctx := context.Background()

	srcRepo := db.NewSourceRepo(d)
	src, _ := srcRepo.Create(ctx, db.NewSource{Name: "S", Type: db.SourceRSS, URL: "u"})

	artRepo := db.NewArticleRepo(d)
	// One article matching a rule, one not.
	_, _, _ = artRepo.Upsert(ctx, db.NewArticle{SourceID: src.ID, GUID: "a", Title: "OpenAI releases new model", URL: "https://x.com/1"})
	_, _, _ = artRepo.Upsert(ctx, db.NewArticle{SourceID: src.ID, GUID: "b", Title: "Something about bananas", URL: "https://x.com/2"})

	ruleRepo := db.NewRuleRepo(d)
	_, _ = ruleRepo.Create(ctx, db.NewRule{
		Name: "openai", Query: `openai`,
		Actions: []db.RuleAction{{Type: "category", Value: "industry"}},
		Enabled: true,
	})

	svc := NewService(d, Options{UseLLM: false, BatchSize: 10, AgeDays: 30}, nil, nil)
	res, err := svc.ClassifyAll(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if res.ByRules != 1 || res.SkippedNoLLM != 1 || res.Classified != 2 {
		t.Fatalf("res = %+v", res)
	}

	art, _ := artRepo.Get(ctx, 1)
	if art.Category != "industry" {
		t.Fatalf("rule category = %q", art.Category)
	}
	art2, _ := artRepo.Get(ctx, 2)
	if art2.Category != "general" || art2.Importance == 0 {
		t.Fatalf("default art = %+v", art2)
	}

	// Re-run: nothing pending left.
	res2, _ := svc.ClassifyAll(ctx)
	if res2.Classified != 0 {
		t.Fatalf("res2 = %+v", res2)
	}
}

func TestDefaultImportance(t *testing.T) {
	if defaultImportance(&db.Article{Title: "OpenAI GPT-5 released"}) != 2 {
		t.Fatal("expected 2 for openai")
	}
	if defaultImportance(&db.Article{Title: "a random story"}) != 1 {
		t.Fatal("expected 1 for random")
	}
}
