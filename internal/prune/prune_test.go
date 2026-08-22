package prune

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ajramos/giznews/internal/db"
)

// seed builds an archive with one article of every kind that matters.
func seed(t *testing.T) (*db.DB, map[string]int64) {
	t.Helper()
	d, err := db.Open(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { d.Close() })
	ctx := context.Background()

	src, err := db.NewSourceRepo(d).Create(ctx, db.NewSource{Name: "S", Type: db.SourceRSS, URL: "u"})
	if err != nil {
		t.Fatal(err)
	}
	repo := db.NewArticleRepo(d)
	ids := map[string]int64{}
	old := time.Now().UTC().AddDate(0, 0, -300).Format(time.RFC3339)
	ancient := time.Now().UTC().AddDate(0, 0, -500).Format(time.RFC3339)

	add := func(name, fetched string, body string) int64 {
		id, _, err := repo.Upsert(ctx, db.NewArticle{
			SourceID: src.ID, GUID: name, URL: "https://x/" + name, Title: name,
			ContentMD: body, ContentHTML: "<p>" + body + "</p>", Status: db.StatusRead,
		})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := d.SQL().ExecContext(ctx,
			"UPDATE articles SET fetched_at = ? WHERE id = ?", fetched, id); err != nil {
			t.Fatal(err)
		}
		ids[name] = id
		return id
	}

	body := strings.Repeat("the extracted body of an article. ", 40)
	add("old-read", old, body)
	add("ancient-read", ancient, body)
	add("recent", time.Now().UTC().Format(time.RFC3339), body)

	starred := add("old-starred", old, body)
	if err := repo.SetStarred(ctx, starred, true); err != nil {
		t.Fatal(err)
	}
	unread := add("old-unread", old, body)
	if err := repo.SetStatus(ctx, unread, db.StatusUnread, db.ActorUser); err != nil {
		t.Fatal(err)
	}
	noted := add("old-with-note", old, body)
	note, err := db.NewKBRepo(d).Create(ctx, db.NewKBNote{
		Type: db.NoteAtom, Title: "A note", Slug: "a-note", Path: "/tmp/a-note.md", Content: "# A note",
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.NewIngestRepo(d).Record(ctx, "article", fmt.Sprintf("%d", noted), note.ID, "processed"); err != nil {
		t.Fatal(err)
	}
	return d, ids
}

// The rules that make pruning safe enough to run unattended.
func TestPruneNeverTouchesWhatMatters(t *testing.T) {
	d, ids := seed(t)
	ctx := context.Background()

	plan, err := Preview(ctx, d, Options{BodyDays: 180, RowDays: 400})
	if err != nil {
		t.Fatal(err)
	}
	if plan.KeptStarred != 1 || plan.KeptUnread != 1 || plan.KeptInVault != 1 {
		t.Fatalf("kept = %+v", plan)
	}

	if _, err := Apply(ctx, d, Options{BodyDays: 180, RowDays: 400}, nil); err != nil {
		t.Fatal(err)
	}

	repo := db.NewArticleRepo(d)
	for _, name := range []string{"old-starred", "old-unread", "old-with-note", "recent"} {
		a, err := repo.Get(ctx, ids[name])
		if err != nil {
			t.Fatalf("%s was deleted: %v", name, err)
		}
		if a.ContentMD == "" {
			t.Fatalf("%s lost its body", name)
		}
	}

	// The read one past the window keeps its row and its classification, and
	// loses only the text.
	stripped, err := repo.Get(ctx, ids["old-read"])
	if err != nil {
		t.Fatalf("old-read should keep its row: %v", err)
	}
	if stripped.ContentMD != "" || stripped.ContentHTML != "" {
		t.Fatalf("old-read kept its body: %d bytes", len(stripped.ContentMD))
	}
	if stripped.Title == "" {
		t.Fatal("old-read lost more than its body")
	}

	// And the one past the second window is gone entirely.
	if _, err := repo.Get(ctx, ids["ancient-read"]); err == nil {
		t.Fatal("ancient-read should have been deleted")
	}
}

// A pruned body must not be fetched again on the next run: that would undo the
// prune and re-download a year of news to do it.
func TestAPrunedBodyIsNotExtractedAgain(t *testing.T) {
	d, ids := seed(t)
	ctx := context.Background()
	if _, err := Apply(ctx, d, Options{BodyDays: 180, RowDays: 400}, nil); err != nil {
		t.Fatal(err)
	}
	pending, err := db.NewArticleRepo(d).ListPendingExtract(ctx, 50)
	if err != nil {
		t.Fatal(err)
	}
	for _, a := range pending {
		if a.ID == ids["old-read"] {
			t.Fatal("a pruned article was queued for extraction again")
		}
	}
}

// A preview must describe the run that follows it, and touch nothing itself.
func TestPreviewWritesNothing(t *testing.T) {
	d, ids := seed(t)
	ctx := context.Background()

	plan, err := Preview(ctx, d, Options{BodyDays: 180, RowDays: 400})
	if err != nil {
		t.Fatal(err)
	}
	if plan.Bodies != 1 || plan.Rows != 1 {
		t.Fatalf("plan = %+v, want one body and one row", plan)
	}
	if plan.BodyBytes <= 0 {
		t.Fatalf("plan promised no bytes: %+v", plan)
	}
	before, err := db.NewArticleRepo(d).Get(ctx, ids["old-read"])
	if err != nil || before.ContentMD == "" {
		t.Fatal("the preview pruned something")
	}

	res, err := Apply(ctx, d, Options{BodyDays: 180, RowDays: 400}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if res.Bodies != plan.Bodies || res.Rows != plan.Rows {
		t.Fatalf("the run did not match its plan: %+v vs %+v", res.Plan, plan)
	}
}

// Copies of one story are pruned together. Deleting the copy the story is
// filed under, while its members stay, would strand them: they point at a row
// that is gone, and nothing lists them ever again.
func TestAStoryIsPrunedAsAUnit(t *testing.T) {
	d, err := db.Open(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	ctx := context.Background()

	src, _ := db.NewSourceRepo(d).Create(ctx, db.NewSource{Name: "S", Type: db.SourceRSS, URL: "u"})
	repo := db.NewArticleRepo(d)
	old := time.Now().UTC().AddDate(0, 0, -500).Format(time.RFC3339)

	var anchor int64
	var members []int64
	for i := 0; i < 3; i++ {
		id, _, err := repo.Upsert(ctx, db.NewArticle{
			SourceID: src.ID, GUID: fmt.Sprintf("g%d", i), URL: fmt.Sprintf("https://x/%d", i),
			Title: "One story, three outlets", ContentMD: "body", Status: db.StatusRead,
		})
		if err != nil {
			t.Fatal(err)
		}
		if i == 0 {
			anchor = id
		} else {
			if err := repo.JoinStory(ctx, id, anchor); err != nil {
				t.Fatal(err)
			}
		}
		members = append(members, id)
		if _, err := d.SQL().ExecContext(ctx, "UPDATE articles SET fetched_at = ? WHERE id = ?", old, id); err != nil {
			t.Fatal(err)
		}
	}
	// One copy is starred, which holds the whole story.
	if err := repo.SetStarred(ctx, members[2], true); err != nil {
		t.Fatal(err)
	}

	if _, err := Apply(ctx, d, Options{BodyDays: 180, RowDays: 400}, nil); err != nil {
		t.Fatal(err)
	}
	for _, id := range members {
		if _, err := repo.Get(ctx, id); err != nil {
			t.Fatalf("copy #%d went while a starred copy of its story stayed: %v", id, err)
		}
	}
}
