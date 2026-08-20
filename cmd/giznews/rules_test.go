package main

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/ajramos/giznews/internal/classify"
	"github.com/ajramos/giznews/internal/db"
)

// The shipped ruleset is a file of regexes that quietly archive things. This
// test is the contract: these headlines survive, those do not. A regex widened
// by one character breaks it here rather than in someone's unread queue.
func TestShippedNoiseRulesetDoesWhatItSays(t *testing.T) {
	d, err := db.Open(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	ctx := context.Background()
	repo := db.NewRuleRepo(d)

	created, _, _, err := importRules(ctx, repo, filepath.Join("..", "..", "docs", "rules", "noise.json"), false)
	if err != nil {
		t.Fatal(err)
	}
	if created == 0 {
		t.Fatal("the ruleset imported nothing")
	}
	rules, err := classify.CompileAll(ctx, repo)
	if err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		title  string
		url    string
		effect string // "" = no rule fires, the article goes to the model
	}{
		// Survives — either untouched or explicitly protected.
		{"OpenAI ships GPT-5 with a new reasoning mode", "https://openai.com/blog", "keep"},
		{"OpenAI shares new details on its safety framework", "https://theverge.com/a", "keep"},
		{"Anthropic raises $4B at a $60B valuation", "https://techcrunch.com/a", "keep"},
		{"Sparse attention makes long context cheap", "https://arxiv.org/abs/2508.1", "keep"},
		{"EU opens a consultation on model licensing", "https://politico.eu/a", ""},
		{"Nvidia unveils the B300 accelerator", "https://nvidia.com/b300", ""},
		{"Show HN: a local RAG server in 400 lines of Go", "https://news.ycombinator.com/item?id=1", ""},

		// Archived.
		{"Top 10 AI tools you need in 2026", "https://seo.com/a", "archive"},
		{"7 ChatGPT prompts to 10x your productivity", "https://medium.com/a", "archive"},
		{"Sponsored: how AcmeCloud speeds up inference", "https://ads.com/a", "archive"},
		{"Bitcoin rips past $120k as ETF inflows surge", "https://coindesk.com/a", "archive"},
		{"Nvidia shares jump 8% after earnings call", "https://cnbc.com/a", "archive"},
		{"Prime Day: best deals on gaming laptops", "https://deals.com/a", "archive"},
		{"Podcast: what agents mean for the enterprise", "https://acme.fm/podcast/ep-42", "archive"},
		{"Acme (YC W24) is hiring a founding engineer", "https://news.ycombinator.com/item?id=2", "archive"},
		{"Ask HN: what is your AI stack in 2026?", "https://news.ycombinator.com/item?id=3", "archive"},
		{"You won't believe what this chatbot said next", "https://clickbait.com/a", "archive"},

		// The rules that ship switched off must not fire until someone says so.
		{"AcmeAI Announces Series B Funding", "https://prnewswire.com/a", ""},
		{"This Week in AI: agents, chips and lawsuits", "https://newsletter.com/a", ""},
		{"iPhone 18 review: Apple's AI finally works", "https://theverge.com/b", ""},
	}

	for _, tc := range cases {
		article := &db.Article{Title: tc.title, URL: tc.url}
		match := classify.MatchFirstRule(rules, article)
		got, name := "", ""
		if match != nil {
			name = match.Name
			switch {
			case match.Keep:
				got = "keep"
			case match.Archive:
				got = "archive"
			default:
				got = "classify"
			}
		}
		if got != tc.effect {
			t.Errorf("%q\n  got  %q (rule %q)\n  want %q", tc.title, got, name, tc.effect)
		}
	}
}

// Importing the same file twice must change nothing: the file is the source of
// truth, and a second import that duplicated every rule would double the
// prefilter's work and make its order meaningless.
func TestRulesImportIsIdempotent(t *testing.T) {
	d, err := db.Open(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	ctx := context.Background()
	repo := db.NewRuleRepo(d)
	path := filepath.Join("..", "..", "docs", "rules", "noise.json")

	created, updated, unchanged, err := importRules(ctx, repo, path, false)
	if err != nil {
		t.Fatal(err)
	}
	if updated != 0 || unchanged != 0 {
		t.Fatalf("first import = %d new, %d updated, %d unchanged", created, updated, unchanged)
	}
	again, updatedAgain, unchangedAgain, err := importRules(ctx, repo, path, false)
	if err != nil {
		t.Fatal(err)
	}
	if again != 0 || updatedAgain != 0 || unchangedAgain != created {
		t.Fatalf("second import = %d new, %d updated, %d unchanged (want %d unchanged)",
			again, updatedAgain, unchangedAgain, created)
	}
	all, err := repo.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != created {
		t.Fatalf("%d rules stored after two imports, want %d", len(all), created)
	}

	// A dry run reports what it would do and writes nothing.
	all[0].Query = "changed"
	if err := repo.Update(ctx, all[0].ID, db.NewRule{
		Name: all[0].Name, Query: "changed", Actions: all[0].Actions, Enabled: all[0].Enabled,
	}); err != nil {
		t.Fatal(err)
	}
	_, wouldUpdate, _, err := importRules(ctx, repo, path, true)
	if err != nil {
		t.Fatal(err)
	}
	if wouldUpdate != 1 {
		t.Fatalf("dry run said %d would be updated, want 1", wouldUpdate)
	}
	after, _ := repo.Get(ctx, all[0].ID)
	if after.Query != "changed" {
		t.Fatal("the dry run wrote to the database")
	}
}
