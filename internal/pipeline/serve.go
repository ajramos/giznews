package pipeline

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/ajramos/giznews/internal/db"
)

// lockName is the one lock the pipeline takes. Stages are not independent —
// classify reads what fetch wrote, the graph reads what classify decided — so
// two copies interleaving them would produce a mess no per-stage lock could
// prevent.
const lockName = "pipeline"

// lockTTL is how long a stage may run before another process is entitled to
// assume it died. Renewed while a cycle is in progress.
const lockTTL = 20 * time.Minute

// tick is how often the loop wakes to see what is due. Nothing here is urgent
// to the second, and a sleepy loop is a loop that does not burn a laptop.
const tick = 30 * time.Second

// ServeOptions configures an unattended run.
type ServeOptions struct {
	FetchEvery    time.Duration
	ClassifyEvery time.Duration
	KBEvery       time.Duration
	IndexEvery    time.Duration
	DigestAt      string // "07:30", empty to skip
	// Once runs every enabled stage a single time and returns, for cron.
	Once bool
}

// Serve runs the pipeline until the context is cancelled.
//
// The loop's promise is that it keeps going: a stage that fails is logged and
// the next one still runs, because a feed that stops updating because the model
// was down is worse than a feed with a gap in its notes.
func (r *Runner) Serve(ctx context.Context, opts ServeOptions) error {
	stages := r.stages(opts)
	enabled := make([]*Stage, 0, len(stages))
	for _, s := range stages {
		if s.Every > 0 || s.At != "" {
			enabled = append(enabled, s)
		}
	}
	if len(enabled) == 0 {
		return errors.New("serve: every stage is switched off — nothing to do")
	}

	locks := db.NewLockRepo(r.db)
	if opts.Once {
		return r.runCycle(ctx, locks, enabled, true)
	}

	for _, s := range enabled {
		r.logf("serve: %s", s)
	}
	for {
		if err := r.runCycle(ctx, locks, enabled, false); err != nil {
			return err
		}
		select {
		case <-ctx.Done():
			r.logf("serve: stopped")
			return nil
		case <-time.After(tick):
		}
	}
}

// runCycle runs whatever is due, under one lock.
func (r *Runner) runCycle(ctx context.Context, locks *db.LockRepo, stages []*Stage, force bool) error {
	now := time.Now()
	due := make([]*Stage, 0, len(stages))
	for _, s := range stages {
		if force || s.Due(now) {
			due = append(due, s)
		}
	}
	if len(due) == 0 {
		return nil
	}

	lock, err := locks.Acquire(ctx, lockName, lockTTL)
	if err != nil {
		if errors.Is(err, db.ErrLocked) {
			// Somebody else is working. Not an error: try again next tick.
			r.logf("serve: waiting, %s holds the pipeline", locks.Holder(ctx, lockName))
			if force {
				return fmt.Errorf("serve --once: %w", err)
			}
			return nil
		}
		return err
	}
	defer func() {
		// The release runs on its own context: the one that got here may
		// already be cancelled, and a lock nobody released is a lock the next
		// run has to wait out.
		releaseCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = lock.Release(releaseCtx)
	}()

	for _, s := range due {
		if ctx.Err() != nil {
			return nil // asked to stop: finish where we are, mid-cycle is fine
		}
		if err := lock.Renew(ctx, lockTTL); err != nil {
			r.logf("serve: could not renew the lock: %v", err)
		}
		started := time.Now()
		summary, err := s.Run(ctx)
		s.Ran(started)
		switch {
		case errors.Is(err, context.Canceled):
			return nil
		case err != nil:
			// One stage failing must not cost the others: the model being down
			// should not stop the feed from fetching.
			r.logf("serve: %s failed: %v", s.Name, err)
		default:
			r.logf("serve: %s — %s (%s)", s.Name, summary, time.Since(started).Round(time.Millisecond))
		}
	}
	return nil
}

// stages is the pipeline in the order it has to happen: nothing can be
// classified before it is fetched, and nothing becomes a note before it is
// classified.
func (r *Runner) stages(opts ServeOptions) []*Stage {
	return []*Stage{
		{Name: "fetch", Every: opts.FetchEvery, Run: r.Fetch},
		{Name: "classify", Every: opts.ClassifyEvery, Run: r.Classify},
		{Name: "kb", Every: opts.KBEvery, Run: r.KB},
		{Name: "index", Every: opts.IndexEvery, Run: r.Index},
		{Name: "digest", At: opts.DigestAt, Run: r.Digest},
	}
}

func (r *Runner) logf(format string, args ...any) {
	if r.logger != nil {
		r.logger.Printf(format, args...)
	}
}

// WithLock runs fn while holding the pipeline lock, so a command typed by hand
// does not run alongside a daemon doing the same work.
func WithLock(ctx context.Context, database *db.DB, logger *log.Logger, fn func(context.Context) error) error {
	locks := db.NewLockRepo(database)
	lock, err := locks.Acquire(ctx, lockName, lockTTL)
	if err != nil {
		if errors.Is(err, db.ErrLocked) {
			return fmt.Errorf("%s is already working (%s) — wait for it, or stop it first",
				lockName, locks.Holder(ctx, lockName))
		}
		return err
	}
	defer func() {
		releaseCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = lock.Release(releaseCtx)
	}()
	return fn(ctx)
}
