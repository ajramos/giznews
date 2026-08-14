// Package search provides hybrid semantic + keyword retrieval over the
// knowledge graph notes and articles.
//
// Strategy (explained to the user as FTS5 + RRF):
//   - FTS5 (SQLite's inverted index) ranks documents by keyword relevance.
//   - Local embeddings (nomic-embed-text via Ollama) rank them by cosine
//     similarity, computed by brute force over the stored vectors — fast at
//     personal scale (thousands of rows × 768 dims < 100 ms in Go).
//   - The two rankings are fused with Reciprocal Rank Fusion (RRF), which only
//     cares about positions, so keyword and semantic scores never need to be
//     normalized against each other.
package search

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"sort"
	"strings"

	"github.com/ajramos/giznews/internal/db"
	"github.com/ajramos/giznews/internal/llm"
)

// Options configures the search service.
type Options struct {
	Model string // embedding model, e.g. nomic-embed-text
}

// Result is one hit from a hybrid query.
type Result struct {
	Kind    string  `json:"kind"` // article | note
	ID      int64   `json:"id"`
	Title   string  `json:"title"`
	Source  string  `json:"source,omitempty"`
	Snippet string  `json:"snippet"`
	Score   float64 `json:"score"`
}

// IndexResult reports what an index run did.
type IndexResult struct {
	NotesEmbedded    int `json:"notes_embedded"`
	ArticlesEmbedded int `json:"articles_embedded"`
	FTSNotes         int `json:"fts_notes"`
	FTSArticles      int `json:"fts_articles"`
	EmbeddingsFailed int `json:"embeddings_failed"`
}

// Service runs indexing and hybrid search.
type Service struct {
	db     *db.DB
	prov   llm.Provider // nil → keyword-only search
	model  string
	logger *log.Logger
}

// NewService builds a search service, creating the FTS5 tables on first use.
func NewService(database *db.DB, prov llm.Provider, opts Options, logger *log.Logger) (*Service, error) {
	if opts.Model == "" {
		opts.Model = "nomic-embed-text"
	}
	s := &Service{db: database, prov: prov, model: opts.Model, logger: logger}
	if err := s.ensureFTS(); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *Service) ensureFTS() error {
	for _, stmt := range []string{
		`CREATE VIRTUAL TABLE IF NOT EXISTS notes_fts USING fts5(note_id, title, content, tags);`,
		`CREATE VIRTUAL TABLE IF NOT EXISTS articles_fts USING fts5(article_id, title, content, summary, tags);`,
	} {
		if _, err := s.db.SQL().Exec(stmt); err != nil {
			return fmt.Errorf("search: create fts: %w", err)
		}
	}
	return nil
}

// Index rebuilds the FTS5 indexes and computes embeddings for every note and
// article that lacks one. Cheap at personal scale; call after fetch/classify
// or on demand before searching.
func (s *Service) Index(ctx context.Context) (*IndexResult, error) {
	res := &IndexResult{}

	// --- FTS5 rebuild (cheap, always fresh). ---
	if err := s.rebuildFTS(ctx); err != nil {
		return nil, err
	}
	res.FTSNotes, res.FTSArticles = s.countFTS()

	// --- Embeddings for notes. ---
	if s.prov != nil {
		kbRepo := db.NewKBRepo(s.db)
		notes, err := kbRepo.List(ctx, "", 100000)
		if err != nil {
			return nil, err
		}
		for _, n := range notes {
			emb, _ := kbRepo.GetEmbedding(ctx, n.ID)
			if len(emb) > 0 {
				continue
			}
			vec, err := s.embed(ctx, embedText(n.Title, n.Content))
			if err != nil {
				res.EmbeddingsFailed++
				continue
			}
			if err := kbRepo.SetEmbedding(ctx, n.ID, vec); err != nil {
				return nil, err
			}
			res.NotesEmbedded++
		}

		arts, err := db.NewArticleRepo(s.db).ListRecent(ctx, 365, 100000)
		if err != nil {
			return nil, err
		}
		for _, a := range arts {
			emb, _ := db.NewArticleRepo(s.db).GetArticleEmbedding(ctx, a.ID)
			if len(emb) > 0 {
				continue
			}
			vec, err := s.embed(ctx, embedText(a.Title, a.ContentMD))
			if err != nil {
				res.EmbeddingsFailed++
				continue
			}
			if err := db.NewArticleRepo(s.db).SetArticleEmbedding(ctx, a.ID, vec); err != nil {
				return nil, err
			}
			res.ArticlesEmbedded++
		}
	}

	if s.logger != nil {
		s.logger.Printf("search index: %d notes, %d articles embedded", res.NotesEmbedded, res.ArticlesEmbedded)
	}
	return res, nil
}

