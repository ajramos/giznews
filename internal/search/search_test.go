package search

import (
	"context"
	"hash/fnv"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ajramos/giznews/internal/db"
	"github.com/ajramos/giznews/internal/llm"
)

const dim = 16

// bagProvider produces a deterministic bag-of-words embedding (dim 16) so
// cosine similarity reflects token overlap.
type bagProvider struct{}

func (bagProvider) Name() string                   { return "bag" }
func (bagProvider) Ping(ctx context.Context) error { return nil }
func (bagProvider) Embed(ctx context.Context, req llm.EmbeddingRequest) (llm.EmbeddingResponse, error) {
	vec := make([]float32, dim)
	for _, tok := range strings.Fields(strings.ToLower(req.Input)) {
		h := fnv.New32a()
		h.Write([]byte(tok))
		vec[h.Sum32()%dim] += 1
	}
	return llm.EmbeddingResponse{Embedding: vec, Model: req.Model}, nil
}
func (bagProvider) Complete(ctx context.Context, req llm.CompletionRequest) (llm.CompletionResponse, error) {
	return llm.CompletionResponse{}, nil
}
func (bagProvider) StreamingComplete(ctx context.Context, req llm.CompletionRequest, onChunk func(string)) (llm.CompletionResponse, error) {
	return llm.CompletionResponse{}, nil
}

func newTestDB(t *testing.T) *db.DB {
	t.Helper()
	d, err := db.Open(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { d.Close() })
	return d
}

func seed(t *testing.T, d *db.DB) {
	t.Helper()
	ctx := context.Background()
	src, _ := db.NewSourceRepo(d).Create(ctx, db.NewSource{Name: "HN", Type: db.SourceHackerNews, URL: "u"})
	repo := db.NewArticleRepo(d)
	for i, title := range []string{
		"Local RAG models beat cloud providers",
		"Agentic workflows with tools",
		"Banana prices in Argentina",
	} {
		_, _, _ = repo.Upsert(ctx, db.NewArticle{SourceID: src.ID, GUID: "a" + string(rune('0'+i)), Title: title, ContentMD: "body about " + title, Status: db.StatusUnread})
	}
	kb := db.NewKBRepo(d)
	_, _ = kb.Create(ctx, db.NewKBNote{Type: db.NoteAtom, Title: "RAG patterns", Slug: "rag-patterns", Content: "retrieval augmented generation agents"})
	_, _ = kb.Create(ctx, db.NewKBNote{Type: db.NoteElectron, Title: "Agents", Slug: "agents", Content: "autonomous agent loops tools"})
}

func TestCosineAndRRF(t *testing.T) {
	a := []float32{1, 0, 1}
	b := []float32{1, 0, 1}
	if cosine(a, b) < 0.99 {
		t.Fatalf("identical cosine = %v", cosine(a, b))
	}
	c := []float32{0, 1, 0}
	if cosine(a, c) != 0 {
		t.Fatalf("orthogonal cosine = %v", cosine(a, c))
	}

	merged := rrfMerge(
		[]*ranked{{kind: "note", id: 1}, {kind: "note", id: 2}},
		[]*scored{{kind: "note", id: 2, sim: 0.9}, {kind: "note", id: 1, sim: 0.8}},
	)
	if len(merged) != 2 || merged[0].id != 1 {
		t.Fatalf("merged = %+v", merged)
	}
	// note 1 appears at pos 1 in both → highest RRF.
	if merged[0].score < merged[1].score {
		t.Fatalf("RRF ordering wrong: %+v", merged)
	}
}

func TestHybridSearch(t *testing.T) {
	d := newTestDB(t)
	seed(t, d)
	svc, err := NewService(d, bagProvider{}, Options{Model: "bag"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()

	if _, err := svc.Index(ctx); err != nil {
		t.Fatal(err)
	}

	res, err := svc.Search(ctx, "rag agents", 5)
	if err != nil {
		t.Fatal(err)
	}
	if len(res) == 0 {
		t.Fatal("expected results")
	}
	// The RAG/agents content should rank above the bananas article.
	top := res[0].Title
	if strings.Contains(strings.ToLower(top), "banana") {
		t.Fatalf("semantic rank wrong, top = %q (%+v)", top, res)
	}
	// Notes should appear in results.
	foundNote := false
	for _, r := range res {
		if r.Kind == "note" {
			foundNote = true
		}
	}
	if !foundNote {
		t.Fatalf("expected notes in results: %+v", res)
	}
}

func TestSearchKeywordFallback(t *testing.T) {
	d := newTestDB(t)
	seed(t, d)
	// No provider → keyword-only (but FTS must be indexed first).
	svc, err := NewService(d, nil, Options{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Index(context.Background()); err != nil {
		t.Fatal(err)
	}
	res, err := svc.Search(context.Background(), "banana", 5)
	if err != nil {
		t.Fatal(err)
	}
	if len(res) != 1 || res[0].Title != "Banana prices in Argentina" {
		t.Fatalf("res = %+v", res)
	}
}

func TestFTSQueryEscaping(t *testing.T) {
	q := ftsQuery(`retrieval "augmented" -generation *agents`)
	if !strings.Contains(q, `"retrieval augmented generation agents"`) {
		t.Fatalf("query = %q", q)
	}
}
