package sources

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"time"

	"golang.org/x/oauth2"
	goauth "golang.org/x/oauth2/google"
	"google.golang.org/api/gmail/v1"
	"google.golang.org/api/option"
)

// ErrNotAuthorized signals that Gmail OAuth has not been set up yet (or the
// token expired and cannot be refreshed). The UI/CLI should trigger the
// browser flow via Authorize.
var ErrNotAuthorized = fmt.Errorf("gmail: not authorized (run `giznews gmail-auth`)")

// gmailScope is the minimal read scope needed to ingest newsletters.
const gmailScope = gmail.GmailReadonlyScope

// GmailAuth loads credentials + token (shared with giztui by default) and
// produces an authenticated gmail.Service. Endpoint is overridable for tests.
type GmailAuth struct {
	CredentialsPath string
	TokenPath       string
	Logger          *log.Logger
	// Endpoint overrides the Gmail API base (test seam).
	Endpoint string
}

// Config builds the OAuth2 client config from the credentials JSON.
func (a *GmailAuth) Config() (*oauth2.Config, error) {
	b, err := os.ReadFile(a.CredentialsPath)
	if err != nil {
		return nil, fmt.Errorf("gmail: read credentials %s: %w", a.CredentialsPath, err)
	}
	cfg, err := goauth.ConfigFromJSON(b, gmailScope)
	if err != nil {
		return nil, fmt.Errorf("gmail: parse credentials: %w", err)
	}
	return cfg, nil
}

// Token loads the persisted oauth2 token, or nil if it does not exist.
func (a *GmailAuth) Token() (*oauth2.Token, error) {
	b, err := os.ReadFile(a.TokenPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("gmail: read token: %w", err)
	}
	tok := &oauth2.Token{}
	if err := json.Unmarshal(b, tok); err != nil {
		return nil, fmt.Errorf("gmail: parse token: %w", err)
	}
	return tok, nil
}

// SaveToken persists an oauth2 token to TokenPath.
func (a *GmailAuth) SaveToken(tok *oauth2.Token) error {
	if tok == nil {
		return fmt.Errorf("gmail: cannot save nil token")
	}
	b, err := json.MarshalIndent(tok, "", "  ")
	if err != nil {
		return fmt.Errorf("gmail: marshal token: %w", err)
	}
	if err := os.WriteFile(a.TokenPath, b, 0o600); err != nil {
		return fmt.Errorf("gmail: write token: %w", err)
	}
	return nil
}

// Service returns an authenticated Gmail client, refreshing the token if
// needed. Returns ErrNotAuthorized when no valid token exists.
func (a *GmailAuth) Service(ctx context.Context) (*gmail.Service, error) {
	if _, err := os.Stat(a.CredentialsPath); err != nil {
		return nil, ErrNotAuthorized
	}
	tok, err := a.Token()
	if err != nil {
		return nil, err
	}
	if tok == nil || !tok.Valid() {
		return nil, ErrNotAuthorized
	}

	cfg, err := a.Config()
	if err != nil {
		return nil, err
	}
	client := cfg.Client(ctx, tok)

	opts := []option.ClientOption{option.WithHTTPClient(client)}
	if a.Endpoint != "" {
		opts = append(opts, option.WithEndpoint(a.Endpoint))
	}
	svc, err := gmail.NewService(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("gmail: new service: %w", err)
	}
	return svc, nil
}

// Authorize runs the interactive browser OAuth flow and saves the token.
// It starts a local callback server, opens the consent page, and waits for the
// redirect. Returns immediately with the listener address if the browser could
// not be opened, so the caller can print the URL.
func (a *GmailAuth) Authorize(ctx context.Context) error {
	cfg, err := a.Config()
	if err != nil {
		return err
	}

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return fmt.Errorf("gmail: callback listener: %w", err)
	}
	redirect := "http://" + ln.Addr().String() + "/callback"
	cfg.RedirectURL = redirect

	state := fmt.Sprintf("giznews-%d", time.Now().UnixNano())
	authURL := cfg.AuthCodeURL(state, oauth2.AccessTypeOffline, oauth2.ApprovalForce)

	codeCh := make(chan string, 1)
	errCh := make(chan error, 1)
	mux := http.NewServeMux()
	mux.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("state") != state {
			errCh <- fmt.Errorf("gmail: state mismatch")
			http.Error(w, "state mismatch", http.StatusBadRequest)
			return
		}
		code := r.URL.Query().Get("code")
		if code == "" {
			errCh <- fmt.Errorf("gmail: no code in callback")
			http.Error(w, "missing code", http.StatusBadRequest)
			return
		}
		fmt.Fprint(w, "Authorization complete. You can close this tab and return to giznews.")
		codeCh <- code
	})
	srv := &http.Server{Handler: mux}
	go srv.Serve(ln)
	defer srv.Close()

	if a.Logger != nil {
		a.Logger.Printf("Open this URL in your browser:\n\n%s\n", authURL)
	}
	if err := openBrowser(authURL); err != nil && a.Logger != nil {
		a.Logger.Printf("Could not open a browser automatically: %v\nOpen this URL manually:\n%s", err, authURL)
	}

	select {
	case code := <-codeCh:
		tok, err := cfg.Exchange(ctx, code)
		if err != nil {
			return fmt.Errorf("gmail: exchange: %w", err)
		}
		if err := a.SaveToken(tok); err != nil {
			return err
		}
		return nil
	case err := <-errCh:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}
