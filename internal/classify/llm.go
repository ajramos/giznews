package classify

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/ajramos/giznews/internal/db"
	"github.com/ajramos/giznews/internal/llm"
)

// BatchClassify sends a batch of articles to the LLM in one call and returns
// the parsed classifications, keyed by article ID.
func BatchClassify(ctx context.Context, provider llm.Provider, model, language string, batch []*db.Article) (map[int64]*Classification, error) {
	resp, err := provider.Complete(ctx, llm.CompletionRequest{
		Model: model,
		Messages: []llm.Message{
			{Role: llm.RoleSystem, Content: classifySystemPrompt + llm.LanguageInstruction(language)},
			{Role: llm.RoleUser, Content: buildClassifyPrompt(batch)},
		},
		Temperature: 0.1,
	})
	if err != nil {
		return nil, err
	}

	parsed, err := parseClassifications(resp.Content)
	if err != nil {
		return nil, err
	}
	return parsed, nil
}

const classifySystemPrompt = `You are a senior AI-industry analyst. Classify the provided news articles for a reader tracking the artificial intelligence industry.

Rules:
- Respond with ONLY a JSON array of objects, one per article, in the same order.
- Each object: {"id": <number>, "category": "<one of the allowed categories>", "importance": <0-3>, "tags": ["..."], "entities": [{"name":"...","type":"person|org|product|paper|model"}], "summary": "<1-2 sentences>", "headline": "<one phrase that would tell the reader why it matters>"}
- allowed categories: models, research, industry, funding, regulation, tools, open-source, opinion, general
- importance: 3 = must-know this week, 2 = relevant, 1 = background, 0 = noise
- Keep summaries factual, max 2 sentences. No preamble, no markdown fences.`

// buildClassifyPrompt serializes the batch compactly.
func buildClassifyPrompt(batch []*db.Article) string {
	type slim struct {
		ID    int64  `json:"id"`
		Title string `json:"title"`
		Body  string `json:"body"`
	}
	items := make([]slim, 0, len(batch))
	for _, a := range batch {
		body := truncateBody(a.ContentMD)
		if body == "" {
			body = a.Summary
		}
		items = append(items, slim{ID: a.ID, Title: a.Title, Body: body})
	}
	b, _ := json.Marshal(items)
	return string(b)
}

func truncateBody(s string) string {
	const max = 1200
	r := []rune(strings.TrimSpace(s))
	if len(r) <= max {
		return string(r)
	}
	return string(r[:max]) + " …"
}

var jsonBlockRe = regexp.MustCompile(`(?s)\[.*\]`)

func parseClassifications(content string) (map[int64]*Classification, error) {
	raw := strings.TrimSpace(content)
	// Peel markdown code fences if the model wrapped the JSON.
	raw = strings.TrimPrefix(raw, "```json")
	raw = strings.TrimPrefix(raw, "```")
	raw = strings.TrimSuffix(raw, "```")
	raw = strings.TrimSpace(raw)

	m := jsonBlockRe.FindString(raw)
	if m == "" {
		m = raw
	}

	var items []*Classification
	if err := json.Unmarshal([]byte(m), &items); err != nil {
		// Some models return a trailing comma; try a lenient second pass.
		cleaned := strings.ReplaceAll(m, ",\n}", "\n}")
		if err2 := json.Unmarshal([]byte(cleaned), &items); err2 != nil {
			return nil, fmt.Errorf("classify: parse LLM JSON: %w", err)
		}
	}

	out := make(map[int64]*Classification, len(items))
	for _, c := range items {
		if c.ArticleID == 0 {
			continue
		}
		c.Category = sanitizeCategory(c.Category)
		if c.Importance < 0 || c.Importance > 3 {
			c.Importance = 1
		}
		c.Tags = dedupeStrings(c.Tags)
		out[c.ArticleID] = c
	}
	return out, nil
}

func sanitizeCategory(c string) string {
	c = strings.TrimSpace(strings.ToLower(c))
	for _, allowed := range Categories {
		if c == allowed {
			return allowed
		}
	}
	return "general"
}

func dedupeStrings(in []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(in))
	for _, s := range in {
		s = strings.TrimSpace(strings.ToLower(s))
		if s == "" || seen[s] {
			continue
		}
		seen[s] = true
		out = append(out, s)
	}
	return out
}
