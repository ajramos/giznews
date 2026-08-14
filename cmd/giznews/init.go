package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/ajramos/giznews/internal/config"
	"github.com/ajramos/giznews/internal/db"
)

// runInit creates the config file, the SQLite database and the knowledge-vault
// skeleton so the user can open the vault in Obsidian right away.
func runInit(logger *log.Logger) {
	cfg := config.DefaultConfig()

	configPath := filepath.Join(config.DefaultConfigDir, "config.json")
	if _, err := os.Stat(configPath); err == nil {
		fmt.Printf("config already exists at %s — leaving it untouched.\n", configPath)
	} else {
		if err := cfg.Save(configPath); err != nil {
			logger.Fatalf("write config: %v", err)
		}
		fmt.Printf("wrote config to %s\n", configPath)
	}

	d, err := db.Open(cfg.ResolveDBPath())
	if err != nil {
		logger.Fatalf("open database: %v", err)
	}
	defer d.Close()
	fmt.Printf("database ready at %s\n", cfg.ResolveDBPath())

	if err := ensureVault(cfg.ResolveVaultPath()); err != nil {
		logger.Fatalf("create vault: %v", err)
	}
	fmt.Printf("knowledge vault ready at %s\n", cfg.ResolveVaultPath())

	fmt.Println("\ngiznews initialized. Next: `giznews sources add <url> --type rss`.")
}

// ensureVault creates the Obsidian-compatible directory skeleton.
func ensureVault(root string) error {
	for _, dir := range []string{
		"00-Inbox",
		"01-Electrons",
		"02-Atoms",
		"03-Molecules",
		"04-GTD",
		"99-Recursos",
	} {
		if err := os.MkdirAll(filepath.Join(root, dir), 0o755); err != nil {
			return fmt.Errorf("create vault dir %s: %w", dir, err)
		}
	}
	return nil
}
