package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.Version != 1 {
		t.Fatalf("version = %d, want 1", cfg.Version)
	}
	if cfg.LLM.Provider != "ollama" {
		t.Fatalf("provider = %q, want ollama", cfg.LLM.Provider)
	}
	if cfg.Gmail.CredentialsPath != "~/.config/giztui/credentials.json" {
		t.Fatalf("gmail creds = %q", cfg.Gmail.CredentialsPath)
	}
	if cfg.Classify.BatchSize != 20 {
		t.Fatalf("batch size = %d, want 20", cfg.Classify.BatchSize)
	}
}

func TestLenientIntString(t *testing.T) {
	var n lenientInt
	if err := n.UnmarshalJSON([]byte(`"42"`)); err != nil {
		t.Fatal(err)
	}
	if n != 42 {
		t.Fatalf("n = %d, want 42", n)
	}
}

func TestLenientIntNumber(t *testing.T) {
	var n lenientInt
	if err := n.UnmarshalJSON([]byte(`7`)); err != nil {
		t.Fatal(err)
	}
	if n != 7 {
		t.Fatalf("n = %d, want 7", n)
	}
}

func TestLoadConfigMissingFieldsDefaulted(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	if err := os.WriteFile(path, []byte(`{"version":1,"llm":{"provider":"custom"}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.LLM.Provider != "custom" {
		t.Fatalf("provider = %q", cfg.LLM.Provider)
	}
	if cfg.LLM.Model != "llama3.2" {
		t.Fatalf("model should be defaulted, got %q", cfg.LLM.Model)
	}
}

func TestLoadConfigMissingFileReturnsDefault(t *testing.T) {
	cfg, err := LoadConfig("/nonexistent/path/config.json")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.VaultPath != DefaultVaultPath {
		t.Fatalf("vault = %q", cfg.VaultPath)
	}
}

func TestResolvePaths(t *testing.T) {
	cfg := DefaultConfig()
	if got := cfg.ResolveVaultPath(); got == "" || got[0] == '~' {
		t.Fatalf("vault not expanded: %q", got)
	}
	if got := cfg.ResolveDBPath(); got == "" || got[0] == '~' {
		t.Fatalf("db not expanded: %q", got)
	}
	if got := cfg.ResolveGmailCredentialsPath(); got == "" || got[0] == '~' {
		t.Fatalf("creds not expanded: %q", got)
	}
}

func TestLLMTimeoutFallback(t *testing.T) {
	cfg := DefaultConfig()
	cfg.LLM.Timeout = "garbage"
	if d := cfg.LLMTimeout(); d != 120*time.Second {
		t.Fatalf("timeout = %v", d)
	}
	cfg.LLM.Timeout = "30s"
	if d := cfg.LLMTimeout(); d != 30*time.Second {
		t.Fatalf("timeout = %v", d)
	}
}
