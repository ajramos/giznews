package db

import (
	"context"
	"testing"
)

func TestDigestRepoRoundTrip(t *testing.T) {
	d := openTestDB(t)
	repo := NewDigestRepo(d)
	ctx := context.Background()

	if err := repo.Save(ctx, "2026-08-15", "Big week.", `[{"theme":"models"}]`); err != nil {
		t.Fatal(err)
	}
	// Upsert by date: a second save overwrites, no duplicate row.
	if err := repo.Save(ctx, "2026-08-15", "Bigger week.", `[{"theme":"models"},{"theme":"regulation"}]`); err != nil {
		t.Fatal(err)
	}
	if err := repo.Save(ctx, "2026-08-14", "Yesterday.", `[{"theme":"tools"}]`); err != nil {
		t.Fatal(err)
	}

	got, err := repo.Get(ctx, "2026-08-15")
	if err != nil {
		t.Fatal(err)
	}
	if got.Overview != "Bigger week." || got.Themes != `[{"theme":"models"},{"theme":"regulation"}]` {
		t.Fatalf("got = %+v", got)
	}

	list, err := repo.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 2 || list[0].Date != "2026-08-15" {
		t.Fatalf("list = %+v", list)
	}

	if _, err := repo.Get(ctx, "nope"); err != ErrNotFound {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}
