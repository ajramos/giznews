package classify

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"

	"github.com/ajramos/giznews/internal/db"
	"github.com/ajramos/giznews/internal/learn"
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

// A keep rule protects an article from the rules below it: nothing is applied
// and the model still sees it. Without this, one broad noise rule silently
// swallows the article it was never meant to catch.
func TestKeepRuleProtectsFromLaterRules(t *testing.T) {
	d, err := db.Open(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	ctx := context.Background()

	src, _ := db.NewSourceRepo(d).Create(ctx, db.NewSource{Name: "S", Type: db.SourceRSS, URL: "u"})
	artRepo := db.NewArticleRepo(d)
	_, _, _ = artRepo.Upsert(ctx, db.NewArticle{SourceID: src.ID, GUID: "a",
		Title: "OpenAI weighs in on crypto payments", URL: "https://x.com/1"})
	_, _, _ = artRepo.Upsert(ctx, db.NewArticle{SourceID: src.ID, GUID: "b",
		Title: "Bitcoin rips past $120k", URL: "https://x.com/2"})

	ruleRepo := db.NewRuleRepo(d)
	// Order is the matching order: the shield goes first.
	if _, err := ruleRepo.Create(ctx, db.NewRule{
		Name: "keep: labs", Query: `\b(openai|anthropic)\b`,
		Actions: []db.RuleAction{{Type: "keep"}}, Enabled: true,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := ruleRepo.Create(ctx, db.NewRule{
		Name: "noise: crypto", Query: `\b(bitcoin|crypto)\b`,
		Actions: []db.RuleAction{{Type: "archive"}}, Enabled: true,
	}); err != nil {
		t.Fatal(err)
	}

	svc := NewService(d, Options{UseLLM: false, BatchSize: 10, AgeDays: 30}, nil, nil)

	plan, err := svc.Preview(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if plan.Pending != 2 || plan.Kept != 1 || plan.Archived != 1 || plan.ToLLM != 1 {
		t.Fatalf("plan = %+v", plan)
	}
	if len(plan.Rules) != 2 || plan.Rules[0].Effect != "keep" || plan.Rules[0].Matches != 1 {
		t.Fatalf("plan rules = %+v", plan.Rules)
	}

	res, err := svc.ClassifyAll(ctx)
	if err != nil {
		t.Fatal(err)
	}
	// The protected one was not resolved by rules: with no LLM it falls through
	// to the deterministic default instead.
	if res.ByRules != 1 || res.SkippedNoLLM != 1 {
		t.Fatalf("res = %+v", res)
	}
	protected, _ := artRepo.Get(ctx, 1)
	if protected.Status == db.StatusArchived {
		t.Fatalf("the keep rule did not protect it: %+v", protected)
	}
	noise, _ := artRepo.Get(ctx, 2)
	if noise.Status != db.StatusArchived {
		t.Fatalf("the noise rule did not archive it: %+v", noise)
	}
}

// A preview must describe the run that follows it: same rules, same articles,
// same outcome.
func TestPreviewLeavesEverythingAlone(t *testing.T) {
	d, err := db.Open(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	ctx := context.Background()

	src, _ := db.NewSourceRepo(d).Create(ctx, db.NewSource{Name: "S", Type: db.SourceRSS, URL: "u"})
	artRepo := db.NewArticleRepo(d)
	_, _, _ = artRepo.Upsert(ctx, db.NewArticle{SourceID: src.ID, GUID: "a", Title: "Podcast: agents", URL: "https://x.com/1"})

	ruleRepo := db.NewRuleRepo(d)
	_, _ = ruleRepo.Create(ctx, db.NewRule{
		Name: "noise: podcast", Query: `^podcast`,
		Actions: []db.RuleAction{{Type: "archive"}}, Enabled: true,
	})

	svc := NewService(d, Options{UseLLM: false, BatchSize: 10, AgeDays: 30}, nil, nil)
	if _, err := svc.Preview(ctx); err != nil {
		t.Fatal(err)
	}
	before, _ := artRepo.Get(ctx, 1)
	if before.Status == db.StatusArchived {
		t.Fatalf("the preview archived it: %+v", before)
	}
	if pending, _ := artRepo.CountUnclassified(ctx); pending != 1 {
		t.Fatalf("the preview classified something: %d still pending, want 1", pending)
	}
	if _, err := svc.ClassifyAll(ctx); err != nil {
		t.Fatal(err)
	}
	after, _ := artRepo.Get(ctx, 1)
	if after.Status != db.StatusArchived {
		t.Fatalf("the run did not do what the preview said: %+v", after)
	}
}

// TestQuery is what a rule is written with, so it has to match exactly what the
// classifier would match — title, author and URL, case-insensitively.
func TestTestQueryMatchesTheSameHaystack(t *testing.T) {
	d, err := db.Open(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	ctx := context.Background()

	src, _ := db.NewSourceRepo(d).Create(ctx, db.NewSource{Name: "S", Type: db.SourceRSS, URL: "u"})
	artRepo := db.NewArticleRepo(d)
	_, _, _ = artRepo.Upsert(ctx, db.NewArticle{SourceID: src.ID, GUID: "a", Title: "A quiet title", URL: "https://acme.fm/podcast/ep-1"})
	_, _, _ = artRepo.Upsert(ctx, db.NewArticle{SourceID: src.ID, GUID: "b", Title: "Another one", URL: "https://x.com/2"})

	svc := NewService(d, Options{}, nil, nil)
	matched, total, err := svc.TestQuery(ctx, `/podcast/`, 10)
	if err != nil {
		t.Fatal(err)
	}
	if total != 1 || len(matched) != 1 || matched[0].GUID != "a" {
		t.Fatalf("matched %d (%+v)", total, matched)
	}
	if _, _, err := svc.TestQuery(ctx, "([unclosed", 10); err == nil {
		t.Fatal("expected a broken regex to be reported, not silently ignored")
	}
}

// A boost is a floor applied after the model, not instead of it: the article
// still gets its summary and entities, and an importance the model already
// judged higher is never pulled back down.
func TestBoostIsAFloorAppliedAfterClassification(t *testing.T) {
	d, err := db.Open(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	ctx := context.Background()

	src, _ := db.NewSourceRepo(d).Create(ctx, db.NewSource{Name: "S", Type: db.SourceRSS, URL: "u"})
	artRepo := db.NewArticleRepo(d)
	_, _, _ = artRepo.Upsert(ctx, db.NewArticle{SourceID: src.ID, GUID: "a", Title: "Acme releases a model", URL: "https://x.com/1"})
	_, _, _ = artRepo.Upsert(ctx, db.NewArticle{SourceID: src.ID, GUID: "b", Title: "A quiet afternoon", URL: "https://x.com/2"})

	ruleRepo := db.NewRuleRepo(d)
	if _, err := ruleRepo.Create(ctx, db.NewRule{
		Name: "high: releases", Query: `\breleases\b`,
		Actions: []db.RuleAction{{Type: "boost", Value: "3"}}, Enabled: true,
	}); err != nil {
		t.Fatal(err)
	}

	svc := NewService(d, Options{UseLLM: false, BatchSize: 10, AgeDays: 30}, nil, nil)
	plan, err := svc.Preview(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if plan.Boosted != 1 || plan.ByRules != 0 || plan.ToLLM != 2 {
		t.Fatalf("plan = %+v", plan)
	}

	res, err := svc.ClassifyAll(ctx)
	if err != nil {
		t.Fatal(err)
	}
	// Nothing was resolved by rules: a boost does not take the article away
	// from the classifier, it only raises it afterwards.
	if res.ByRules != 0 || res.Boosted != 1 {
		t.Fatalf("res = %+v", res)
	}
	boosted, _ := artRepo.Get(ctx, 1)
	if boosted.Importance != 3 {
		t.Fatalf("importance = %d, want 3", boosted.Importance)
	}
	plain, _ := artRepo.Get(ctx, 2)
	if plain.Importance >= 3 {
		t.Fatalf("an unboosted article was raised too: %+v", plain)
	}

	// A floor below what the article already carries changes nothing: it is a
	// floor, not a value.
	if err := artRepo.SetImportance(ctx, 2, 3); err != nil {
		t.Fatal(err)
	}
	raised, _, err := svc.settleImportance(ctx, artRepo, []*db.Article{{ID: 2}}, map[int64]int{2: 1}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if raised != 0 {
		t.Fatalf("the floor touched %d article(s), want 0", raised)
	}
	after, _ := artRepo.Get(ctx, 2)
	if after.Importance != 3 {
		t.Fatalf("the floor pulled an article down to %d", after.Importance)
	}
}

// An explicit "this matters" outranks a pattern that says "this usually does
// not": a boosted article is never archived by a rule below it.
func TestBoostOutranksArchive(t *testing.T) {
	d, err := db.Open(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	ctx := context.Background()

	src, _ := db.NewSourceRepo(d).Create(ctx, db.NewSource{Name: "S", Type: db.SourceRSS, URL: "u"})
	artRepo := db.NewArticleRepo(d)
	_, _, _ = artRepo.Upsert(ctx, db.NewArticle{SourceID: src.ID, GUID: "a",
		Title: "Court fines a crypto exchange over an AI trading bot", URL: "https://x.com/1"})

	ruleRepo := db.NewRuleRepo(d)
	// The archive rule is created first, so it would win on order alone.
	_, _ = ruleRepo.Create(ctx, db.NewRule{
		Name: "noise: crypto", Query: `\bcrypto\b`,
		Actions: []db.RuleAction{{Type: "archive"}}, Enabled: true,
	})
	_, _ = ruleRepo.Create(ctx, db.NewRule{
		Name: "high: regulation", Query: `\bcourt fines\b`,
		Actions: []db.RuleAction{{Type: "boost", Value: "3"}}, Enabled: true,
	})

	svc := NewService(d, Options{UseLLM: false, BatchSize: 10, AgeDays: 30}, nil, nil)
	if _, err := svc.ClassifyAll(ctx); err != nil {
		t.Fatal(err)
	}
	got, _ := artRepo.Get(ctx, 1)
	if got.Status == db.StatusArchived {
		t.Fatalf("a boosted article was archived: %+v", got)
	}
	if got.Importance != 3 {
		t.Fatalf("importance = %d, want 3", got.Importance)
	}
}

// A rules-only run applies the deterministic rules and leaves everything else
// pending: no LLM, no deterministic fallback, no boost floors — those are a
// floor over what the model decides, and the model has not run.
func TestRulesOnlyLeavesTheRestPending(t *testing.T) {
	d, err := db.Open(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	ctx := context.Background()

	src, _ := db.NewSourceRepo(d).Create(ctx, db.NewSource{Name: "S", Type: db.SourceRSS, URL: "u"})
	artRepo := db.NewArticleRepo(d)
	seed := func(guid, title string) {
		_, _, _ = artRepo.Upsert(ctx, db.NewArticle{SourceID: src.ID, GUID: guid, Title: title, URL: "https://x.com/" + guid})
	}
	seed("a", "Bitcoin rips past $120k")             // archive
	seed("b", "OpenAI details its safety framework") // keep
	seed("c", "Nvidia builds a supercomputer")       // boost (floor deferred)
	seed("d", "A quiet paper on retrieval")          // unmatched

	ruleRepo := db.NewRuleRepo(d)
	_, _ = ruleRepo.Create(ctx, db.NewRule{
		Name: "noise: crypto", Query: `\b(bitcoin|crypto)\b`,
		Actions: []db.RuleAction{{Type: "archive"}}, Enabled: true,
	})
	_, _ = ruleRepo.Create(ctx, db.NewRule{
		Name: "keep: labs", Query: `\bopenai\b`,
		Actions: []db.RuleAction{{Type: "keep"}}, Enabled: true,
	})
	_, _ = ruleRepo.Create(ctx, db.NewRule{
		Name: "boost: compute", Query: `\bsupercomputer\b`,
		Actions: []db.RuleAction{{Type: "boost", Value: "3"}}, Enabled: true,
	})

	svc := NewService(d, Options{RulesOnly: true, UseLLM: false, BatchSize: 10, AgeDays: 30}, nil, nil)
	res, err := svc.ClassifyAll(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if res.Classified != 1 || res.ByRules != 1 || res.Archived != 1 || res.Pending != 3 {
		t.Fatalf("res = %+v, want 1 classified by rule, 1 archived, 3 pending", res)
	}
	if res.ByLLM != 0 || res.SkippedNoLLM != 0 {
		t.Fatalf("res = %+v, want no LLM/fallback", res)
	}

	archived, _ := artRepo.Get(ctx, 1)
	if archived.Status != db.StatusArchived {
		t.Fatalf("the archive rule did not run: %+v", archived)
	}
	// The boost floor is not applied yet: the article is still pending, and its
	// importance is whatever it started at.
	boosted, _ := artRepo.Get(ctx, 3)
	if boosted.Importance != 0 {
		t.Fatalf("floor applied early: importance = %d", boosted.Importance)
	}

	pending, err := artRepo.ListUnclassified(ctx, 10, 30)
	if err != nil {
		t.Fatal(err)
	}
	if len(pending) != 3 {
		t.Fatalf("pending = %d, want 3 (keep, boost, unmatched)", len(pending))
	}
}

// The order of the last word on importance is the argument for the whole
// feature: a rule someone wrote beats a habit inferred from history, and
// history beats nothing but the model's guess.
func TestARuleOutranksWhatWasLearned(t *testing.T) {
	d, err := db.Open(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	ctx := context.Background()

	src, _ := db.NewSourceRepo(d).Create(ctx, db.NewSource{Name: "Wire", Type: db.SourceRSS, URL: "https://wire.example/f"})
	artRepo := db.NewArticleRepo(d)
	// Two articles from a source the reader throws away.
	for _, title := range []string{"Ordinary wire item", "Anthropic releases a system card"} {
		id, _, err := artRepo.Upsert(ctx, db.NewArticle{
			SourceID: src.ID, GUID: title, URL: "https://wire.example/" + title, Title: title,
		})
		if err != nil {
			t.Fatal(err)
		}
		_ = id
	}
	// A boost rule claims the second one.
	ruleRepo := db.NewRuleRepo(d)
	if _, err := ruleRepo.Create(ctx, db.NewRule{
		Name: "high: system cards", Query: `system card`,
		Actions: []db.RuleAction{{Type: "boost", Value: "3"}}, Enabled: true,
	}); err != nil {
		t.Fatal(err)
	}
	// And the reader's history says everything from this source is worth less.
	for i := 0; i < 25; i++ {
		id, _, err := artRepo.Upsert(ctx, db.NewArticle{
			SourceID: src.ID, GUID: fmt.Sprintf("old%d", i), URL: fmt.Sprintf("https://wire.example/old%d", i),
			Title: "Old wire item",
		})
		if err != nil {
			t.Fatal(err)
		}
		if err := artRepo.ApplyClassification(ctx, id, "industry", "", nil, nil, 2); err != nil {
			t.Fatal(err)
		}
		if err := artRepo.SetStatus(ctx, id, db.StatusArchived, db.ActorUser); err != nil {
			t.Fatal(err)
		}
	}
	signals, err := learn.Compute(ctx, d, learn.Options{MinSamples: 20, MaxDelta: 1})
	if err != nil {
		t.Fatal(err)
	}
	if err := learn.Store(ctx, d, signals); err != nil {
		t.Fatal(err)
	}

	svc := NewService(d, Options{UseLLM: false, BatchSize: 10, AgeDays: 30, Learn: true, MaxDelta: 1}, nil, nil)
	if _, err := svc.ClassifyAll(ctx); err != nil {
		t.Fatal(err)
	}

	plain, err := artRepo.Get(ctx, 1)
	if err != nil {
		t.Fatal(err)
	}
	// defaultImportance would have given it 1; the history takes it down.
	if plain.Importance != 0 {
		t.Fatalf("unclaimed article = %d, want 0 (moved down by history)", plain.Importance)
	}
	claimed, err := artRepo.Get(ctx, 2)
	if err != nil {
		t.Fatal(err)
	}
	if claimed.Importance != 3 {
		t.Fatalf("boosted article = %d, want 3 — a rule outranks a habit", claimed.Importance)
	}
}
