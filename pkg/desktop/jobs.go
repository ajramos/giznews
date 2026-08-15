package desktop

import (
	"context"
	"errors"
	"sort"
	"sync"
	"time"
)

// JobStatus is the lifecycle state of a background job.
type JobStatus string

const (
	JobRunning  JobStatus = "running"
	JobDone     JobStatus = "done"
	JobError    JobStatus = "error"
	JobCanceled JobStatus = "canceled"
)

// JobDTO is the JSON view of a background job shown in the jobs picker.
type JobDTO struct {
	ID         int64     `json:"id"`
	Name       string    `json:"name"`
	Type       string    `json:"type"`
	Status     JobStatus `json:"status"`
	Phase      string    `json:"phase"`
	Done       int       `json:"done"`
	Total      int       `json:"total"`
	Message    string    `json:"message,omitempty"`
	ErrMsg     string    `json:"err_msg,omitempty"`
	CreatedAt  string    `json:"created_at"`
	FinishedAt string    `json:"finished_at,omitempty"`
}

// job is the internal tracked job. The cancel func drives cancellation.
type job struct {
	ID         int64
	Name       string
	Type       string
	Status     JobStatus
	Phase      string
	Done       int
	Total      int
	Message    string
	ErrMsg     string
	CreatedAt  time.Time
	FinishedAt time.Time
	cancel     context.CancelFunc
}

// JobManager is an in-memory, mutex-guarded registry of background jobs
// (mirrors giztui's aiJobsRegistry, generalized to any long operation). Results
// are durable elsewhere (DB/vault); losing the registry on restart only loses
// the browse list, not the work.
type JobManager struct {
	mu     sync.RWMutex
	jobs   []*job
	nextID int64
}

// NewJobManager builds an empty registry.
func NewJobManager() *JobManager { return &JobManager{} }

// Begin registers a running job, derives a cancellable context from parent, and
// returns the job id, the derived context, and the cancel func.
func (m *JobManager) Begin(parent context.Context, name, typ string) (int64, context.Context, context.CancelFunc) {
	ctx, cancel := context.WithCancel(parent)
	m.mu.Lock()
	m.nextID++
	j := &job{ID: m.nextID, Name: name, Type: typ, Status: JobRunning, CreatedAt: time.Now(), cancel: cancel}
	m.jobs = append(m.jobs, j)
	m.mu.Unlock()
	return j.ID, ctx, cancel
}

// Progress updates a job's phase/progress. No-op if the job is gone.
func (m *JobManager) Progress(id int64, phase string, done, total int) {
	m.update(id, func(j *job) { j.Phase, j.Done, j.Total = phase, done, total })
}

// SetMessage sets an informational/completion message. No-op if gone.
func (m *JobManager) SetMessage(id int64, msg string) {
	m.update(id, func(j *job) { j.Message = msg })
}

// Finish records the terminal state: canceled context → canceled, an error →
// error (message preserved), otherwise done.
func (m *JobManager) Finish(id int64, err error) {
	m.update(id, func(j *job) {
		j.FinishedAt = time.Now()
		switch {
		case err == nil:
			j.Status = JobDone
		case errors.Is(err, context.Canceled):
			j.Status = JobCanceled
		default:
			j.Status = JobError
			j.ErrMsg = err.Error()
		}
	})
}

// Cancel aborts a running job via its cancel func.
func (m *JobManager) Cancel(id int64) {
	m.mu.RLock()
	var cancel context.CancelFunc
	for _, j := range m.jobs {
		if j.ID == id {
			cancel = j.cancel
			break
		}
	}
	m.mu.RUnlock()
	if cancel != nil {
		cancel()
	}
}

// Remove drops a job. No-op if absent.
func (m *JobManager) Remove(id int64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for i, j := range m.jobs {
		if j.ID == id {
			m.jobs = append(m.jobs[:i], m.jobs[i+1:]...)
			return
		}
	}
}

// ClearFinished drops every non-running job.
func (m *JobManager) ClearFinished() {
	m.mu.Lock()
	defer m.mu.Unlock()
	kept := make([]*job, 0, len(m.jobs))
	for _, j := range m.jobs {
		if j.Status == JobRunning {
			kept = append(kept, j)
		}
	}
	m.jobs = kept
}

// List returns snapshots of all jobs, newest first.
func (m *JobManager) List() []*JobDTO {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]*JobDTO, 0, len(m.jobs))
	for _, j := range m.jobs {
		out = append(out, toJobDTO(j))
	}
	sort.SliceStable(out, func(i, k int) bool { return out[i].ID > out[k].ID })
	return out
}

func (m *JobManager) update(id int64, fn func(*job)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, j := range m.jobs {
		if j.ID == id {
			fn(j)
			return
		}
	}
}

func toJobDTO(j *job) *JobDTO {
	d := &JobDTO{
		ID: j.ID, Name: j.Name, Type: j.Type, Status: j.Status,
		Phase: j.Phase, Done: j.Done, Total: j.Total,
		Message: j.Message, ErrMsg: j.ErrMsg,
		CreatedAt: j.CreatedAt.UTC().Format(time.RFC3339),
	}
	if !j.FinishedAt.IsZero() {
		d.FinishedAt = j.FinishedAt.UTC().Format(time.RFC3339)
	}
	return d
}
