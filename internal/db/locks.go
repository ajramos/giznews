package db

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync/atomic"
	"time"
)

// lockSeq makes every holder distinguishable, even within one process.
var lockSeq uint64

// ErrLocked says somebody else is already doing this.
var ErrLocked = errors.New("already running elsewhere")

// LockRepo hands out advisory locks. They exist so a `serve` loop and a person
// at a terminal do not fetch the same feeds into the same database at the same
// moment.
//
// Every lock expires. A process that is killed mid-stage leaves its row behind,
// and the next run takes it over once the deadline passes — which is the whole
// reason the deadline is short and renewed rather than long and hopeful.
type LockRepo struct {
	db *DB
}

// NewLockRepo creates a lock repository.
func NewLockRepo(db *DB) *LockRepo {
	return &LockRepo{db: db}
}

// Lock is a held lock, and how it is let go.
type Lock struct {
	repo  *LockRepo
	name  string
	owner string
}

// Acquire takes the named lock for ttl, or reports who has it. The owner string
// is only ever read by a person wondering what is holding it.
func (r *LockRepo) Acquire(ctx context.Context, name string, ttl time.Duration) (*Lock, error) {
	if ttl <= 0 {
		ttl = 15 * time.Minute
	}
	now := time.Now().UTC()
	// The owner identifies the holder, not the process: two jobs inside one
	// program must exclude each other exactly as two programs do.
	owner := fmt.Sprintf("%s/%d#%d", hostname(), os.Getpid(), atomic.AddUint64(&lockSeq, 1))

	res, err := r.db.sql.ExecContext(ctx, `
		INSERT INTO locks (name, owner, acquired_at, expires_at) VALUES (?, ?, ?, ?)
		ON CONFLICT(name) DO UPDATE SET
			owner = excluded.owner,
			acquired_at = excluded.acquired_at,
			expires_at = excluded.expires_at
		WHERE locks.expires_at < ?`,
		name, owner, now.Format(time.RFC3339), now.Add(ttl).Format(time.RFC3339),
		now.Format(time.RFC3339))
	if err != nil {
		return nil, fmt.Errorf("acquire lock %q: %w", name, err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return nil, fmt.Errorf("acquire lock %q: %w", name, err)
	}
	if n == 0 {
		return nil, fmt.Errorf("%q: %w", name, ErrLocked)
	}
	return &Lock{repo: r, name: name, owner: owner}, nil
}

// Renew pushes the deadline out, for a stage taking longer than expected.
func (l *Lock) Renew(ctx context.Context, ttl time.Duration) error {
	if l == nil {
		return nil
	}
	_, err := l.repo.db.sql.ExecContext(ctx,
		"UPDATE locks SET expires_at = ? WHERE name = ? AND owner = ?",
		time.Now().UTC().Add(ttl).Format(time.RFC3339), l.name, l.owner)
	if err != nil {
		return fmt.Errorf("renew lock %q: %w", l.name, err)
	}
	return nil
}

// Release gives the lock back. It is safe to call twice, and safe to call on a
// lock somebody else has already taken over.
func (l *Lock) Release(ctx context.Context) error {
	if l == nil {
		return nil
	}
	_, err := l.repo.db.sql.ExecContext(ctx,
		"DELETE FROM locks WHERE name = ? AND owner = ?", l.name, l.owner)
	if err != nil {
		return fmt.Errorf("release lock %q: %w", l.name, err)
	}
	return nil
}

// Holder names whoever holds the lock, for an error message worth reading.
func (r *LockRepo) Holder(ctx context.Context, name string) string {
	var owner, expires string
	err := r.db.sql.QueryRowContext(ctx,
		"SELECT owner, expires_at FROM locks WHERE name = ?", name).Scan(&owner, &expires)
	if err != nil {
		return ""
	}
	return fmt.Sprintf("%s (until %s)", owner, expires)
}

func hostname() string {
	h, err := os.Hostname()
	if err != nil || h == "" {
		return "unknown"
	}
	return h
}
