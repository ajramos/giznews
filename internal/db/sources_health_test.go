package db

import (
	"context"
	"strings"
	"testing"
)

// A failed fetch stores the error and increments the streak; a successful one
// clears both, and a feed that keeps answering but brings in nothing is flagged
// only once it has been empty enough times.
func TestSourceHealthCounters(t *testing.T) {
	d := openTestDB(t)
	ctx := context.Background()
	repo := NewSourceRepo(d)

	s, err := repo.Create(ctx, NewSource{Name: "S", Type: SourceRSS, URL: "u", Enabled: true})
	if err != nil {
		t.Fatal(err)
	}

	// A healthy run clears everything and records when it last worked.
	if err := repo.MarkSourceOK(ctx, s.ID); err != nil {
		t.Fatal(err)
	}
	after, _ := repo.Get(ctx, s.ID)
	if after.LastOK == "" || after.LastError != "" || after.ConsecutiveFailures != 0 || after.EmptyCycles != 0 {
		t.Fatalf("after ok = %+v", after)
	}

	// Failures accumulate, and each returns the running streak.
	n, err := repo.MarkSourceFailure(ctx, s.ID, "boom")
	if err != nil || n != 1 {
		t.Fatalf("first failure = %d, %v", n, err)
	}
	n, _ = repo.MarkSourceFailure(ctx, s.ID, "boom")
	if n != 2 {
		t.Fatalf("second failure = %d, want 2", n)
	}
	after, _ = repo.Get(ctx, s.ID)
	if after.ConsecutiveFailures != 2 || after.LastError != "boom" {
		t.Fatalf("after failures = %+v", after)
	}

	// A success stops the streak.
	if err := repo.MarkSourceOK(ctx, s.ID); err != nil {
		t.Fatal(err)
	}
	after, _ = repo.Get(ctx, s.ID)
	if after.ConsecutiveFailures != 0 || after.LastError != "" {
		t.Fatalf("after recovery = %+v", after)
	}

	// Empty cycles are counted separately and flagged once they reach the
	// threshold.
	if _, err := repo.MarkSourceEmpty(ctx, s.ID, 3); err != nil {
		t.Fatal(err)
	}
	if n, _ := repo.MarkSourceEmpty(ctx, s.ID, 3); n != 2 {
		t.Fatalf("empty after two = %d, want 2", n)
	}
	if n, _ := repo.MarkSourceEmpty(ctx, s.ID, 3); n != 3 {
		t.Fatalf("empty after three = %d, want 3", n)
	}
	after, _ = repo.Get(ctx, s.ID)
	if after.EmptyCycles != 3 || !strings.Contains(after.LastError, "no items") {
		t.Fatalf("suspect source = %+v", after)
	}

	// An empty cycle is not an HTTP failure.
	if after.ConsecutiveFailures != 0 {
		t.Fatalf("empty cycles leaked into failures: %+v", after)
	}
}
