package learn

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/ajramos/giznews/internal/db"
)

func TestSignalVerdicts(t *testing.T) {
	opts := Options{MinSamples: 10, MaxDelta: 1}
	cases := []struct {
		name     string
		verdicts []verdict
		want     int
	}{
		{
			name:     "thrown away unopened",
			verdicts: repeat(12, verdict{sourceID: 1, archived: true}),
			want:     -1,
		},
		{
			name:     "starred often enough",
			verdicts: append(repeat(9, verdict{sourceID: 1, read: true}), repeat(3, verdict{sourceID: 1, read: true, starred: true})...),
			want:     1,
		},
		{
			name: "dropped a lot but still starred sometimes",
			verdicts: append(repeat(10, verdict{sourceID: 1, archived: true}),
				repeat(2, verdict{sourceID: 1, read: true, starred: true})...),
			want: 0,
		},
		{
			name:     "too few to judge",
			verdicts: repeat(9, verdict{sourceID: 1, archived: true}),
			want:     0,
		},
		{
			name:     "read but never starred is not a verdict",
			verdicts: repeat(12, verdict{sourceID: 1, read: true}),
			want:     0,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			signals := signalsFrom(tc.verdicts, opts)
			if len(signals) != 1 {
				t.Fatalf("signals = %+v", signals)
			}
			if signals[0].Delta != tc.want {
				t.Fatalf("delta = %d, want %d (%+v)", signals[0].Delta, tc.want, signals[0])
			}
		})
	}
}

func repeat(n int, v verdict) []verdict {
	out := make([]verdict, 0, n)
	for i := 0; i < n; i++ {
		out = append(out, v)
	}
	return out
}

// The one thing that would quietly corrupt everything: a prefilter rule
// archives an article, the learner reads that as "the user hates this", and the
// rules end up teaching themselves.
func TestSystemDecisionsAreNotTaste(t *testing.T) {
	d, err := db.Open(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	ctx := context.Background()

	src, _ := db.NewSourceRepo(d).Create(ctx, db.NewSource{Name: "S", Type: db.SourceRSS, URL: "https://s.example/rss"})
	repo := db.NewArticleRepo(d)
	for i := 0; i < 30; i++ {
		id, _, err := repo.Upsert(ctx, db.NewArticle{
			SourceID: src.ID, GUID: string(rune('a' + i)), URL: "https://s.example/" + string(rune('a'+i)),
			Title: "Something", Status: db.StatusUnread,
		})
		if err != nil {
			t.Fatal(err)
		}
		// Every one of them archived by a rule, none of them by a person.
		if err := repo.SetStatus(ctx, id, db.StatusArchived, db.ActorSystem); err != nil {
			t.Fatal(err)
		}
	}

	signals, err := Compute(ctx, d, Options{MinSamples: 10})
	if err != nil {
		t.Fatal(err)
	}
	if len(signals) != 0 {
		t.Fatalf("the machine's own decisions came back as taste: %+v", signals)
	}
}

// What a person does is learned from, and the rates are what the history says.
func TestComputeReadsTheHistory(t *testing.T) {
	d, err := db.Open(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	ctx := context.Background()

	src, _ := db.NewSourceRepo(d).Create(ctx, db.NewSource{Name: "Wire", Type: db.SourceRSS, URL: "https://wire.example/feed"})
	repo := db.NewArticleRepo(d)
	for i := 0; i < 12; i++ {
		id, _, err := repo.Upsert(ctx, db.NewArticle{
			SourceID: src.ID, GUID: string(rune('a' + i)), URL: "https://wire.example/" + string(rune('a'+i)),
			Title: "Press release", Status: db.StatusUnread,
		})
		if err != nil {
			t.Fatal(err)
		}
		if err := repo.ApplyClassification(ctx, id, "industry", "", []string{"press"}, nil, 1); err != nil {
			t.Fatal(err)
		}
		if err := repo.SetStatus(ctx, id, db.StatusArchived, db.ActorUser); err != nil {
			t.Fatal(err)
		}
	}

	signals, err := Compute(ctx, d, Options{MinSamples: 10})
	if err != nil {
		t.Fatal(err)
	}
	bySource := map[string]Signal{}
	for _, s := range signals {
		bySource[s.Kind+":"+s.Label] = s
	}
	source, ok := bySource["source:Wire"]
	if !ok {
		t.Fatalf("no signal for the source: %+v", signals)
	}
	if source.Samples != 12 || source.DropRate != 1 || source.Delta != -1 {
		t.Fatalf("source signal = %+v", source)
	}
	if source.Match != `wire\.example` {
		t.Fatalf("match = %q, want the escaped domain", source.Match)
	}
	// The tag its articles carry earns the same verdict.
	if tag := bySource["tag:press"]; tag.Delta != -1 {
		t.Fatalf("tag signal = %+v", tag)
	}

	// Stored and loaded, only what moves something comes back.
	if err := Store(ctx, d, signals); err != nil {
		t.Fatal(err)
	}
	adj, err := Load(ctx, d)
	if err != nil {
		t.Fatal(err)
	}
	if got := adj.For(src.ID, []string{"press"}, 1); got != -1 {
		t.Fatalf("adjustment = %d, want -1 — bounded even though source and tag both say so", got)
	}
	if got := adj.For(src.ID, []string{"press"}, 2); got != -2 {
		t.Fatalf("adjustment = %d, want -2 when two steps are allowed", got)
	}
	if got := adj.For(999, []string{"unknown"}, 1); got != 0 {
		t.Fatalf("adjustment for something never seen = %d, want 0", got)
	}
}

func TestDomainOf(t *testing.T) {
	cases := map[string]string{
		"https://prwire.example/feed":    `prwire\.example`,
		"http://news.site.co.uk/rss?x=1": `news\.site\.co\.uk`,
		"":                               "",
	}
	for in, want := range cases {
		if got := domainOf(in); got != want {
			t.Errorf("domainOf(%q) = %q, want %q", in, got, want)
		}
	}
}
