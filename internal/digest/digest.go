// Package digest builds the daily "what happened in AI" summary: articles are
// grouped by theme (category), each theme gets a 2-3 sentence summary, and the
// whole digest gets a one-paragraph overview. The LLM step is optional; without
// it the digest degrades to a deterministic grouped list.
package digest

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/ajramos/giznews/internal/db"
	"github.com/ajramos/giznews/internal/llm"
)

// Theme is one group in a digest.
type Theme struct {
	Name     string        `json:"name"`
	Summary  string        `json:"summary"`
	Articles []*db.Article `json:"articles"`
}

// Digest is the full daily digest.
type Digest struct {
	Date     string   `json:"date"`
	Overview string   `json:"overview"`
	Themes   []*Theme `json:"themes"`
}

// Options configures digest generation.
type Options struct {
	Model               string
	Days                int
	MaxArticlesPerTheme int
	UseLLM              bool
}

// Service generates digests.
type Service struct {
	db     *db.DB
	opts   Options
	prov   llm.Provider
	logger *log.Logger
}

// NewService builds a digest service.
func NewService(database *db.DB, opts Options, prov llm.Provider, logger *log.Logger) *Service {
	if opts.Days <= 0 {
		opts.Days = 7
	}
	if opts.MaxArticlesPerTheme <= 0 {
		opts.MaxArticlesPerTheme = 5
	}
	return &Service{db: database, opts: opts, prov: prov, logger: logger}
}

// Generate produces a digest from recent classified articles.
func (s *Service) Generate(ctx context.Context) (*Digest, error) {
	articles, err := db.NewArticleRepo(s.db).ListRecent(ctx, s.opts.Days, 0)
	if err != nil {
		return nil, fmt.Errorf("digest: list recent: %w", err)
	}

	// Group by category, best importance first.
	groups := map[string][]*db.Article{}
	for _, a := range articles {
		cat := a.Category
		if cat == "" {
			cat = "general"
		}
		groups[cat] = append(groups[cat], a)
	}
	names := make([]string, 0, len(groups))
	for name := range groups {
		sort.SliceStable(groups[name], func(i, j int) bool {
			return groups[name][i].Importance > groups[name][j].Importance
		})
		names = append(names, name)
	}
	sort.Strings(names)

	d := &Digest{
		Date:   time.Now().UTC().Format("2006-01-02"),
		Themes: make([]*Theme, 0, len(names)),
	}
	for _, name := range names {
		arts := groups[name]
		if len(arts) > s.opts.MaxArticlesPerTheme {
			arts = arts[:s.opts.MaxArticlesPerTheme]
		}
		d.Themes = append(d.Themes, &Theme{Name: name, Articles: arts})
	}

	// LLM enrichment (overview + per-theme summaries) in one call.
	if s.opts.UseLLM && s.prov != nil && len(d.Themes) > 0 {
		if err := s.enrich(ctx, d); err != nil {
			if s.logger != nil {
				s.logger.Printf("digest: LLM enrichment failed (falling back to grouped list): %v", err)
			}
		}
	}
	return d, nil
}

// enrich summarizes the digest with a single LLM call.
func (s *Service) enrich(ctx context.Context, d *Digest) error {
	type slimTheme struct {
		Name      string `json:"name"`
		Headlines []struct {
			Title      string `json:"title"`
			Importance int    `json:"importance"`
		} `json:"headlines"`
	}
	themes := make([]slimTheme, 0, len(d.Themes))
	for _, th := range d.Themes {
		st := slimTheme{Name: th.Name}
		for _, a := range th.Articles {
			st.Headlines = append(st.Headlines, struct {
				Title      string `json:"title"`
				Importance int    `json:"importance"`
			}{Title: a.Title, Importance: a.Importance})
		}
		themes = append(themes, st)
	}
	body, _ := json.Marshal(themes)

	resp, err := s.prov.Complete(ctx, llm.CompletionRequest{
		Model:       s.opts.Model,
		Messages:    []llm.Message{{Role: llm.RoleSystem, Content: digestSystemPrompt}, {Role: llm.RoleUser, Content: string(body)}},
		Temperature: 0.2,
	})
	if err != nil {
		return err
	}

	type outShape struct {
		Overview string `json:"overview"`
		Themes   []struct {
			Name    string `json:"name"`
			Summary string `json:"summary"`
		} `json:"themes"`
	}
	var out outShape
	if err := parseJSON(resp.Content, &out); err != nil {
		return err
	}

	d.Overview = out.Overview
	for _, th := range out.Themes {
		for _, gt := range d.Themes {
			if strings.EqualFold(gt.Name, th.Name) {
				gt.Summary = th.Summary
				break
			}
		}
	}
	return nil
}

const digestSystemPrompt = `You write a daily briefing for an AI-industry professional.

Input: JSON of theme groups, each with a name and headlines (title + importance 0-3).

Output: JSON only, no markdown, no preamble:
{"overview": "<one paragraph, 3-4 sentences, the key story of the day>", "themes": [{"name": "<exact theme name>", "summary": "<2-3 sentences synthesizing the headlines, including why it matters>"}]}

Rules:
- Keep theme names exactly as given.
- Overview should call out the single most important development.
- Be specific and factual; no hedging filler.`

// parseJSON extracts a JSON object from an LLM reply (tolerating fences).
func parseJSON(content string, v any) error {
	raw := strings.TrimSpace(content)
	raw = strings.TrimPrefix(raw, "```json")
	raw = strings.TrimPrefix(raw, "```")
	raw = strings.TrimSuffix(raw, "```")
	raw = strings.TrimSpace(raw)
	objRe := regexp.MustCompile(`(?s)\{.*\}`)
	if m := objRe.FindString(raw); m != "" {
		raw = m
	}
	if err := json.Unmarshal([]byte(raw), v); err != nil {
		return fmt.Errorf("digest: parse LLM JSON: %w", err)
	}
	return nil
}
