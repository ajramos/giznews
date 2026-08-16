package desktop

import (
	"log"
	"os"
	"path/filepath"
	"strings"
)

// logger returns the shared logger writing to giznews.log (next to the DB),
// so long-running jobs leave an auditable trail. Falls back to a no-op logger
// if the file cannot be opened.
func (a *App) logger() *log.Logger {
	a.logOnce.Do(func() {
		dir := filepath.Dir(a.cfg.ResolveDBPath())
		f, err := os.OpenFile(filepath.Join(dir, "giznews.log"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
		if err != nil {
			a.loggerL = discardLogger()
			return
		}
		a.loggerL = log.New(f, "giznews: ", log.LstdFlags)
	})
	return a.loggerL
}

// Logs returns the tail of the giznews.log file (last `limit` lines), so the
// UI can show what the pipeline has been deciding.
func (a *App) Logs(limit int) string {
	if limit <= 0 {
		limit = 200
	}
	dir := filepath.Dir(a.cfg.ResolveDBPath())
	b, err := os.ReadFile(filepath.Join(dir, "giznews.log"))
	if err != nil {
		return "No log file yet."
	}
	lines := strings.Split(strings.TrimRight(string(b), "\n"), "\n")
	if len(lines) > limit {
		lines = lines[len(lines)-limit:]
	}
	return strings.Join(lines, "\n")
}
