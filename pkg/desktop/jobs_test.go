package desktop

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/ajramos/giznews/internal/db"
)

func TestJobManagerLifecycle(t *testing.T) {
	m := NewJobManager()

	id, ctx, cancel := m.Begin(context.Background(), "Test job", "test")
	if id != 1 {
		t.Fatalf("first job id = %d, want 1", id)
	}
	m.Progress(id, "phase", 1, 3)
	m.Finish(id, nil)

	jobs := m.List()
	if len(jobs) != 1 {
		t.Fatalf("len(jobs) = %d, want 1", len(jobs))
	}
	if jobs[0].Status != JobDone || jobs[0].Phase != "phase" || jobs[0].Done != 1 || jobs[0].Total != 3 {
		t.Fatalf("job = %+v", jobs[0])
	}
	_ = ctx
	_ = cancel
}

func TestJobManagerCancel(t *testing.T) {
	m := NewJobManager()

	release := make(chan struct{})
	id, ctx, _ := m.Begin(context.Background(), "Long job", "test")
	done := make(chan error, 1)
	go func() {
		select {
		case <-ctx.Done():
			done <- ctx.Err()
		case <-release:
			done <- nil
		}
	}()

	m.Cancel(id)
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("err = %v, want context.Canceled", err)
		}
	case <-time.After(time.Second):
		t.Fatal("cancel did not propagate")
	}
	close(release)
}

func TestJobManagerRemoveAndClearFinished(t *testing.T) {
	m := NewJobManager()

	id1, _, _ := m.Begin(context.Background(), "A", "t")
	id2, _, _ := m.Begin(context.Background(), "B", "t")
	m.Finish(id1, nil) // done
	// id2 stays running

	m.Remove(id1)
	if got := m.List(); len(got) != 1 || got[0].ID != id2 {
		t.Fatalf("after remove, jobs = %+v", got)
	}

	m.ClearFinished() // only drops non-running (id1 already removed; id2 running stays)
	if got := m.List(); len(got) != 1 || got[0].ID != id2 {
		t.Fatalf("after clear, jobs = %+v", got)
	}
}

func TestBulkSetStatus(t *testing.T) {
	app := newTestApp(t)
	ctx := context.Background()

	s, err := app.AddSource(ctx, "Src", "rss", "https://x.example/rss", "")
	if err != nil {
		t.Fatal(err)
	}
	var ids []int64
	repo := db.NewArticleRepo(app.db)
	for i := 0; i < 5; i++ {
		id, _, err := repo.Upsert(ctx, db.NewArticle{
			SourceID: s.ID, GUID: string(rune('a' + i)), URL: "https://x.example/" + string(rune('a'+i)), Title: "T", Status: db.StatusUnread,
		})
		if err != nil {
			t.Fatal(err)
		}
		ids = append(ids, id)
	}

	res, err := app.BulkSetStatus(ctx, ids, "archived")
	if err != nil {
		t.Fatal(err)
	}
	if res.Updated != 5 || res.Total != 5 {
		t.Fatalf("res = %+v", res)
	}
	for _, id := range ids {
		a, err := repo.Get(ctx, id)
		if err != nil {
			t.Fatal(err)
		}
		if a.Status != db.StatusArchived {
			t.Fatalf("article %d status = %s, want archived", id, a.Status)
		}
	}

	// invalid status is rejected
	if _, err := app.BulkSetStatus(ctx, ids, "bogus"); err == nil {
		t.Fatal("expected error for invalid status")
	}
}

func TestEnsureManualSource(t *testing.T) {
	app := newTestApp(t)
	ctx := context.Background()

	s, err := app.ensureManualSource(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if s.Name != manualSourceName || s.Type != db.SourceManual {
		t.Fatalf("source = %+v", s)
	}
	// idempotent: a second call returns the same source
	s2, err := app.ensureManualSource(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if s2.ID != s.ID {
		t.Fatalf("second call returned different id: %d vs %d", s2.ID, s.ID)
	}
	// hidden sources do not appear in the picker list
	all, err := app.ListSources(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for _, x := range all {
		if x.ID == s.ID {
			t.Fatal("manual source should be hidden from ListSources")
		}
	}
}