func (s *Service) rebuildFTS(ctx context.Context) error {
	sql := s.db.SQL()

	// Materialize before inserting: with MaxOpenConns(1), an open SELECT holds
	// the only connection and INSERTs would deadlock.
	type noteRow struct {
		id      int64
		title   string
		content string
		tags    string
	}
	notes := []noteRow{}
	rows, err := sql.Query(`SELECT id, title, content, tags FROM kb_notes`)
	if err != nil {
		return err
	}
	for rows.Next() {
		var r noteRow
		if err := rows.Scan(&r.id, &r.title, &r.content, &r.tags); err != nil {
			rows.Close()
			return err
		}
		notes = append(notes, r)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}

	type artRow struct {
		id      int64
		title   string
		content string
		summary string
		tags    string
	}
	articles := []artRow{}
	arows, err := sql.Query(`SELECT id, title, content_md, summary, tags FROM articles`)
	if err != nil {
		return err
	}
	for arows.Next() {
		var r artRow
		if err := arows.Scan(&r.id, &r.title, &r.content, &r.summary, &r.tags); err != nil {
			arows.Close()
			return err
		}
		articles = append(articles, r)
	}
	arows.Close()
	if err := arows.Err(); err != nil {
		return err
	}

	// Rebuild both FTS tables.
	if _, err := sql.Exec("DELETE FROM notes_fts;"); err != nil {
		return err
	}
	for _, r := range notes {
		if _, err := sql.Exec("INSERT INTO notes_fts (note_id, title, content, tags) VALUES (?,?,?,?)", r.id, r.title, r.content, r.tags); err != nil {
			return err
		}
	}

	if _, err := sql.Exec("DELETE FROM articles_fts;"); err != nil {
		return err
	}
	for _, r := range articles {
		if _, err := sql.Exec("INSERT INTO articles_fts (article_id, title, content, summary, tags) VALUES (?,?,?,?,?)", r.id, r.title, r.content, r.summary, r.tags); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) countFTS() (notes, articles int) {
	sql := s.db.SQL()
	_ = sql.QueryRow("SELECT COUNT(*) FROM notes_fts").Scan(&notes)
	_ = sql.QueryRow("SELECT COUNT(*) FROM articles_fts").Scan(&articles)
	return
}

// Search runs hybrid retrieval. Without an embedding provider it degrades to
// keyword-only FTS5.
func (s *Service) Search(ctx context.Context, query string, limit int) ([]*Result, error) {
	if limit <= 0 {
		limit = 20
	}
	q := strings.TrimSpace(query)
	if q == "" {
		return nil, nil
	}

	// 1. Keyword ranking.
	ftsRanks := s.ftsRanked(ctx, q)

	// 2. Semantic ranking.
	var vecRanks []*scored
	if s.prov != nil {
		qv, err := s.embed(ctx, q)
		if err == nil {
			vecRanks = s.vectorRanked(ctx, qv)
		}
	}

	// 3. Merge with RRF.
	merged := rrfMerge(ftsRanks, vecRanks)

	// 4. Hydrate snippets.
	out := make([]*Result, 0, limit)
	for _, m := range merged {
		if len(out) >= limit {
			break
		}
		r, err := s.hydrate(ctx, m.kind, m.id)
		if err != nil {
			continue
		}
		r.Score = m.score
		out = append(out, r)
	}
	return out, nil
}

func (s *Service) ftsRanked(ctx context.Context, q string) []*ranked {
	var out []*ranked
	rows, err := s.db.SQL().Query(
		`SELECT note_id, title, bm25(notes_fts) AS rank FROM notes_fts WHERE notes_fts MATCH ? ORDER BY rank LIMIT 200`, ftsQuery(q))
	if err == nil {
		for rows.Next() {
			var id int64
			var title string
			var rank float64
			if rows.Scan(&id, &title, &rank) == nil {
				out = append(out, &ranked{kind: "note", id: id, title: title})
			}
		}
		rows.Close()
	}
	arows, err := s.db.SQL().Query(
		`SELECT article_id, title, bm25(articles_fts) AS rank FROM articles_fts WHERE articles_fts MATCH ? ORDER BY rank LIMIT 200`, ftsQuery(q))
	if err == nil {
		for arows.Next() {
			var id int64
			var title string
			var rank float64
			if arows.Scan(&id, &title, &rank) == nil {
				out = append(out, &ranked{kind: "article", id: id, title: title})
			}
		}
		arows.Close()
	}
	return out
}

// ftsQuery escapes the user query into an FTS5 match expression. Falls back to
// a quoted phrase when the raw query is not valid FTS syntax.
func ftsQuery(q string) string {
	clean := strings.Map(func(r rune) rune {
		switch r {
		case '"', '\'', '(', ')', '*', '^', ':', '-', '~', '+':
			return ' '
		}
		return r
	}, q)
	return `"` + strings.Join(strings.Fields(clean), " ") + `"`
}

type vectorRow struct {
	id  int64
	vec []float32
}

func (s *Service) vectorRanked(ctx context.Context, qv []float32) []*scored {
	rows, err := s.db.SQL().Query(`SELECT id, title, embedding FROM kb_notes WHERE embedding IS NOT NULL`)
	if err != nil {
		return nil
	}
	defer rows.Close()
	var out []*scored
	for rows.Next() {
		var id int64
		var title string
		var blob []byte
		if rows.Scan(&id, &title, &blob) != nil {
			continue
		}
		vec, err := unmarshalVec(blob)
		if err != nil || len(vec) == 0 {
			continue
		}
		out = append(out, &scored{kind: "note", id: id, title: title, sim: cosine(qv, vec)})
	}
	_ = rows.Err()

	arows, err := s.db.SQL().Query(`SELECT id, title, embedding FROM articles WHERE embedding IS NOT NULL`)
	if err != nil {
		return out
	}
	defer arows.Close()
	for arows.Next() {
		var id int64
		var title string
		var blob []byte
		if arows.Scan(&id, &title, &blob) != nil {
			continue
		}
		vec, err := unmarshalVec(blob)
		if err != nil || len(vec) == 0 {
			continue
		}
		out = append(out, &scored{kind: "article", id: id, title: title, sim: cosine(qv, vec)})
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].sim > out[j].sim })
	return out
}

