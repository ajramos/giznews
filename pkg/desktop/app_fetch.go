package desktop

import (
	"context"
	"fmt"

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
	svc, err := fetch.NewService(a.db, man, a.logger())
	if err != nil {
		return nil, err
	}
	svc.SetMaxAge(a.cfg.Fetch.MaxAgeDays)
	if a.cfg.Extract.OnFetch {
		svc.SetExtraction(a.cfg.Extract.Limit, a.cfg.Extract.Concurrency)
	}
	svc.SetSourceWarnAfter(a.cfg.Fetch.SourceWarnAfter)
	return svc, nil
}

type logWriter struct{}

func (logWriter) Write(p []byte) (int, error) { return len(p), nil }

// Fetch runs the pipeline and maps the result to the API DTO.
func (a *App) Fetch(ctx context.Context) (*FetchResult, error) {
	var result *FetchResult
	err := a.trackJob(ctx, "Fetch articles", "fetch", func(jctx context.Context, p *jobProgress) error {
		svc, err := a.fetchService()
		if err != nil {
			return fmt.Errorf("fetch service: %w", err)
		}
		res, err := svc.FetchAll(jctx)
		if err != nil {
			return err
		}
		result = &FetchResult{
			NewArticles:    res.NewArticles,
			Grouped:        res.Grouped,
			Updated:        res.Updated,
			SourcesFetched: res.SourcesFetched,
			SourcesFailed:  res.SourcesFailed,
			Extracted:      res.Extracted,
			ElapsedMs:      res.ElapsedMs,
		}
		msg := fmt.Sprintf("%d new · %d extracted", res.NewArticles, res.Extracted)
		if res.Grouped > 0 {
			msg += fmt.Sprintf(" · %d joined a story", res.Grouped)
		}
		p.Message(msg)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}
