package search

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/ajramos/giznews/internal/db"
	"github.com/ajramos/giznews/internal/llm"
)

// Asking the knowledge base a question is the point of having built it. The
// retrieval half was already here — FTS5, embeddings, RRF — and returned a
// ranked list of rows, which is a search engine, not an answer. What was
// missing is the last step: read the notes back and say what they say.
//
// The rule the whole thing hangs on is that it may only say what the notes say.
// An answer from the model's own memory would be indistinguishable from one
// grounded in the vault, and far more confident than it deserves, so every
// citation is checked against the database before the answer is shown.

// AskOptions tunes one question.
type AskOptions struct {
	// Notes and Articles cap how much goes to the model. Notes come first:
	// they are what the reader wrote or the graph distilled, and they are the
	// only sources an answer can cite.
	Notes    int
	Articles int
	// Excerpt is how much of each source is quoted into the prompt.
	Excerpt  int
	Language string
	Model    string
}

func (o AskOptions) notes() int {
	if o.Notes > 0 {
		return o.Notes
	}
	return 8
}

func (o AskOptions) articles() int {
	if o.Articles > 0 {
		return o.Articles
	}
	return 4
}

func (o AskOptions) excerpt() int {
	if o.Excerpt > 0 {
		return o.Excerpt
	}
	return 900
}

// Answer is what the knowledge base has to say.
type Answer struct {
	Question string    `json:"question"`
	Text     string    `json:"text"`
	Sources  []*Result `json:"sources"`
	// Grounded is false when there is no answer to give — nothing was
	// retrieved, or no model is available to write one. The sources are still
	// returned: a ranked list is worth more than an apology.
	Grounded bool `json:"grounded"`
	// Dropped names citations the model invented. They are stripped from the
	// text before anyone sees it; keeping the list makes the failure visible
	// instead of silent.
	Dropped []string `json:"dropped,omitempty"`
}

// ErrNoQuestion is returned for an empty question.
var ErrNoQuestion = errors.New("ask: no question")

var citationRe = regexp.MustCompile(`\[\[([^\]|#]+)\]\]`)

// Ask answers a question from the notes, with citations to them.
func (s *Service) Ask(ctx context.Context, question string, opts AskOptions) (*Answer, error) {
	q := strings.TrimSpace(question)
	if q == "" {
		return nil, ErrNoQuestion
	}

	// An empty index answers every question with silence, which reads exactly
	// like an empty vault. Building it here means "nothing in your notes covers
	// that" is always the truth rather than a missing step.
	s.ensureIndexed(ctx)

	hits, err := s.retrieve(ctx, q, questionQuery(q), opts.notes()+opts.articles()+10)
	if err != nil {
		return nil, err
	}
	sources := prefer(hits, opts.notes(), opts.articles())
	answer := &Answer{Question: q, Sources: sources}
	if len(sources) == 0 {
		return answer, nil // nothing retrieved: say so rather than invent
	}
	if s.prov == nil {
		return answer, nil // no model: the ranked list is the answer
	}

	text, err := s.writeAnswer(ctx, q, sources, opts)
	if err != nil {
		if s.logger != nil {
			s.logger.Printf("ask: %v", err)
		}
		return answer, nil // the retrieval still stands on its own
	}
	text, dropped := s.checkCitations(ctx, text)
	if strings.TrimSpace(text) == "" {
		return answer, nil
	}
	answer.Text, answer.Dropped, answer.Grounded = text, dropped, true
	return answer, nil
}

// ensureIndexed builds the keyword index the first time somebody asks, when
// nobody has run `search index` yet.
func (s *Service) ensureIndexed(ctx context.Context) {
	notes, articles := s.countFTS()
	if notes > 0 || articles > 0 {
		return
	}
	if s.logger != nil {
		s.logger.Printf("ask: the search index was empty, building it")
	}
	if _, err := s.Index(ctx); err != nil && s.logger != nil {
		s.logger.Printf("ask: could not build the index: %v", err)
	}
}

// prefer keeps notes ahead of articles while holding the ranking inside each
// group: a note is the distilled version of what an article said, and it is the
// only kind of source an answer can point at.
func prefer(hits []*Result, maxNotes, maxArticles int) []*Result {
	out := make([]*Result, 0, maxNotes+maxArticles)
	for _, h := range hits {
		if h.Kind == "note" && len(out) < maxNotes {
			out = append(out, h)
		}
	}
	articles := 0
	for _, h := range hits {
		if h.Kind == "article" && articles < maxArticles {
			out = append(out, h)
			articles++
		}
	}
	return out
}

