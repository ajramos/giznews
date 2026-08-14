package sources

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"google.golang.org/api/gmail/v1"
)

// openBrowser opens url in the platform default browser.
func openBrowser(url string) error {
	var cmd string
	switch runtime.GOOS {
	case "darwin":
		cmd = "open"
	case "windows":
		cmd = "rundll32"
		return exec.Command(cmd, "url.dll,FileProtocolHandler", url).Start()
	case "linux":
		cmd = "xdg-open"
	default:
		return fmt.Errorf("unsupported platform")
	}
	return exec.Command(cmd, url).Start()
}

// GmailFetcher ingests newsletters from Gmail. The source params control the
// search:
//
//	{"query": "...", "max_results": 50}
//
// When query is empty it falls back to the app-level configured queries joined
// by OR; the age window comes from the app config (newer_than:<days>d).
type GmailFetcher struct {
	cfg    *configConfig
	params gmailParams
	svc    *gmail.Service // injected in tests; nil → auth from cfg
	logger *log.Logger
}

type gmailParams struct {
	Query      string `json:"query"`
	MaxResults int    `json:"max_results"`
}

// NewGmailFetcher builds a fetcher; svc is nil in production (authenticated
// lazily) but can be injected in tests.
func NewGmailFetcher(cfg *configConfig, paramsJSON string) *GmailFetcher {
	f := &GmailFetcher{cfg: cfg, params: gmailParams{MaxResults: 50}}
	if paramsJSON != "" {
		_ = json.Unmarshal([]byte(paramsJSON), &f.params)
	}
	return f
}

// NewGmailFetcherWithService is the test seam.
func NewGmailFetcherWithService(cfg *configConfig, paramsJSON string, svc *gmail.Service) *GmailFetcher {
	f := NewGmailFetcher(cfg, paramsJSON)
	f.svc = svc
	return f
}

func (f *GmailFetcher) Fetch(ctx context.Context) ([]*Item, error) {
	svc := f.svc
	if svc == nil {
		auth := &GmailAuth{
			CredentialsPath: f.cfg.CredentialsPath,
			TokenPath:       f.cfg.TokenPath,
			Logger:          f.logger,
		}
		var err error
		svc, err = auth.Service(ctx)
		if err != nil {
			return nil, err
		}
	}

	query := f.params.Query
	if strings.TrimSpace(query) == "" {
		query = strings.Join(f.cfg.Queries, " OR ")
	}
	days := f.maxAgeDays()
	if days > 0 && strings.TrimSpace(query) != "" {
		query = query + " newer_than:" + fmt.Sprintf("%dd", days)
	}

	max := f.params.MaxResults
	if max <= 0 {
		max = 50
	}

	req := svc.Users.Messages.List("me").MaxResults(int64(max))
	if strings.TrimSpace(query) != "" {
		req = req.Q(query)
	}
	list, err := req.Do()
	if err != nil {
		return nil, fmt.Errorf("gmail: list messages: %w", err)
	}

	items := make([]*Item, 0, len(list.Messages))
	for _, m := range list.Messages {
		it, err := f.fetchMessage(ctx, svc, m.Id)
		if err != nil {
			if f.logger != nil {
				f.logger.Printf("gmail: skip message %s: %v", m.Id, err)
			}
			continue
		}
		items = append(items, it)
	}
	return items, nil
}

func (f *GmailFetcher) fetchMessage(ctx context.Context, svc *gmail.Service, id string) (*Item, error) {
	msg, err := svc.Users.Messages.Get("me", id).Format("full").Do()
	if err != nil {
		return nil, err
	}

	headers := map[string]string{}
	for _, h := range msg.Payload.Headers {
		switch strings.ToLower(h.Name) {
		case "subject", "from", "date", "message-id":
			headers[strings.ToLower(h.Name)] = h.Value
		}
	}

	text := extractText(msg.Payload)
	html := extractHTML(msg.Payload)

	pub := time.Now()
	if d, err := time.Parse(time.RFC1123Z, headers["date"]); err == nil {
		pub = d
	}

	threadURL := ""
	if msg.ThreadId != "" {
		threadURL = "https://mail.google.com/mail/u/0/#all/" + msg.ThreadId
	}

	return &Item{
		GUID:        id,
		URL:         threadURL,
		Title:       headers["subject"],
		Author:      headers["from"],
		ContentHTML: html,
		ContentMD:   text,
		Published:   pub,
	}, nil
}

func (f *GmailFetcher) maxAgeDays() int {
	d, err := time.ParseDuration(f.cfg.MaxAge)
	if err != nil || d <= 0 {
		return 7
	}
	return int(d.Hours() / 24)
}

// extractText walks the MIME tree for the first text/plain part.
func extractText(p *gmail.MessagePart) string {
	if p == nil {
		return ""
	}
	if p.MimeType == "text/plain" && p.Body != nil && p.Body.Data != "" {
		if b, err := base64.URLEncoding.DecodeString(p.Body.Data); err == nil {
			return string(b)
		}
	}
	for _, part := range p.Parts {
		if t := extractText(part); t != "" {
			return t
		}
	}
	return ""
}

// extractHTML walks the MIME tree for the first text/html part.
func extractHTML(p *gmail.MessagePart) string {
	if p == nil {
		return ""
	}
	if p.MimeType == "text/html" && p.Body != nil && p.Body.Data != "" {
		if b, err := base64.URLEncoding.DecodeString(p.Body.Data); err == nil {
			return string(b)
		}
	}
	for _, part := range p.Parts {
		if h := extractHTML(part); h != "" {
			return h
		}
	}
	return ""
}
