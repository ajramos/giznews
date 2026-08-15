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

func TestSimHashNearDuplicate(t *testing.T) {	a := SimHash("OpenAI releases GPT-5 with advanced reasoning capabilities today")
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
