package sources

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// HackerNewsFetcher reads stories from the Algolia HN API.
// Params support:
//
//	{"query": "...", "tags": "story", "hits_per_page": 50, "age_days": 7}
type HackerNewsFetcher struct {
	baseURL string
	params  hnParams
}

type hnParams struct {
	Query       string `json:"query"`
	Tags        string `json:"tags"`
	HitsPerPage int    `json:"hits_per_page"`
	AgeDays     int    `json:"age_days"`
}

const hnDefaultBase = "https://hn.algolia.com/api/v1"

// NewHackerNewsFetcher builds a fetcher using the Algolia search_by_date API.
func NewHackerNewsFetcher(paramsJSON string) *HackerNewsFetcher {
	f := &HackerNewsFetcher{baseURL: hnDefaultBase, params: hnParams{Tags: "story", HitsPerPage: 50, AgeDays: 7}}
	if paramsJSON != "" {
		_ = json.Unmarshal([]byte(paramsJSON), &f.params)
	}
	return f
}

// NewHackerNewsFetcherWithBase is the test seam for injecting a mock API base.
func NewHackerNewsFetcherWithBase(baseURL, paramsJSON string) *HackerNewsFetcher {
	f := NewHackerNewsFetcher(paramsJSON)
	if baseURL != "" {
		f.baseURL = baseURL
	}
	return f
}

type hnHit struct {
	ObjectID  string   `json:"objectID"`
	Title     string   `json:"title"`
	URL       string   `json:"url"`
	Author    string   `json:"author"`
	StoryText string   `json:"story_text"`
	Points    int      `json:"points"`
	Comments  int      `json:"num_comments"`
	CreatedAt string   `json:"created_at"`
	Tags      []string `json:"_tags"`
}

type hnResponse struct {
	Hits []hnHit `json:"hits"`
}

func (f *HackerNewsFetcher) Fetch(ctx context.Context) ([]*Item, error) {
	q := url.Values{}
	tags := f.params.Tags
	if tags == "" {
		tags = "story"
	}
	q.Set("tags", tags)
	if f.params.Query != "" {
		q.Set("query", f.params.Query)
	}
	hits := f.params.HitsPerPage
	if hits <= 0 {
		hits = 50
	}
	q.Set("hitsPerPage", strconv.Itoa(hits))

	endpoint := f.baseURL + "/search_by_date?" + q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "giznews/0.1")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("hn fetch: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return nil, fmt.Errorf("hn fetch: status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var out hnResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("hn decode: %w", err)
	}

	items := make([]*Item, 0, len(out.Hits))
	for _, h := range out.Hits {
		if strings.TrimSpace(h.Title) == "" {
			continue
		}
		link := h.URL
		if link == "" {
			link = "https://news.ycombinator.com/item?id=" + h.ObjectID
		}
		var pub time.Time
		if t, err := time.Parse(time.RFC3339, h.CreatedAt); err == nil {
			pub = t
		}
		body := htmlToMarkdown(h.StoryText)
		if body == "" {
			body = fmt.Sprintf("HN discussion (%d points, %d comments)", h.Points, h.Comments)
		}
		items = append(items, &Item{
			GUID:      "hn-" + h.ObjectID,
			URL:       link,
			Title:     strings.TrimSpace(h.Title),
			Author:    h.Author,
			ContentMD: body,
			Published: pub,
		})
	}
	return items, nil
}
