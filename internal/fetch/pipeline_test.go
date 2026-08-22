package fetch

import (
	"context"
	"log"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ajramos/giznews/internal/db"
	"github.com/ajramos/giznews/internal/sources"
)

const feedA = `<?xml version="1.0"?>
<rss version="2.0"><channel>
<item><guid>a1</guid><title>Neural scaling laws revisited</title>
<link>https://x.example/scale?utm_source=rss&amp;utm_medium=feed</link></item>
<item><guid>a2</guid><title>Quantization at the edge</title>
<link>https://x.example/quant</link></item>
</channel></rss>`

var feedOld = `<?xml version="1.0"?>
<rss version="2.0"><channel>
<item><guid>old1</guid><title>Ancient archive post</title>
<link>https://x.example/old1</link><pubDate>Mon, 01 Jan 2015 00:00:00 GMT</pubDate></item>
<item><guid>recent1</guid><title>Fresh post</title>
<link>https://x.example/recent1</link><pubDate>` + time.Now().UTC().Format(time.RFC1123Z) + `</pubDate></item>
</channel></rss>`

func newPipeline(t *testing.T) (*Service, *db.DB) {
	t.Helper()
	d, err := db.Open(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { d.Close() })
	man := sources.NewManager("", "", nil, "168h")
	svc, err := NewService(d, man, log.New(discard{}, "", 0))
	if err != nil {
		t.Fatal(err)
	}
	return svc, d
}

type discard struct{}

func (discard) Write(p []byte) (int, error) { return len(p), nil }

func TestFetchAllDedupAcrossSources(t *testing.T) {
	svc, d := newPipeline(t)
	ctx := context.Background()
	repo := db.NewSourceRepo(d)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/rss+xml")
		w.Write([]byte(feedA))
	}))
	defer srv.Close()

	s1, _ := repo.Create(ctx, db.NewSource{Name: "S1", Type: db.SourceRSS, URL: srv.URL, Enabled: true})
	// Second source, same content, different URL with tracking params.
	s2, _ := repo.Create(ctx, db.NewSource{Name: "S2", Type: db.SourceRSS, URL: srv.URL + "?feed=2", Enabled: true})
	_ = s2

	res, err := svc.FetchAll(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if res.SourcesFetched != 2 || res.SourcesFailed != 0 {
		t.Fatalf("res = %+v", res)
	}
	if res.NewArticles != 2 {
		t.Fatalf("new = %d, want 2 (dups: %d)", res.NewArticles, res.Duplicates)
	}

	// Second fetch should yield all duplicates.
	res2, err := svc.FetchAll(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if res2.NewArticles != 0 || res2.Duplicates != 4 {
		t.Fatalf("res2 = %+v", res2)
	}

	total, _ := db.NewArticleRepo(d).Count(ctx, "")
	if total != 2 {
		t.Fatalf("total articles = %d, want 2", total)
	}

	// The normalized URL must have had the tracking param stripped.
	arts, _ := db.NewArticleRepo(d).List(ctx, db.ListOptions{Limit: 10})
	found := false
	for _, a := range arts {
		if strings.Contains(a.URL, "utm_source") {
			t.Fatalf("tracking param not stripped: %s", a.URL)
		}
		if a.URL == "https://x.example/scale" {
			found = true
		}
	}
	if !found {
		t.Fatalf("normalized url not found, got %+v", arts)
	}
	_ = s1
}

func TestSetMaxAgeDropsStaleItems(t *testing.T) {
	svc, d := newPipeline(t)
	svc.SetMaxAge(30)
	ctx := context.Background()
	repo := db.NewSourceRepo(d)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/rss+xml")
		w.Write([]byte(feedOld))
	}))
	defer srv.Close()

	if _, err := repo.Create(ctx, db.NewSource{Name: "S", Type: db.SourceRSS, URL: srv.URL, Enabled: true}); err != nil {
		t.Fatal(err)
	}

	res, err := svc.FetchAll(ctx)
	if err != nil {
		t.Fatal(err)
	}
	// Only the fresh post should have been ingested; the 2015 post is dropped.
	if res.NewArticles != 1 {
		t.Fatalf("new = %d, want 1 (dups/skipped: %d)", res.NewArticles, res.Duplicates)
	}
	total, _ := db.NewArticleRepo(d).Count(ctx, "")
	if total != 1 {
		t.Fatalf("total articles = %d, want 1", total)
	}
}

func TestSimHashNearDuplicate(t *testing.T) {
	a := SimHash("OpenAI releases GPT-5 with advanced reasoning capabilities today")
	b := SimHash("OpenAI releases GPT-5 with advanced reasoning capabilities today!")
	if HammingDistance(a, b) > 2 {
		t.Fatalf("near duplicates should be close: %d", HammingDistance(a, b))
	}
	c := SimHash("The price of bananas in Argentina jumped 40% this week")
	if HammingDistance(a, c) < 20 {
		t.Fatalf("unrelated texts should be far apart: %d", HammingDistance(a, c))
	}
}

