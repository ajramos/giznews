package sources

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"google.golang.org/api/gmail/v1"
	"google.golang.org/api/option"
)

func b64(s string) string { return base64.URLEncoding.EncodeToString([]byte(s)) }

func newTestGmailService(t *testing.T) (*gmail.Service, string) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/gmail/v1/users/me/messages":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"messages": []map[string]any{
					{"id": "msg-1"},
					{"id": "msg-2"},
				},
			})
		case strings.HasPrefix(r.URL.Path, "/gmail/v1/users/me/messages/"):
			id := strings.TrimPrefix(r.URL.Path, "/gmail/v1/users/me/messages/")
			body := "From: Substack <noreply@substack.com>\nSubject: AI newsletter #42\nDate: Tue, 12 Aug 2026 09:00:00 +0000\n\nWeekly AI roundup\n\n- Item one\n- Item two\n"
			_ = json.NewEncoder(w).Encode(map[string]any{
				"id":       id,
				"threadId": "thread-" + id,
				"payload": map[string]any{
					"mimeType": "multipart/alternative",
					"headers": []map[string]any{
						{"name": "Subject", "value": "AI newsletter #42"},
						{"name": "From", "value": "Substack <noreply@substack.com>"},
						{"name": "Date", "value": "Tue, 12 Aug 2026 09:00:00 +0000"},
					},
					"parts": []map[string]any{
						{"mimeType": "text/plain", "body": map[string]any{"data": b64(body)}},
					},
				},
			})
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)

	svc, err := gmail.NewService(context.Background(),
		option.WithEndpoint(srv.URL), option.WithHTTPClient(srv.Client()))
	if err != nil {
		t.Fatal(err)
	}
	return svc, srv.URL
}

func TestGmailFetcher(t *testing.T) {
	svc, _ := newTestGmailService(t)
	cfg := &configConfig{MaxAge: "168h", Queries: []string{"from:(substack.com)"}}
	f := NewGmailFetcherWithService(cfg, `{}`, svc)

	items, err := f.Fetch(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 {
		t.Fatalf("items = %d, want 2", len(items))
	}
	it := items[0]
	if it.GUID != "msg-1" {
		t.Fatalf("guid = %q", it.GUID)
	}
	if it.Title != "AI newsletter #42" {
		t.Fatalf("title = %q", it.Title)
	}
	if !strings.Contains(it.ContentMD, "Weekly AI roundup") {
		t.Fatalf("content = %q", it.ContentMD)
	}
	if it.URL != "https://mail.google.com/mail/u/0/#all/thread-msg-1" {
		t.Fatalf("url = %q", it.URL)
	}
	if it.Published.IsZero() {
		t.Fatal("expected published date")
	}
}

func TestGmailFetcherQueryFromParams(t *testing.T) {
	var capturedQ string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/gmail/v1/users/me/messages" {
			capturedQ = r.URL.Query().Get("q")
			_ = json.NewEncoder(w).Encode(map[string]any{"messages": []map[string]any{}})
			return
		}
		http.NotFound(w, r)
	}))
	t.Cleanup(srv.Close)
	svc, err := gmail.NewService(context.Background(),
		option.WithEndpoint(srv.URL), option.WithHTTPClient(srv.Client()))
	if err != nil {
		t.Fatal(err)
	}

	cfg := &configConfig{MaxAge: "168h"}
	f := NewGmailFetcherWithService(cfg, `{"query":"from:(theverge.com)"}`, svc)
	if _, err := f.Fetch(context.Background()); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(capturedQ, "theverge.com") || !strings.Contains(capturedQ, "newer_than:7d") {
		t.Fatalf("query = %q", capturedQ)
	}
}

func TestGmailAuthNotAuthorized(t *testing.T) {
	a := &GmailAuth{CredentialsPath: "/nonexistent/creds.json", TokenPath: "/nonexistent/token.json"}
	_, err := a.Service(context.Background())
	if err == nil || err.Error() != ErrNotAuthorized.Error() {
		t.Fatalf("err = %v, want ErrNotAuthorized", err)
	}
}
