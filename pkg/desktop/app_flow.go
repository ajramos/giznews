package desktop

import (
	"context"

	"github.com/ajramos/giznews/internal/db"
)

// FlowStatus is a one-shot snapshot of every pipeline stage, used by the
// in-app `:flow` panel to show live counts along the end-to-end process.
type FlowStatus struct {
	SourcesTotal     int    `json:"sources_total"`
	SourcesEnabled   int    `json:"sources_enabled"`
	ArticlesTotal    int    `json:"articles_total"`
	Classified       int    `json:"classified"`
	PendingClassify  int    `json:"pending_classify"`
	Atoms            int    `json:"atoms"`
	Electrons        int    `json:"electrons"`
	Molecules        int    `json:"molecules"`
	VaultPath        string `json:"vault_path"`
	NotesEmbedded    int    `json:"notes_embedded"`
	ArticlesEmbedded int    `json:"articles_embedded"`
	RunningJobs      int    `json:"running_jobs"`
}

// Flow aggregates the counts behind each pipeline stage.
func (a *App) Flow(ctx context.Context) (*FlowStatus, error) {
	fs := &FlowStatus{VaultPath: a.cfg.ResolveVaultPath()}

	sources, err := db.NewSourceRepo(a.db).List(ctx)
	if err != nil {
		return nil, err
	}
	fs.SourcesTotal = len(sources)
	for _, s := range sources {
		if s.Enabled {
			fs.SourcesEnabled++
		}
	}

	sql := a.db.SQL()
	if err := sql.QueryRowContext(ctx, "SELECT COUNT(*) FROM articles").Scan(&fs.ArticlesTotal); err != nil {
		return nil, err
	}
	if err := sql.QueryRowContext(ctx, "SELECT COUNT(*) FROM articles WHERE classified = 1").Scan(&fs.Classified); err != nil {
		return nil, err
	}
	if err := sql.QueryRowContext(ctx, "SELECT COUNT(*) FROM articles WHERE status != 'archived' AND classified = 0").Scan(&fs.PendingClassify); err != nil {
		return nil, err
	}
	if err := sql.QueryRowContext(ctx, "SELECT COUNT(*) FROM kb_notes WHERE embedding IS NOT NULL").Scan(&fs.NotesEmbedded); err != nil {
		return nil, err
	}
	if err := sql.QueryRowContext(ctx, "SELECT COUNT(*) FROM articles WHERE embedding IS NOT NULL").Scan(&fs.ArticlesEmbedded); err != nil {
		return nil, err
	}

	rows, err := sql.QueryContext(ctx, "SELECT note_type, COUNT(*) FROM kb_notes GROUP BY note_type")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var t string
		var n int
		if err := rows.Scan(&t, &n); err != nil {
			return nil, err
		}
		switch t {
		case "atom":
			fs.Atoms = n
		case "electron":
			fs.Electrons = n
		case "molecule":
			fs.Molecules = n
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	for _, j := range a.jobs.List() {
		if j.Status == JobRunning {
			fs.RunningJobs++
		}
	}
	return fs, nil
}