func TestNormalizeURL(t *testing.T) {
	cases := map[string]string{
		"https://x.com/a?utm_source=rss&utm_medium=feed&b=1": "https://x.com/a?b=1",
		"https://x.com/a#frag":                               "https://x.com/a",
		"http://X.com/Path/":                                 "http://x.com/Path",
		"https://x.com/a?fbclid=123&id=4":                    "https://x.com/a?id=4",
	}
	for in, want := range cases {
		if got := NormalizeURL(in); got != want {
			t.Errorf("NormalizeURL(%q) = %q, want %q", in, got, want)
		}
	}
}

// Five outlets running the same story is not five articles and it is not one
// article either: it is one story with five copies, and how many picked it up
// is the point. Dropping the copies, which is what the pipeline used to do,
// threw that away.
func TestNearDuplicatesJoinOneStory(t *testing.T) {
	svc, d := newPipeline(t)
	ctx := context.Background()
	repo := db.NewSourceRepo(d)

	// The same headline as five newsrooms would write it, each on its own URL.
	headlines := []string{
		"OpenAI launches GPT-5 with a new reasoning mode",
		"OpenAI launches GPT-5 with a new reasoning mode today",
		"OpenAI launches GPT-5 with new reasoning mode",
		"OpenAI launches GPT-5 with a new reasoning mode.",
		"OpenAI launches GPT-5 with a new reasoning mode!",
	}
	for i, headline := range headlines {
		feed := `<?xml version="1.0"?><rss version="2.0"><channel>
			<item><guid>g` + itoa(i) + `</guid><title>` + headline + `</title>
			<link>https://outlet` + itoa(i) + `.example/gpt5</link></item>
			</channel></rss>`
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/rss+xml")
			w.Write([]byte(feed))
		}))
		defer srv.Close()
		if _, err := repo.Create(ctx, db.NewSource{
			Name: "Outlet " + itoa(i), Type: db.SourceRSS, URL: srv.URL, Enabled: true,
		}); err != nil {
			t.Fatal(err)
		}
	}

	res, err := svc.FetchAll(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if res.NewArticles != 1 || res.Grouped != 4 {
		t.Fatalf("res = %+v, want 1 new and 4 joined", res)
	}

	artRepo := db.NewArticleRepo(d)
	// Nothing was thrown away.
	total, _ := artRepo.Count(ctx, "")
	if total != 5 {
		t.Fatalf("stored articles = %d, want 5 — copies must be kept", total)
	}
	// But the list is one row, carrying the coverage.
	list, err := artRepo.List(ctx, db.ListOptions{Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 {
		t.Fatalf("list = %d rows, want 1 story", len(list))
	}
	if list[0].StorySize != 5 {
		t.Fatalf("story size = %d, want 5", list[0].StorySize)
	}
	if len(list[0].StorySources) != 5 {
		t.Fatalf("story sources = %v, want 5 outlets", list[0].StorySources)
	}
	// And the copies are reachable, anchored on the first one in.
	members, err := artRepo.StoryMembers(ctx, list[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(members) != 5 || members[0].ID != list[0].ID {
		t.Fatalf("members = %d, anchored on %d (list head %d)", len(members), members[0].ID, list[0].ID)
	}

	// A story costs one row of attention, not five: the classifier and the
	// knowledge base only ever see the anchor.
	pending, err := artRepo.ListUnclassified(ctx, 100, 30)
	if err != nil {
		t.Fatal(err)
	}
	if len(pending) != 1 {
		t.Fatalf("unclassified = %d, want 1 — copies must not be classified again", len(pending))
	}
	if n, _ := artRepo.CountUnclassified(ctx); n != 1 {
		t.Fatalf("unclassified count = %d, want 1", n)
	}

	// Re-fetching changes nothing: same URLs, already known.
	res2, err := svc.FetchAll(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if res2.NewArticles != 0 || res2.Grouped != 0 {
		t.Fatalf("second fetch = %+v, want nothing new", res2)
	}
	if again, _ := artRepo.Count(ctx, ""); again != 5 {
		t.Fatalf("articles after refetch = %d, want 5", again)
	}
}

func itoa(i int) string { return string(rune('0' + i)) }

// The matcher decides whether an article is visible on its own or folded under
// another one, so both kinds of mistake are worth pinning down: the pairs that
// must group, and the ones that must not.
func TestSameStory(t *testing.T) {
	cases := []struct {
		a, b string
		same bool
	}{
		// One story, five newsrooms.
		{"OpenAI launches GPT-5 with a new reasoning mode", "OpenAI launches GPT-5 with a new reasoning mode today", true},
		{"OpenAI launches GPT-5 with a new reasoning mode", "OpenAI launches GPT-5 with new reasoning mode", true},
		{"EU opens a consultation on model licensing", "EU opens a consultation on model licensing rules", true},
		{"Anthropic raises $10B at a $350B valuation", "Anthropic closes $10B round at $350B valuation", true},

		// Different events that read alike. The Apple pair is the reason a
		// ratio alone is not enough: it overlaps as much as a real rewrite and
		// differs in the only word that matters.
		{"Apple brings AI to the iPhone", "Apple brings AI to the iPad", false},
		{"OpenAI launches GPT-5 with a new reasoning mode", "OpenAI launches GPT-4.5 with a new reasoning mode", false},
		{"EU opens a consultation on model licensing", "EU fines Meta over ad-free subscriptions", false},
		{"A closer look at how retrieval pipelines drift", "Show HN: a local RAG server in 400 lines of Go", false},

		// A rewrite deep enough that only the subject survives is left alone:
		// missing a pair costs a duplicate row, a false pair hides an article.
		{"OpenAI launches GPT-5 with a new reasoning mode", "OpenAI ships GPT-5, its most capable reasoning model", false},
	}
	for _, tc := range cases {
		got := SameStory(TitleTokens(tc.a), TitleTokens(tc.b))
		if got != tc.same {
			t.Errorf("SameStory(%q, %q) = %v, want %v (similarity %.2f)",
				tc.a, tc.b, got, tc.same, TitleSimilarity(TitleTokens(tc.a), TitleTokens(tc.b)))
		}
	}
}

// A source that fails, recovers, and then goes empty is told about exactly
// once at each threshold crossing, and a healthy run clears the counters.
func TestSourceHealthWarnsOnceAtThreshold(t *testing.T) {
	d, err := db.Open(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { d.Close() })
	ctx := context.Background()

	var logBuf strings.Builder
	man := sources.NewManager("", "", nil, "168h")
	svc, err := NewService(d, man, log.New(&logBuf, "", 0))
	if err != nil {
		t.Fatal(err)
	}
	svc.SetSourceWarnAfter(3)

	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if calls <= 3 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/rss+xml")
		w.Write([]byte(feedA))
	}))
	defer srv.Close()

	repo := db.NewSourceRepo(d)
	if _, err := repo.Create(ctx, db.NewSource{Name: "S", Type: db.SourceRSS, URL: srv.URL, Enabled: true}); err != nil {
		t.Fatal(err)
	}

	// Three failed fetches, then a successful one.
	for i := 0; i < 4; i++ {
		if _, err := svc.FetchAll(ctx); err != nil {
			t.Fatal(err)
		}
	}

	if n := strings.Count(logBuf.String(), "looks broken"); n != 1 {
		t.Fatalf("warned %d time(s), want exactly 1 (once per threshold crossing):\n%s", n, logBuf.String())
	}

	src, _ := repo.Get(ctx, 1)
	if src.ConsecutiveFailures != 0 || src.LastError != "" || src.LastOK == "" {
		t.Fatalf("after recovery = %+v", src)
	}
}

// A feed that answers but brings in nothing is flagged as suspect once it has
// been empty for the threshold number of cycles.
func TestSourceThatGoesEmptyIsFlagged(t *testing.T) {
	d, err := db.Open(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { d.Close() })
	ctx := context.Background()

	var logBuf strings.Builder
	man := sources.NewManager("", "", nil, "168h")
	svc, err := NewService(d, man, log.New(&logBuf, "", 0))
	if err != nil {
		t.Fatal(err)
	}
	svc.SetSourceWarnAfter(3)

	emptyFeed := `<?xml version="1.0"?><rss version="2.0"><channel><title>Nothing</title></channel></rss>`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/rss+xml")
		w.Write([]byte(emptyFeed))
	}))
	defer srv.Close()

	repo := db.NewSourceRepo(d)
	if _, err := repo.Create(ctx, db.NewSource{Name: "S", Type: db.SourceRSS, URL: srv.URL, Enabled: true}); err != nil {
		t.Fatal(err)
	}

	for i := 0; i < 3; i++ {
		if _, err := svc.FetchAll(ctx); err != nil {
			t.Fatal(err)
		}
	}

	if n := strings.Count(logBuf.String(), "returned nothing"); n != 1 {
		t.Fatalf("warned %d time(s), want exactly 1:\n%s", n, logBuf.String())
	}
	src, _ := repo.Get(ctx, 1)
	if src.EmptyCycles != 3 || src.LastError == "" {
		t.Fatalf("source not flagged: %+v", src)
	}
}
