package desktop

import (
	"context"
	"fmt"

	"github.com/ajramos/giznews/internal/db"
)

// jobProgress is the per-job progress handle passed to a running job so it can
// report phase/done/total without reaching into the manager.
type jobProgress struct {
	mgr *JobManager
	id  int64
}

func (p *jobProgress) Progress(phase string, done, total int) {
	p.mgr.Progress(p.id, phase, done, total)
}

func (p *jobProgress) Message(msg string) { p.mgr.SetMessage(p.id, msg) }

// trackJob runs fn as a background job. It blocks until fn returns (each Wails
// call already runs in its own goroutine), so the API method keeps returning
// its result while the job stays visible and cancellable in the picker.
func (a *App) trackJob(ctx context.Context, name, typ string, run func(ctx context.Context, p *jobProgress) error) error {
	id, jobCtx, cancel := a.jobs.Begin(ctx, name, typ)
	defer cancel()
	err := run(jobCtx, &jobProgress{mgr: a.jobs, id: id})
	a.jobs.Finish(id, err)
	return err
}

// ListJobs returns all tracked jobs (newest first).
func (a *App) ListJobs(ctx context.Context) ([]*JobDTO, error) {
	return a.jobs.List(), nil
}

// RemoveJob drops a single job from the registry.
func (a *App) RemoveJob(ctx context.Context, id int64) error {
	a.jobs.Remove(id)
	return nil
}

// ClearFinishedJobs drops every non-running job.
func (a *App) ClearFinishedJobs(ctx context.Context) error {
	a.jobs.ClearFinished()
	return nil
}

// CancelJob aborts a running job.
func (a *App) CancelJob(ctx context.Context, id int64) error {
	a.jobs.Cancel(id)
	return nil
}

// BulkResult reports what a bulk status change did.
type BulkResult struct {
	Updated int `json:"updated"`
	Total   int `json:"total"`
}

// BulkSetStatus applies a status to many articles in one cancellable background
// job, reporting progress per item. It replaces the frontend's per-article loop.
func (a *App) BulkSetStatus(ctx context.Context, ids []int64, status string) (*BulkResult, error) {
	st := db.ArticleStatus(status)
	switch st {
	case db.StatusUnread, db.StatusRead, db.StatusArchived, db.StatusStarred:
	default:
		return nil, fmt.Errorf("invalid status: %q", status)
	}
	total := len(ids)
	label := fmt.Sprintf("Mark %d %s", total, status)

	var res BulkResult
	res.Total = total
	err := a.trackJob(ctx, label, "bulk", func(ctx context.Context, p *jobProgress) error {
		repo := db.NewArticleRepo(a.db)
		for i, id := range ids {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			if err := repo.SetStatus(ctx, id, st); err != nil {
				return fmt.Errorf("article %d: %w", id, err)
			}
			res.Updated++
			p.Progress("bulk", i+1, total)
		}
		p.Message(fmt.Sprintf("%d updated", res.Updated))
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &res, nil
}
