package desktop

import (
	"context"
	"fmt"
	"log"

	"github.com/ajramos/giznews/internal/fetch"
	"github.com/ajramos/giznews/internal/sources"
)

// fetchService is built lazily (config + sources.Manager + db).
func (a *App) fetchService() (*fetch.Service, error) {
	man := sources.NewManager(
		a.cfg.ResolveGmailCredentialsPath(),
		a.cfg.ResolveGmailTokenPath(),
		a.cfg.Gmail.Queries,
		a.cfg.Gmail.MaxAge,
	)
	svc, err := fetch.NewService(a.db, man, log.New(logWriter{}, "giznews: ", 0))
	if err != nil {
		return nil, err
	}
	if a.cfg.Extract.OnFetch {
		svc.SetExtraction(a.cfg.Extract.Limit, a.cfg.Extract.Concurrency)
	}
	return svc, nil
}

type logWriter struct{}

func (logWriter) Write(p []byte) (int, error) { return len(p), nil }

// Fetch runs the pipeline and maps the result to the API DTO.
func (a *App) Fetch(ctx context.Context) (*FetchResult, error) {
	svc, err := a.fetchService()
	if err != nil {
		return nil, fmt.Errorf("fetch service: %w", err)
	}
	res, err := svc.FetchAll(ctx)
	if err != nil {
		return nil, err
	}
	return &FetchResult{
		NewArticles:    res.NewArticles,
		Updated:        res.Updated,
		SourcesFetched: res.SourcesFetched,
		SourcesFailed:  res.SourcesFailed,
		Extracted:      res.Extracted,
		ElapsedMs:      res.ElapsedMs,
	}, nil
}
