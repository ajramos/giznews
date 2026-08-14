package sources

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/mmcdole/gofeed"
)

// RSSFetcher reads an RSS/Atom feed and normalizes its entries.
type RSSFetcher struct {
	url    string
	params rssParams
}

type rssParams struct {
	// UserAgent overrides the default UA if set.
	UserAgent string `json:"user_agent"`
}

// NewRSSFetcher builds a fetcher for a generic RSS/Atom feed.
func NewRSSFetcher(url, paramsJSON string) *RSSFetcher {
	f := &RSSFetcher{url: url}
	if paramsJSON != "" {
		_ = json.Unmarshal([]byte(paramsJSON), &f.params)
	}
	return f
}

func (f *RSSFetcher) Fetch(ctx context.Context) ([]*Item, error) {
	if strings.TrimSpace(f.url) == "" {
		return nil, fmt.Errorf("rss: empty feed URL")
	}
	fp := gofeed.NewParser()
	if f.params.UserAgent != "" {
		fp.UserAgent = f.params.UserAgent
	}
	feed, err := fp.ParseURLWithContext(f.url, ctx)
	if err != nil {
		return nil, fmt.Errorf("rss parse %s: %w", f.url, err)
	}

	items := make([]*Item, 0, len(feed.Items))
	for _, it := range feed.Items {
		guid := it.GUID
		if guid == "" {
			guid = it.Link
		}
		if guid == "" {
			guid = it.Title
		}
		url := it.Link
		if url == "" {
			url = it.GUID
		}
		if url == "" {
			continue
		}

		html := it.Content
		if html == "" {
			html = it.Description
		}
		author := ""
		if it.Author != nil {
			author = it.Author.Name
			if author == "" {
				author = it.Author.Email
			}
		}
		pub := it.PublishedParsed
		if pub == nil {
			pub = it.UpdatedParsed
		}

		item := &Item{
			GUID:        guid,
			URL:         url,
			Title:       strings.TrimSpace(it.Title),
			Author:      author,
			ContentHTML: html,
			ContentMD:   htmlToMarkdown(html),
		}
		if pub != nil {
			item.Published = *pub
		}
		items = append(items, item)
	}
	return items, nil
}

// ArxivFetcher reads the arXiv atom/RSS feed for a category or search URL.
// arXiv exposes per-category RSS at http://export.arxiv.org/rss/<category>.
type ArxivFetcher struct {
	url string
}

// NewArxivFetcher builds an arXiv fetcher.
func NewArxivFetcher(url string) *ArxivFetcher {
	return &ArxivFetcher{url: url}
}

func (f *ArxivFetcher) Fetch(ctx context.Context) ([]*Item, error) {
	url := f.url
	if url == "" {
		url = "http://export.arxiv.org/rss/cs.AI"
	}
	fp := gofeed.NewParser()
	feed, err := fp.ParseURLWithContext(url, ctx)
	if err != nil {
		return nil, fmt.Errorf("arxiv parse %s: %w", url, err)
	}

	items := make([]*Item, 0, len(feed.Items))
	for _, it := range feed.Items {
		guid := it.GUID
		if guid == "" {
			guid = it.Link
		}
		if guid == "" {
			continue
		}
		html := it.Content
		if html == "" {
			html = it.Description
		}
		var pub *time.Time
		if it.PublishedParsed != nil {
			pub = it.PublishedParsed
		} else if it.UpdatedParsed != nil {
			pub = it.UpdatedParsed
		}
		item := &Item{
			GUID:        guid,
			URL:         it.Link,
			Title:       strings.TrimSpace(it.Title),
			ContentHTML: html,
			ContentMD:   htmlToMarkdown(html),
		}
		if pub != nil {
			item.Published = *pub
		}
		items = append(items, item)
	}
	return items, nil
}
