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
	prov, err := a.provider()
	if err != nil {
		return nil, err
	}
	svc := classify.NewService(a.db, classify.Options{
		Limit:     limit,
		BatchSize: a.cfg.Classify.BatchSize,
		UseLLM:    a.cfg.Classify.UseLLM && prov != nil,
		Model:     a.cfg.LLM.Model,
	}, prov, discardLogger())

	res, err := svc.ClassifyAll(ctx)
	if err != nil {
		return nil, err
	}
	return &ClassifyResult{
		Classified:   res.Classified,
		ByRules:      res.ByRules,
		ByLLM:        res.ByLLM,
		SkippedNoLLM: res.SkippedNoLLM,
		Batches:      res.Batches,
		Errors:       res.Errors,
	}, nil
}

// SummarizeArticle generates a summary for one article and persists it.
func (a *App) SummarizeArticle(ctx context.Context, id int64) (*ArticleDTO, error) {
	prov, err := a.provider()
	if err != nil {
		return nil, err
	}
	if prov == nil {
		return nil, fmt.Errorf("LLM is disabled (enable it in config) or unreachable")
	}
	repo := db.NewArticleRepo(a.db)
	art, err := repo.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	summary, err := summarizeOne(ctx, prov, a.cfg.LLM.Model, art)
	if err != nil {
		return nil, err
	}
	if err := repo.SetSummary(ctx, id, summary); err != nil {
		return nil, err
	}
	art.Summary = summary
	return toArticleDTO(art), nil
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
	prov, err := a.provider()
	if err != nil {
		return nil, err
	}
	svc := digest.NewService(a.db, digest.Options{
		Model:               a.cfg.LLM.Model,
		Days:                7,
		MaxArticlesPerTheme: a.cfg.Digest.MaxArticlesPerTheme,
		UseLLM:              a.cfg.LLM.Enabled && prov != nil,
	}, prov, discardLogger())

	d, err := svc.Generate(ctx)
	if err != nil {
		return nil, err
	}

	out := &DigestDTO{Date: d.Date, Overview: d.Overview}
	for _, th := range d.Themes {
		dt := &DigestThemeDTO{Theme: th.Name, Summary: th.Summary}
		for _, a := range th.Articles {
			dt.Articles = append(dt.Articles, toArticleDTO(a))
		}
		out.Themes = append(out.Themes, dt)
	}
	return out, nil
}