func (s *Service) hydrate(ctx context.Context, kind string, id int64) (*Result, error) {
	if kind == "note" {
		n, err := db.NewKBRepo(s.db).Get(ctx, id)
		if err != nil {
			return nil, err
		}
		return &Result{Kind: "note", ID: n.ID, Title: n.Title, Source: string(n.Type), Snippet: snippet(stripFrontmatter(n.Content))}, nil
	}
	a, err := db.NewArticleRepo(s.db).Get(ctx, id)
	if err != nil {
		return nil, err
	}
	src := a.Summary
	if src == "" {
		src = a.ContentMD
	}
	return &Result{Kind: "article", ID: a.ID, Title: a.Title, Source: a.SourceName, Snippet: snippet(src)}, nil
}

func (s *Service) embed(ctx context.Context, text string) ([]float32, error) {
	resp, err := s.prov.Embed(ctx, llm.EmbeddingRequest{Model: s.model, Input: text})
	if err != nil {
		return nil, fmt.Errorf("search: embed: %w", err)
	}
	return resp.Embedding, nil
}

// --- scoring helpers ---

type ranked struct {
	kind  string
	id    int64
	title string
}

type scored struct {
	kind  string
	id    int64
	title string
	sim   float64
}

// mergedDoc is the fusion output keyed by (kind, id).
type mergedDoc struct {
	kind  string
	id    int64
	score float64
}

const rrfK = 60.0

// rrfMerge fuses ranked lists by position (Reciprocal Rank Fusion).
func rrfMerge(fts []*ranked, vec []*scored) []*mergedDoc {
	scores := map[[2]any]float64{} // key: kind+id → score
	order := []string{}
	keys := map[[2]any]bool{}

	add := func(key [2]any, pos int) {
		if !keys[key] {
			keys[key] = true
			order = append(order, key[0].(string)+"#"+fmt.Sprint(key[1]))
		}
		scores[key] += 1.0 / (rrfK + float64(pos))
	}

	for i, r := range fts {
		add([2]any{r.kind, r.id}, i+1)
	}
	for i, s := range vec {
		add([2]any{s.kind, s.id}, i+1)
	}

	out := make([]*mergedDoc, 0, len(order))
	for _, k := range order {
		// parse back kind#id
		var kind string
		var id int64
		if idx := strings.IndexByte(k, '#'); idx >= 0 {
			kind = k[:idx]
			_, _ = fmt.Sscanf(k[idx+1:], "%d", &id)
		}
		out = append(out, &mergedDoc{kind: kind, id: id, score: scores[[2]any{kind, id}]})
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].score > out[j].score })
	return out
}

func cosine(a, b []float32) float64 {
	if len(a) == 0 || len(b) == 0 || len(a) != len(b) {
		return 0
	}
	var dot, na, nb float64
	for i := range a {
		dot += float64(a[i]) * float64(b[i])
		na += float64(a[i]) * float64(a[i])
		nb += float64(b[i]) * float64(b[i])
	}
	if na == 0 || nb == 0 {
		return 0
	}
	return dot / (math.Sqrt(na) * math.Sqrt(nb))
}

func embedText(title, content string) string {
	t := strings.TrimSpace(title)
	c := strings.TrimSpace(content)
	if len(c) > 1200 {
		c = c[:1200]
	}
	if t == "" {
		return c
	}
	return t + "\n\n" + c
}

// stripFrontmatter removes the leading YAML frontmatter block for snippets.
func stripFrontmatter(s string) string {
	if !strings.HasPrefix(s, "---\n") {
		return s
	}
	if i := strings.Index(s[4:], "\n---"); i >= 0 {
		return s[4+i+4:]
	}
	return s
}

func snippet(s string) string {
	s = strings.Join(strings.Fields(s), " ")
	r := []rune(s)
	if len(r) <= 220 {
		return string(r)
	}
	return string(r[:220]) + " …"
}

func unmarshalVec(b []byte) ([]float32, error) {
	var out []float32
	if err := json.Unmarshal(b, &out); err != nil {
		return nil, err
	}
	return out, nil
}
