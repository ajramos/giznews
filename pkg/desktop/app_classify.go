package desktop

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

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
			Limit:           limit,
			BatchSize:       a.cfg.Classify.BatchSize,
			Concurrency:     a.cfg.Classify.Concurrency,
			CoverageSources: a.cfg.Classify.CoverageSources,
			CoverageFloor:   a.cfg.Classify.CoverageFloor,
			Learn:           a.cfg.Classify.Learn.Enabled,
			MaxDelta:        a.cfg.Classify.Learn.MaxDelta,
			UseLLM:          a.cfg.Classify.UseLLM && prov != nil,
			Model:           a.cfg.LLM.Model,
			Language:        a.cfg.LLM.Language,
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
			Watched:      res.Watched,
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

// ClassifyArticles classifies an explicit set of articles (bulk selection),
// letting the user prioritize specific items. Runs as a background job.
func (a *App) ClassifyArticles(ctx context.Context, ids []int64) (*ClassifyResult, error) {
	var result *ClassifyResult
	err := a.trackJob(ctx, fmt.Sprintf("Classify %d selected", len(ids)), "classify", func(jctx context.Context, p *jobProgress) error {
		prov, err := a.provider()
		if err != nil {
			return err
		}
		svc := classify.NewService(a.db, classify.Options{
			Limit:           0,
			BatchSize:       a.cfg.Classify.BatchSize,
			Concurrency:     a.cfg.Classify.Concurrency,
			CoverageSources: a.cfg.Classify.CoverageSources,
			CoverageFloor:   a.cfg.Classify.CoverageFloor,
			Learn:           a.cfg.Classify.Learn.Enabled,
			MaxDelta:        a.cfg.Classify.Learn.MaxDelta,
			UseLLM:          a.cfg.Classify.UseLLM && prov != nil,
			Model:           a.cfg.LLM.Model,
			Language:        a.cfg.LLM.Language,
			OnProgress: func(phase string, done, total int) {
				p.Progress(phase, done, total)
			},
		}, prov, a.logger())

		res, err := svc.ClassifyIDs(jctx, ids)
		if err != nil {
			return err
		}
		result = &ClassifyResult{
			Classified:   res.Classified,
			ByRules:      res.ByRules,
			ByLLM:        res.ByLLM,
			SkippedNoLLM: res.SkippedNoLLM,
			Watched:      res.Watched,
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

// ClassifyRules applies the deterministic rules only, leaving every article no
// rule resolved — keep, boost and unmatched — pending for a later LLM classify.
// It never calls the model, so it needs no provider.
func (a *App) ClassifyRules(ctx context.Context, limit int) (*ClassifyResult, error) {
	var result *ClassifyResult
	err := a.trackJob(ctx, "Apply rules", "classify", func(jctx context.Context, p *jobProgress) error {
		svc := classify.NewService(a.db, classify.Options{
			Limit:     limit,
			RulesOnly: true,
			OnProgress: func(phase string, done, total int) {
				p.Progress(phase, done, total)
			},
		}, nil, a.logger())

		res, err := svc.ClassifyAll(jctx)
		if err != nil {
			return err
		}
		result = &ClassifyResult{
			Classified: res.Classified,
			ByRules:    res.ByRules,
			Archived:   res.Archived,
			Pending:    res.Pending,
			Watched:    res.Watched,
			Errors:     res.Errors,
		}
		p.Message(fmt.Sprintf("%d resolved by rules (%d archived) · %d left for LLM", res.ByRules, res.Archived, res.Pending))
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
		summary, err := summarizeOne(jctx, prov, a.cfg.LLM.Model, a.cfg.LLM.Language, art)
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

func summarizeOne(ctx context.Context, prov llm.Provider, model, language string, art *db.Article) (string, error) {
	resp, err := prov.Complete(ctx, llm.CompletionRequest{
		Model: model,
		Messages: []llm.Message{
			{Role: llm.RoleSystem, Content: "You summarize AI news articles for a busy professional. Return 2-4 sentences: what happened, who is involved, and why it matters. No markdown, no preamble." + llm.LanguageInstruction(language)},
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
			Language:            a.cfg.LLM.Language,
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
		if err := digest.Save(jctx, a.db, d); err != nil {
			a.logger().Printf("save digest: %v", err)
		}
		p.Message(fmt.Sprintf("%d themes", len(out.Themes)))
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// ExportDigest writes a stored digest to a file next to the database and
// returns its path, so the app can hand it to the reader (or to whatever opens
// .html) without inventing a file dialog.
//
// date is empty for today's. format is "md" or "html".
func (a *App) ExportDigest(ctx context.Context, date, format string) (string, error) {
	if date == "" {
		date = time.Now().UTC().Format("2006-01-02")
	}
	d, err := digest.Load(ctx, a.db, date)
	if err != nil {
		return "", fmt.Errorf("no digest stored for %s", date)
	}

	body, ext := digest.Markdown(d), "md"
	if strings.EqualFold(format, "html") {
		body, ext = digest.HTML(d), "html"
	}
	dir := filepath.Join(filepath.Dir(a.cfg.ResolveDBPath()), "digests")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	path := filepath.Join(dir, date+"."+ext)
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		return "", err
	}
	return path, nil
}

// WatchHitDTO is an article a watch caught.
type WatchHitDTO struct {
	Rule      string      `json:"rule"`
	Seen      bool        `json:"seen"`
	CreatedAt string      `json:"created_at"`
	Article   *ArticleDTO `json:"article"`
}

// ListWatchHits returns what the watches have caught, newest first.
func (a *App) ListWatchHits(ctx context.Context, onlyUnseen bool) ([]*WatchHitDTO, error) {
	hits, err := db.NewWatchRepo(a.db).List(ctx, onlyUnseen, 50)
	if err != nil {
		return nil, err
	}
	out := make([]*WatchHitDTO, 0, len(hits))
	for _, h := range hits {
		out = append(out, &WatchHitDTO{
			Rule: h.Rule, Seen: h.Seen, CreatedAt: h.CreatedAt, Article: toArticleDTO(h.Article),
		})
	}
	return out, nil
}

// MarkWatchHitsSeen records that the reader has been shown these, so nothing
// announces them a second time.
func (a *App) MarkWatchHitsSeen(ctx context.Context, ids []int64) error {
	return db.NewWatchRepo(a.db).MarkSeen(ctx, ids)
}
