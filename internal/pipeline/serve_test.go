package pipeline

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/ajramos/giznews/internal/db"
)

func TestStageDue(t *testing.T) {
	now := time.Date(2026, 8, 21, 9, 30, 0, 0, time.Local)

	// A stage that has never run is due at once: starting the loop should bring
	// the feed up to date, not wait an hour to begin.
	fresh := &Stage{Name: "fetch", Every: time.Hour}
	if !fresh.Due(now) {
		t.Fatal("a stage that never ran must be due")
	}
	fresh.Ran(now.Add(-30 * time.Minute))
	if fresh.Due(now) {
		t.Fatal("it ran half an hour ago and runs hourly")
	}
	fresh.Ran(now.Add(-90 * time.Minute))
	if !fresh.Due(now) {
		t.Fatal("it is overdue")
	}

	// A stage with no cadence is off, and so is one with an unreadable time:
	// a typo must never become a stage running every nanosecond.
	if (&Stage{Name: "off"}).Due(now) {
		t.Fatal("a stage with no cadence must stay off")
	}
	if (&Stage{Name: "digest", At: "half past eight"}).Due(now) {
		t.Fatal("an unparseable time must switch the stage off, not crash it")
	}

	// A daily stage waits for its hour, then goes once.
	daily := &Stage{Name: "digest", At: "08:00"}
	if !daily.Due(now) {
		t.Fatal("08:00 has passed and it has not run today")
	}
	daily.Ran(now)
	if daily.Due(now.Add(2 * time.Hour)) {
		t.Fatal("it already ran today")
	}
	if !daily.Due(now.Add(24 * time.Hour)) {
		t.Fatal("tomorrow it is due again")
	}
	early := &Stage{Name: "digest", At: "23:30"}
	if early.Due(now) {
		t.Fatal("23:30 has not come round yet")
	}
}

// The loop's promise: a stage that fails does not take the others with it.
func TestOneFailingStageDoesNotStopTheRest(t *testing.T) {
	d, err := db.Open(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	r := &Runner{db: d}

	var ran []string
	stages := []*Stage{
		{Name: "fetch", Every: time.Minute, Run: func(ctx context.Context) (string, error) {
			ran = append(ran, "fetch")
			return "ok", nil
		}},
		{Name: "classify", Every: time.Minute, Run: func(ctx context.Context) (string, error) {
			ran = append(ran, "classify")
			return "", errors.New("the model is down")
		}},
		{Name: "kb", Every: time.Minute, Run: func(ctx context.Context) (string, error) {
			ran = append(ran, "kb")
			return "ok", nil
		}},
	}

	if err := r.runCycle(context.Background(), db.NewLockRepo(d), stages, true); err != nil {
		t.Fatalf("a failing stage ended the cycle: %v", err)
	}
	if fmt.Sprint(ran) != "[fetch classify kb]" {
		t.Fatalf("stages ran: %v — the graph must still be built when the model is down", ran)
	}
	// The lock is given back, however the cycle went.
	lock, err := db.NewLockRepo(d).Acquire(context.Background(), lockName, time.Minute)
	if err != nil {
		t.Fatalf("the lock was not released: %v", err)
	}
	_ = lock.Release(context.Background())
}

// Being asked to stop lands between two stages, never inside a write.
func TestCancellationStopsBetweenStages(t *testing.T) {
	d, err := db.Open(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	r := &Runner{db: d}

	ctx, cancel := context.WithCancel(context.Background())
	var ran []string
	stages := []*Stage{
		{Name: "fetch", Every: time.Minute, Run: func(context.Context) (string, error) {
			ran = append(ran, "fetch")
			cancel() // the signal arrives while the first stage is working
			return "ok", nil
		}},
		{Name: "classify", Every: time.Minute, Run: func(context.Context) (string, error) {
			ran = append(ran, "classify")
			return "ok", nil
		}},
	}
	if err := r.runCycle(ctx, db.NewLockRepo(d), stages, true); err != nil {
		t.Fatal(err)
	}
	if fmt.Sprint(ran) != "[fetch]" {
		t.Fatalf("stages ran: %v — the second one should not have started", ran)
	}
}

// Two copies of giznews must not fetch the same feeds into the same database.
func TestOnlyOnePipelineRunsAtATime(t *testing.T) {
	d, err := db.Open(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	ctx := context.Background()
	locks := db.NewLockRepo(d)

	held, err := locks.Acquire(ctx, lockName, time.Hour)
	if err != nil {
		t.Fatal(err)
	}

	// A scheduled cycle waits its turn rather than failing.
	r := &Runner{db: d}
	ran := false
	stages := []*Stage{{Name: "fetch", Every: time.Minute, Run: func(context.Context) (string, error) {
		ran = true
		return "ok", nil
	}}}
	if err := r.runCycle(ctx, locks, stages, false); err != nil {
		t.Fatalf("a busy pipeline is not an error for the loop: %v", err)
	}
	if ran {
		t.Fatal("the stage ran while another process held the lock")
	}

	// A one-shot run says so instead, so cron does not report success for work
	// it never did.
	if err := r.runCycle(ctx, locks, stages, true); !errors.Is(err, db.ErrLocked) {
		t.Fatalf("--once error = %v, want ErrLocked", err)
	}

	// And a command typed by hand is told who has it.
	err = WithLock(ctx, d, nil, func(context.Context) error { return nil })
	if err == nil {
		t.Fatal("expected the manual path to refuse")
	}

	// Once it is let go, the next run proceeds.
	if err := held.Release(ctx); err != nil {
		t.Fatal(err)
	}
	if err := r.runCycle(ctx, locks, stages, true); err != nil {
		t.Fatal(err)
	}
	if !ran {
		t.Fatal("the stage never ran after the lock was released")
	}
}

// A process that dies holds nothing for long.
func TestALockExpires(t *testing.T) {
	d, err := db.Open(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	ctx := context.Background()
	locks := db.NewLockRepo(d)

	if _, err := locks.Acquire(ctx, "pipeline", -time.Minute); err != nil {
		t.Fatal(err)
	}
	// A negative ttl is clamped to the default, so force the expiry by hand the
	// way a crashed process leaves it: in the past.
	if _, err := d.SQL().ExecContext(ctx,
		"UPDATE locks SET expires_at = ? WHERE name = 'pipeline'",
		time.Now().UTC().Add(-time.Hour).Format(time.RFC3339)); err != nil {
		t.Fatal(err)
	}
	if _, err := locks.Acquire(ctx, "pipeline", time.Minute); err != nil {
		t.Fatalf("an expired lock must be takeable: %v", err)
	}
}
