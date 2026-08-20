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

// The high-value set hands out three stars, which is what puts an article into
// the knowledge base. Same contract as the noise set, in the other direction:
// these headlines earn it, those are left for the model to judge.
func TestShippedHighValueRulesetDoesWhatItSays(t *testing.T) {
	d, err := db.Open(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	ctx := context.Background()
	repo := db.NewRuleRepo(d)

	// Both sets, as a reader would have them, so the interaction is covered too.
	for _, name := range []string{"noise.json", "high-value.json"} {
		if _, _, _, err := importRules(ctx, repo, filepath.Join("..", "..", "docs", "rules", name), false); err != nil {
			t.Fatal(err)
		}
	}
	rules, err := classify.CompileAll(ctx, repo)
	if err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		title string
		floor int // 0 = no boost: the model decides on its own
	}{
		{"OpenAI announces GPT-5 with a new reasoning mode", 3},
		{"Claude 5 is here, and it runs for a day unattended", 3},
		{"Mistral open-sources a 24B model under Apache 2.0", 3},
		{"Anthropic publishes the Claude 5 system card", 3},
		{"EU AI Act enforcement begins for general-purpose models", 3},
		{"NYT lawsuit against OpenAI survives motion to dismiss", 3},
		{"Anthropic raises $10 billion at a $350B valuation", 3},
		{"New model beats human experts on GPQA Diamond", 3},
		{"US tightens chip export controls on inference accelerators", 3},
		{"xAI announces a $6 billion data center in Memphis", 3},
		{"Nvidia unveils the B300 accelerator", 3},
		{"OpenAI's chief scientist steps down after four years", 3},
		{"Model weights leaked from an unsecured S3 bucket", 3},

		// Real but ordinary: three stars would drown the ones above.
		{"A closer look at how retrieval pipelines drift", 0},
		{"Researchers propose a new attention variant", 0},
		{"Show HN: a local RAG server in 400 lines of Go", 0},
		{"Startup raises $4 million seed for AI note-taking", 0},
		{"Nvidia posts $30 billion in quarterly revenue", 0},
	}

	for _, tc := range cases {
		d := classify.Decide(rules, &db.Article{Title: tc.title, URL: "https://feed/x"})
		if d.Floor != tc.floor {
			t.Errorf("%q\n  got  floor %d (rule %q)\n  want floor %d", tc.title, d.Floor, d.BoostedBy, tc.floor)
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

// Every rules subcommand takes a positional argument, and Go's flag package
// stops reading at the first one — which once made `rules import file.json
// --dry-run` import for real. The flags are found by hand instead, so this is
// the test that says they are found wherever they sit.
func TestFlagsAreReadWhereverTheySit(t *testing.T) {
	cases := []struct {
		name     string
		args     []string
		flag     string
		want     string
		dryRun   bool
		bareWant string
	}{
		{"flag after the file", []string{"noise.json", "--dry-run"}, "", "", true, "noise.json"},
		{"flag before the file", []string{"--dry-run", "noise.json"}, "", "", true, "noise.json"},
		{"no flag at all", []string{"noise.json"}, "", "", false, "noise.json"},
		{"value flag after the regex", []string{`\bshares\b`, "--limit", "5"}, "limit", "5", false, `\bshares\b`},
		{"value flag before the regex", []string{"--limit", "5", `\bshares\b`}, "limit", "5", false, `\bshares\b`},
		{"value flag with an equals sign", []string{`\bshares\b`, "--limit=5"}, "limit", "5", false, `\bshares\b`},
		{"a flag's value is not the positional", []string{"--rule", "noise: crypto"}, "rule", "noise: crypto", false, ""},
		{"config is not the positional", []string{"--config", "/tmp/c.json", "noise.json"}, "", "", false, "noise.json"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := firstNonFlag(tc.args); got != tc.bareWant {
				t.Errorf("firstNonFlag = %q, want %q", got, tc.bareWant)
			}
			if got := hasFlag(tc.args, "dry-run"); got != tc.dryRun {
				t.Errorf("hasFlag(dry-run) = %v, want %v", got, tc.dryRun)
			}
			if tc.flag != "" {
				if got := flagValue(tc.args, tc.flag); got != tc.want {
					t.Errorf("flagValue(%s) = %q, want %q", tc.flag, got, tc.want)
				}
			}
		})
	}
}
