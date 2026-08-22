package search

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ajramos/giznews/internal/db"
	"github.com/ajramos/giznews/internal/llm"
)

// answerProvider replies with whatever a test needs the model to say.
type answerProvider struct {
	reply  string
	prompt string
}

func (p *answerProvider) Name() string                   { return "fake" }
func (p *answerProvider) Ping(ctx context.Context) error { return nil }
func (p *answerProvider) Embed(context.Context, llm.EmbeddingRequest) (llm.EmbeddingResponse, error) {
	return llm.EmbeddingResponse{}, nil
}
func (p *answerProvider) StreamingComplete(ctx context.Context, req llm.CompletionRequest, _ func(string)) (llm.CompletionResponse, error) {
	return p.Complete(ctx, req)
}
func (p *answerProvider) Complete(_ context.Context, req llm.CompletionRequest) (llm.CompletionResponse, error) {
	for _, m := range req.Messages {
		if m.Role == llm.RoleUser {
			p.prompt = m.Content
		}
	}
	return llm.CompletionResponse{Content: p.reply}, nil
}

// askFixture seeds a vault with two notes and an article about one subject.
func askFixture(t *testing.T, prov llm.Provider) (*Service, *db.DB) {
	t.Helper()
	d, err := db.Open(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { d.Close() })
	ctx := context.Background()

	kbRepo := db.NewKBRepo(d)
	if _, err := kbRepo.Create(ctx, db.NewKBNote{
		Type: db.NoteElectron, Title: "Sparse Attention", Slug: "sparse-attention", Path: "p1",
		Content: "---\ntype: electron\n---\n# Sparse Attention\n\nAttending to a subset of tokens keeps long context affordable.",
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := kbRepo.Create(ctx, db.NewKBNote{
		Type: db.NoteAtom, Title: "Sparse attention at scale", Slug: "sparse-attention-at-scale", Path: "p2",
		Content: "---\ntype: atom\n---\n# Sparse attention at scale\n\nBenchmarks show sparse attention holding quality at long context.",
	}); err != nil {
		t.Fatal(err)
	}
	src, _ := db.NewSourceRepo(d).Create(ctx, db.NewSource{Name: "Feed", Type: db.SourceRSS, URL: "u"})
	if _, _, err := db.NewArticleRepo(d).Upsert(ctx, db.NewArticle{
		SourceID: src.ID, GUID: "a1", URL: "https://x/1", Title: "Sparse attention paper lands",
		Summary: "A paper on sparse attention.",
	}); err != nil {
		t.Fatal(err)
	}

	svc, err := NewService(d, prov, Options{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Index(ctx); err != nil {
		t.Fatal(err)
	}
	return svc, d
}

// The promise of answering from the vault is that a claim can be followed back
// to the note it came from. A citation to a note that does not exist looks
// exactly like a real one, so it never reaches the reader.
func TestInventedCitationsAreDropped(t *testing.T) {
	prov := &answerProvider{
		reply: "Sparse attention keeps long context affordable [[sparse-attention]], " +
			"and it was proven in production [[imaginary-note]].",
	}
	svc, _ := askFixture(t, prov)

	answer, err := svc.Ask(context.Background(), "what is sparse attention?", AskOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if !answer.Grounded {
		t.Fatalf("expected an answer: %+v", answer)
	}
	if !strings.Contains(answer.Text, "[[sparse-attention]]") {
		t.Fatalf("the real citation was lost:\n%s", answer.Text)
	}
	if strings.Contains(answer.Text, "[[imaginary-note]]") {
		t.Fatalf("an invented citation reached the reader:\n%s", answer.Text)
	}
	if !strings.Contains(answer.Text, "imaginary-note") {
		t.Fatalf("the words should stay, only the promise goes:\n%s", answer.Text)
	}
	if len(answer.Dropped) != 1 || answer.Dropped[0] != "imaginary-note" {
		t.Fatalf("dropped = %v, want the invented slug reported", answer.Dropped)
	}
}

// Notes are what an answer is built from; articles may support it but can never
// be cited, so they come last and only if there is room.
func TestNotesComeFirstAndArticlesCannotBeCited(t *testing.T) {
	prov := &answerProvider{reply: "It keeps long context affordable [[sparse-attention]]."}
	svc, _ := askFixture(t, prov)

	answer, err := svc.Ask(context.Background(), "sparse attention", AskOptions{Notes: 2, Articles: 1})
	if err != nil {
		t.Fatal(err)
	}
	if len(answer.Sources) < 2 {
		t.Fatalf("sources = %+v", answer.Sources)
	}
	if answer.Sources[0].Kind != "note" {
		t.Fatalf("first source is a %s, notes must come first", answer.Sources[0].Kind)
	}
	if answer.Sources[0].Slug == "" {
		t.Fatal("a note source must carry the slug a citation points at")
	}
	// The prompt has to tell the model which sources it may cite, and which it
	// may not.
	if !strings.Contains(prov.prompt, "cite as [[sparse-attention]]") {
		t.Fatalf("the prompt never offered the slug:\n%s", prov.prompt)
	}
	if strings.Contains(prov.prompt, "Sparse attention paper lands\n") &&
		!strings.Contains(prov.prompt, "an article, not citable") {
		t.Fatalf("an article was offered as citable:\n%s", prov.prompt)
	}
}

// With nothing retrieved there is nothing to say, and with no model there is
// nobody to say it. Both return the ranking instead of an invented answer.
func TestUngroundedAnswersRefuseToInvent(t *testing.T) {
	svc, _ := askFixture(t, &answerProvider{reply: "Something confident."})
	ctx := context.Background()

	empty, err := svc.Ask(ctx, "zzzz-nothing-in-this-vault-zzzz", AskOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if empty.Grounded || empty.Text != "" {
		t.Fatalf("answered a question the vault knows nothing about: %+v", empty)
	}

	noModel, _ := askFixture(t, nil)
	res, err := noModel.Ask(ctx, "sparse attention", AskOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if res.Grounded || res.Text != "" {
		t.Fatalf("answered without a model: %+v", res)
	}
	if len(res.Sources) == 0 {
		t.Fatal("the ranked list should still come back — it is worth more than an apology")
	}

	if _, err := svc.Ask(ctx, "   ", AskOptions{}); err != ErrNoQuestion {
		t.Fatalf("empty question error = %v", err)
	}
}

// A question is not a phrase. The search box sends one and gets a phrase match,
// which is right there and useless here: "what is sparse attention?" as a
// phrase matches only a note that literally asks it.
func TestAQuestionRetrievesByItsNouns(t *testing.T) {
	prov := &answerProvider{reply: "It keeps long context affordable [[sparse-attention]]."}
	svc, _ := askFixture(t, prov)
	ctx := context.Background()

	// The old path finds nothing for a natural question…
	phrase, err := svc.Search(ctx, "what is sparse attention?", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(phrase) != 0 {
		t.Logf("phrase search happened to match %d — the point stands for longer questions", len(phrase))
	}
	// …while asking does.
	answer, err := svc.Ask(ctx, "what is sparse attention?", AskOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if !answer.Grounded || len(answer.Sources) == 0 {
		t.Fatalf("a plain question found nothing: %+v", answer)
	}

	if got := questionQuery("What is sparse attention?"); got != `"sparse" OR "attention"` {
		t.Fatalf("questionQuery = %q", got)
	}
	if got := questionQuery("what is it?"); got == "" {
		t.Fatal("a question with nothing but question words should still try something")
	}
}
