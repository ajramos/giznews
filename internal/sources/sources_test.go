package sources

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const sampleRSS = `<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0">
  <channel>
    <title>AI Daily</title>
    <link>https://ai.example</link>
    <item>
      <guid>post-1</guid>
      <title>Local RAG models beat cloud</title>
      <link>https://ai.example/rag-local</link>
      <pubDate>Mon, 12 Aug 2026 09:00:00 GMT</pubDate>
      <author>ana@example.com (Ana)</author>
      <description>&lt;p&gt;New benchmarks show local models winning.&lt;/p&gt;</description>
    </item>
    <item>
      <guid>post-2</guid>
      <title>Agentic workflows explained</title>
      <link>https://ai.example/agents</link>
      <description>No date, no author</description>
    </item>
  </channel>
</rss>`

func TestRSSFetcher(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/rss+xml")
		w.Write([]byte(sampleRSS))
	}))
	defer srv.Close()

	items, err := NewRSSFetcher(srv.URL, "").Fetch(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 {
		t.Fatalf("items = %d, want 2", len(items))
	}

	a := items[0]
	if a.GUID != "post-1" || a.Title != "Local RAG models beat cloud" {
		t.Fatalf("item = %+v", a)
	}
	if a.URL != "https://ai.example/rag-local" {
		t.Fatalf("url = %q", a.URL)
	}
	if a.Author == "" {
		t.Fatal("expected author")
	}
	if a.Published.IsZero() {
		t.Fatal("expected published date")
	}
	if !strings.Contains(a.ContentMD, "local models winning") {
		t.Fatalf("content_md = %q", a.ContentMD)
	}

	b := items[1]
	if !b.Published.IsZero() {
		t.Fatalf("item 2 published should be zero, got %v", b.Published)
	}
}

func TestRSSFetcherEmptyURL(t *testing.T) {
	if _, err := NewRSSFetcher("", "").Fetch(context.Background()); err == nil {
		t.Fatal("expected error for empty URL")
	}
}

func TestArxivFetcher(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/atom+xml")
		w.Write([]byte(sampleRSS))
	}))
	defer srv.Close()

	items, err := NewArxivFetcher(srv.URL).Fetch(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 {
		t.Fatalf("items = %d", len(items))
	}
	if items[0].GUID != "post-1" {
		t.Fatalf("guid = %q", items[0].GUID)
	}
}

func TestHackerNewsFetcher(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/search_by_date" {
			http.NotFound(w, r)
			return
		}
		q := r.URL.Query()
		if q.Get("tags") != "story" {
			t.Errorf("tags = %q", q.Get("tags"))
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"hits": []map[string]any{
				{
					"objectID": "1001", "title": "Show HN: GizNews", "url": "https://x.com",
					"author": "ajramos", "created_at": "2026-08-12T09:00:00Z",
					"points": 100, "num_comments": 20,
				},
				{
					"objectID": "1002", "title": "No URL story",
					"author": "bob", "created_at": "2026-08-12T10:00:00Z",
				},
			},
		})
	}))
	defer srv.Close()

	items, err := NewHackerNewsFetcherWithBase(srv.URL, `{"tags":"story"}`).Fetch(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 {
		t.Fatalf("items = %d", len(items))
	}
	if items[0].GUID != "hn-1001" {
		t.Fatalf("guid = %q", items[0].GUID)
	}
	// No URL → link to HN item page.
	if items[1].URL != "https://news.ycombinator.com/item?id=1002" {
		t.Fatalf("fallback url = %q", items[1].URL)
	}
}

func TestHTMLToMarkdown(t *testing.T) {
	md := htmlToMarkdown("<p>Hello <b>world</b></p>")
	if !strings.Contains(md, "Hello **world**") {
		t.Fatalf("md = %q", md)
	}
	plain := htmlToMarkdown("<div>plain text</div>")
	if !strings.Contains(plain, "plain text") {
		t.Fatalf("plain = %q", plain)
	}
}
