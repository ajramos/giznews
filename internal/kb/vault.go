package kb

import (
	"fmt"
	"os"
	"path/filepath"
)

// dirOf is filepath.Dir, kept here so sync.go does not need the import.
func dirOf(path string) string { return filepath.Dir(path) }

// Vault is an Obsidian-compatible directory. Folder names mirror the chronicles
// structure so the vault opens cleanly in Obsidian and shares conventions.
type Vault struct {
	Root string
}

// FolderFor maps a note type to its vault subdirectory.
func FolderFor(noteType string) string {
	switch noteType {
	case "inbox":
		return "00-Inbox"
	case "atom":
		return "02-Atoms"
	case "electron":
		return "01-Electrons"
	case "molecule":
		return "03-Molecules"
	case "map":
		return "" // Index.md and Dangling.md sit at the vault root
	default:
		return "00-Inbox"
	}
}

// NewVault validates and prepares the vault root.
func NewVault(root string) (*Vault, error) {
	if root == "" {
		return nil, fmt.Errorf("kb: empty vault path")
	}
	if err := os.MkdirAll(root, 0o755); err != nil {
		return nil, fmt.Errorf("kb: create vault: %w", err)
	}
	return &Vault{Root: root}, nil
}

// NotePath returns the absolute markdown path for a note.
func (v *Vault) NotePath(noteType, slug string) string {
	return filepath.Join(v.Root, FolderFor(noteType), slug+".md")
}
