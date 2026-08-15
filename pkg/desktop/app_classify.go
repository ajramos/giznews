package desktop

import (
	"context"
	"fmt"
	"log"

	"github.com/ajramos/giznews/internal/classify"
	"github.com/ajramos/giznews/internal/db"
	"github.com/ajramos/giznews/internal/digest"
	"github.com/ajramos/giznews/internal/llm"
)

func discardLogger() *log.Logger {
	return log.New(logWriter{}, "giznews: ", 0)
}

// Classify runs the classification pipeline (rules + LLM).
func (a *App) Classify(ctx context.Context, limit int) (*ClassifyResult, error) {
	var result *ClassifyResult
	err := a.trackJob(ctx, "Classify articles", "classify", func(jctx context.Context, p *jobProgress) error {
		prov, err := a.provider()
		if err != nil {
			return err
		}
		svc := classify.NewService(a.db, classify.Options{
			Limit:       limit,
			BatchSize:   a.cfg.Classify.BatchSize,
			Concurrency: a.cfg.Classify.Concurrency,
			UseLLM:      a.cfg.Classify.UseLLM && prov != nil,
			Model:       a.cfg.LLM.Model,
			OnProgress: func(phase string, done, total int) {
				p.Progress(phase, done, total)
			},
		}, prov, a.logger())

		res, err := svc.ClassifyAll(jctx)
		if err != nil {
			return err
		}
		result = &ClassifyResult{
			Classified:   res.Classified,
			ByRules:      res.ByRules,
			ByLLM:        res.ByLLM,
			SkippedNoLLM: res.SkippedNoLLM,
			Batches:      res.Batches,
			Errors:       res.Errors,
		}
		msg := fmt.Sprintf("%d classified (%d rules · %d LLM)", res.Classified, res.ByRules, res.ByLLM)
		if len(res.Errors) > 0 {
			msg += fmt.Sprintf(" · %d batch errors", len(res.Errors))
		}
		p.Message(msg)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

// SummarizeArticle generates a summary for one article and persists it.
func (a *App) SummarizeArticle(ctx context.Context, id int64) (*ArticleDTO, error) {
	var out *ArticleDTO
	err := a.trackJob(ctx, "Summarize article", "summarize", func(jctx context.Context, p *jobProgress) error {
		prov, err := a.provider()
		if err != nil {
			return err
		}
		if prov == nil {
			return fmt.Errorf("LLM is disabled (enable it in config) or unreachable")
		}
		repo := db.NewArticleRepo(a.db)
		art, err := repo.Get(jctx, id)
		if err != nil {
			return err
		}
		summary, err := summarizeOne(jctx, prov, a.cfg.LLM.Model, art)
		if err != nil {
			return err
		}
		if err := repo.SetSummary(jctx, id, summary); err != nil {
			return err
		}
		art.Summary = summary
		out = toArticleDTO(art)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

func summarizeOne(ctx context.Context, prov llm.Provider, model string, art *db.Article) (string, error) {
	resp, err := prov.Complete(ctx, llm.CompletionRequest{
		Model: model,
		Messages: []llm.Message{
			{Role: llm.RoleSystem, Content: "You summarize AI news articles for a busy professional. Return 2-4 sentences: what happened, who is involved, and why it matters. No markdown, no preamble."},
			{Role: llm.RoleUser, Content: fmt.Sprintf("Title: %s\n\n%s", art.Title, truncateBodyMD(art.ContentMD))},
		},
		Temperature: 0.3,
	})
	if err != nil {
		return "", err
	}
	return resp.Content, nil
}

func truncateBodyMD(s string) string {
	const max = 3000
	if len(s) <= max {
		return s
	}
	return s[:max] + " …"
}

// Digest generates the daily digest.
func (a *App) Digest(ctx context.Context) (*DigestDTO, error) {
	var out *DigestDTO
	err := a.trackJob(ctx, "Generate digest", "digest", func(jctx context.Context, p *jobProgress) error {
		prov, err := a.provider()
		if err != nil {
			return err
		}
		svc := digest.NewService(a.db, digest.Options{
			Model:               a.cfg.LLM.Model,
			Days:                7,
			MaxArticlesPerTheme: a.cfg.Digest.MaxArticlesPerTheme,
			UseLLM:              a.cfg.LLM.Enabled && prov != nil,
		}, prov, a.logger())

		d, err := svc.Generate(jctx)
		if err != nil {
			return err
		}

		out = &DigestDTO{Date: d.Date, Overview: d.Overview}
		for _, th := range d.Themes {
			dt := &DigestThemeDTO{Theme: th.Name, Summary: th.Summary}
			for _, art := range th.Articles {
				dt.Articles = append(dt.Articles, toArticleDTO(art))
			}
			out.Themes = append(out.Themes, dt)
		}
		p.Message(fmt.Sprintf("%d themes", len(out.Themes)))
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}
