package desktop

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/ajramos/giznews/internal/config"
	"github.com/ajramos/giznews/internal/db"
)

// OpenApp boots the API for the desktop app: it loads the default config,
// opens the database and ensures the knowledge-vault skeleton exists. This
// keeps the Wails module (a separate Go module) on the public pkg/desktop
// boundary only — it must never import internal/.
func OpenApp() (*App, error) {
	cfg, err := config.LoadConfig("")
	if err != nil {
		return nil, fmt.Errorf("config: %w", err)
	}
	database, err := db.Open(cfg.ResolveDBPath())
	if err != nil {
		return nil, fmt.Errorf("database: %w", err)
	}
	app := NewApp(cfg, database)
	if err := app.EnsureVault(); err != nil {
		return nil, fmt.Errorf("vault: %w", err)
	}
	return app, nil
}

// EnsureVault creates the Obsidian-compatible directory skeleton.
func (a *App) EnsureVault() error {
	for _, dir := range []string{
		"00-Inbox", "01-Electrons", "02-Atoms", "03-Molecules", "04-GTD", "99-Recursos",
	} {
		if err := os.MkdirAll(filepath.Join(a.cfg.ResolveVaultPath(), dir), 0o755); err != nil {
			return err
		}
	}
	return nil
}

// VaultPath returns the absolute knowledge-vault path (used by the UI to open
// the vault in Obsidian).
func (a *App) VaultPath() string {
	return a.cfg.ResolveVaultPath()
}

// Close releases the underlying database.
func (a *App) Close() error {
	if a.db != nil {
		return a.db.Close()
	}
	return nil
}