// writeAnswer asks the model to say what the sources say, and nothing else.
func (s *Service) writeAnswer(ctx context.Context, question string, sources []*Result, opts AskOptions) (string, error) {
	var b strings.Builder
	for i, src := range sources {
		body, err := s.fullText(ctx, src)
		if err != nil {
			continue
		}
		b.WriteString(fmt.Sprintf("--- source %d", i+1))
		if src.Slug != "" {
			b.WriteString(fmt.Sprintf(" · cite as [[%s]]", src.Slug))
		} else {
			b.WriteString(" · an article, not citable")
		}
		b.WriteString("\n" + src.Title + "\n" + excerptOf(body, opts.excerpt()) + "\n\n")
	}
	if b.Len() == 0 {
		return "", errors.New("nothing readable in the retrieved sources")
	}

	resp, err := s.prov.Complete(ctx, llm.CompletionRequest{
		Model: opts.Model,
		Messages: []llm.Message{
			{Role: llm.RoleSystem, Content: "You answer from a personal knowledge base, using only the sources given to you. " +
				"Write 2-5 sentences of plain prose. Cite the note a claim comes from inline, as [[slug]], using exactly the slugs offered — " +
				"never invent one, and never cite an article. " +
				"If the sources do not answer the question, say so in one sentence instead of answering from your own knowledge. " +
				"No preamble, no markdown headings, no bullet list." + llm.LanguageInstruction(opts.Language)},
			{Role: llm.RoleUser, Content: "Question: " + question + "\n\nSources:\n" + b.String()},
		},
		Temperature: 0.2,
	})
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(resp.Content), nil
}

// fullText reads a source back out of the database, since a snippet is too
// little to answer from.
func (s *Service) fullText(ctx context.Context, r *Result) (string, error) {
	if r.Kind == "note" {
		n, err := db.NewKBRepo(s.db).Get(ctx, r.ID)
		if err != nil {
			return "", err
		}
		return stripFrontmatter(n.Content), nil
	}
	a, err := db.NewArticleRepo(s.db).Get(ctx, r.ID)
	if err != nil {
		return "", err
	}
	if a.Summary != "" {
		return a.Summary, nil
	}
	return a.ContentMD, nil
}

// checkCitations removes any citation pointing at a note that does not exist.
//
// A made-up citation is worse than no citation: it looks exactly like a real
// one, and the whole promise of answering from the vault is that a claim can be
// followed back to the note it came from.
func (s *Service) checkCitations(ctx context.Context, text string) (string, []string) {
	repo := db.NewKBRepo(s.db)
	var dropped []string
	seen := map[string]bool{}

	out := citationRe.ReplaceAllStringFunc(text, func(match string) string {
		slug := strings.TrimSpace(citationRe.FindStringSubmatch(match)[1])
		if seen[slug] {
			return match // already checked and kept
		}
		if _, err := repo.GetBySlug(ctx, slug); err != nil {
			dropped = append(dropped, slug)
			if s.logger != nil {
				s.logger.Printf("ask: dropped a citation to %q, no such note", slug)
			}
			return slug // keep the words, lose the promise
		}
		seen[slug] = true
		return match
	})
	return out, dropped
}

func excerptOf(text string, max int) string {
	text = strings.TrimSpace(text)
	r := []rune(text)
	if len(r) <= max {
		return text
	}
	return string(r[:max]) + " …"
}

// questionWords are what a question is made of besides its subject. Left in a
// keyword query they match nothing and rank nothing.
var questionWords = map[string]bool{
	"what": true, "who": true, "when": true, "where": true, "why": true,
	"how": true, "which": true, "is": true, "are": true, "was": true,
	"were": true, "do": true, "does": true, "did": true, "the": true,
	"a": true, "an": true, "of": true, "in": true, "on": true, "to": true,
	"for": true, "about": true, "and": true, "or": true, "i": true,
	"know": true, "tell": true, "me": true, "my": true, "that": true,
	"this": true, "it": true, "with": true, "from": true, "can": true,
	"should": true, "would": true, "there": true, "any": true, "some": true,
}

// questionQuery turns a question into something FTS5 can rank.
//
// The search box sends a phrase, which is right for a search box: typing three
// words there means you expect those three words together. A question is the
// opposite — "what is sparse attention?" as a phrase matches only a note that
// literally asks it. So the question words come out and what remains is matched
// as alternatives, letting bm25 rank whatever contains most of them.
func questionQuery(question string) string {
	clean := strings.Map(func(r rune) rune {
		switch r {
		case '"', '\'', '(', ')', '*', '^', ':', '~', '+', '?', '!', ',', '.':
			return ' '
		}
		return r
	}, strings.ToLower(question))

	terms := make([]string, 0, 8)
	seen := map[string]bool{}
	for _, w := range strings.Fields(clean) {
		w = strings.Trim(w, "-")
		if len(w) < 2 || questionWords[w] || seen[w] {
			continue
		}
		seen[w] = true
		terms = append(terms, `"`+w+`"`)
	}
	if len(terms) == 0 {
		return ftsQuery(question) // nothing but question words: try it verbatim
	}
	return strings.Join(terms, " OR ")
}
