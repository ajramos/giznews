// Package config loads and validates giznews configuration from a JSON file.
//
// It mirrors the patterns used by giztui (lenient integer unmarshalling,
// defaulting, home-directory expansion) so the two apps feel consistent.
package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// DefaultConfigDir is where giznews looks for its configuration.
const DefaultConfigDir = "~/.config/giznews"

// DefaultDBName is the SQLite database file name inside the config dir.
const DefaultDBName = "giznews.db"

// DefaultVaultPath is the default location of the dedicated knowledge vault.
// Obsidian can open it directly. Overridable via config "vault_path".
const DefaultVaultPath = "~/Documents/obsidian/chronicles-ai"

// lenientInt is an int that also unmarshals from a JSON string ("12"), so a
// number accidentally quoted in config.json (a common hand-editing mistake)
// doesn't fail the whole load and discard every other setting. It marshals back
// as a plain number.
type lenientInt int

func (n *lenientInt) UnmarshalJSON(b []byte) error {
	if len(b) > 0 && b[0] != '"' {
		var i int
		if err := json.Unmarshal(b, &i); err != nil {
			return err
		}
		*n = lenientInt(i)
		return nil
	}
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return err
	}
	s = strings.TrimSpace(s)
	if s == "" {
		*n = 0
		return nil
	}
	i, err := strconv.Atoi(s)
	if err != nil {
		return fmt.Errorf("invalid integer %q", s)
	}
	*n = lenientInt(i)
	return nil
}

// Config is the top-level configuration.
type Config struct {
	// Version of the config schema. Currently 1.
	Version int `json:"version"`

	// DBPath is the SQLite database path. Empty → <config_dir>/giznews.db.
	DBPath string `json:"db_path"`

	// VaultPath is the dedicated Obsidian-compatible knowledge vault.
	VaultPath string `json:"vault_path"`

	// LLM configures the language model used for classification, summaries,
	// digests and embeddings.
	LLM LLMConfig `json:"llm"`

	// Gmail configures newsletter ingestion. Credentials and token are shared
	// with giztui by default (~/.config/giztui/).
	Gmail GmailConfig `json:"gmail"`

	// Digest configures daily digest generation.
	Digest DigestConfig `json:"digest"`

	// Classify configures article classification.
	Classify ClassifyConfig `json:"classify"`

	// Fetch configures article ingestion from sources.
	Fetch FetchConfig `json:"fetch"`

	// KB configures how the knowledge graph is built.
	KB KBConfig `json:"kb"`

	// Extract configures full-article content extraction (readability) during
	// fetch, so bodies are ready before you open them.
	Extract ExtractConfig `json:"extract"`

	// Sources are the news sources to track. Managed via CLI/UI; the JSON
	// field is a convenience for hand-editing.
	Sources []SourceConfig `json:"sources,omitempty"`
}

// LLMConfig holds all LLM-related settings.
type LLMConfig struct {
	Enabled bool `json:"enabled"`
	// Provider: "ollama", "openai", "anthropic", "bedrock", "custom".
	Provider string `json:"provider"`
	// Model used for chat/completion tasks (summaries, classification, digests).
	Model string `json:"model"`
	// EmbeddingModel used for semantic search embeddings.
	EmbeddingModel string `json:"embedding_model"`
	// Endpoint base URL. For "ollama" defaults to http://localhost:11434.
	// For "custom"/"openai" it is the base of the OpenAI-compatible API.
	Endpoint string `json:"endpoint"`
	// Region for AWS Bedrock.
	Region string `json:"region"`
	APIKey string `json:"api_key"`
	// Timeout for LLM HTTP calls, e.g. "120s".
	Timeout string `json:"timeout"`
	// Language is the ISO 639-1 code (e.g. "en", "es") used for LLM-generated
	// prose: article summaries, digest overviews/themes, classification
	// summaries/headlines, and KB molecule synthesis. Empty → English.
	Language string `json:"language"`
}

// GmailConfig configures newsletter ingestion via the Gmail API.
type GmailConfig struct {
	Enabled bool `json:"enabled"`
	// CredentialsPath points to the OAuth client credentials JSON. Defaults to
	// giztui's file so both apps share the same Google Cloud project.
	CredentialsPath string `json:"credentials_path"`
	// TokenPath points to the OAuth token JSON. Defaults to giztui's token.
	TokenPath string `json:"token_path"`
	// Queries are Gmail searches that select newsletters to ingest, e.g.
	// "from:(substack.com) category:updates". Empty → all labels.
	Queries []string `json:"queries,omitempty"`
	// MaxAge limits how far back the first fetch looks (e.g. "168h").
	MaxAge string `json:"max_age"`
}

// DigestConfig configures the daily digest.
type DigestConfig struct {
	// Schedule is a cron expression (5 fields). Empty → disabled.
	Schedule string `json:"schedule"`
	// MaxArticlesPerTheme caps how many articles are shown per theme group.
	MaxArticlesPerTheme int `json:"max_articles_per_theme"`
}

// ClassifyConfig configures classification.
type ClassifyConfig struct {
	// UseLLM sends non-deterministic articles to the LLM for classification.
	UseLLM bool `json:"use_llm"`
	// BatchSize is how many articles go in one LLM call.
	BatchSize int `json:"batch_size"`
	// Concurrency is how many LLM batches run in parallel.
	Concurrency int `json:"concurrency"`
	// ImportanceThreshold: articles at/above this importance surface in the UI.
	ImportanceThreshold int `json:"importance_threshold"`
	// CoverageSources is how many outlets must run the same story before it is
	// treated as important on that basis alone — the one signal a regex cannot
	// fake and the model cannot see. 0 disables it.
	CoverageSources int `json:"coverage_sources"`
	// CoverageFloor is the importance such a story gets at least.
	CoverageFloor int `json:"coverage_floor"`
	// Learn applies what `giznews learn` worked out from your own history:
	// what you star, and what you throw away without opening. Nothing happens
	// until that command has been run at least once.
	Learn LearnConfig `json:"learn"`
}

// KBConfig configures a knowledge-graph build. The defaults suit a personal
// feed; a heavier one wants a higher threshold, a quieter one a lower.
type KBConfig struct {
	// MinOccurrences is how many notes must mention a concept before it gets an
	// Electron of its own. Mentions accumulate across runs.
	MinOccurrences int `json:"min_occurrences"`
	// AgeDays limits how far back a build looks for articles to turn into atoms.
	AgeDays int `json:"age_days"`
	// Limit caps how many atoms one run writes.
	Limit int `json:"limit"`
	// ThemeDays is how far back theme clustering looks for notes to group into
	// molecules. Wider than AgeDays on purpose: a story told over two months is
	// still one story, while an article that old is no longer news.
	ThemeDays int `json:"theme_days"`
}

// LearnConfig configures the adjustment learned from the reader's history.
// Deliberately conservative: a personal feed produces little evidence, and an
// adjustment nobody can explain is worse than none.
type LearnConfig struct {
	// Enabled applies the stored adjustments during classification.
	Enabled bool `json:"enabled"`
	// WindowDays is how far back a learning pass looks.
	WindowDays int `json:"window_days"`
	// MinSamples is how many articles a source or tag needs before it is
	// allowed to move anything at all.
	MinSamples int `json:"min_samples"`
	// MaxDelta bounds the move, in either direction.
	MaxDelta int `json:"max_delta"`
}

// FetchConfig configures article ingestion from sources.
type FetchConfig struct {
	// MaxAgeDays drops feed items published more than N days ago, so archive
	// dumps (e.g. a blog RSS exposing its whole history) don't flood the queue.
	// 0 keeps everything.
	MaxAgeDays int `json:"max_age_days"`
}

// ExtractConfig configures full-content extraction during fetch.
type ExtractConfig struct {
	// OnFetch extracts article bodies (readability → markdown) after fetching,
	// so articles are ready to read without an on-open network round trip.
	OnFetch bool `json:"on_fetch"`
	// Limit caps how many short articles are extracted per fetch run.
	Limit int `json:"limit"`
	// Concurrency is the number of parallel extraction workers.
	Concurrency int `json:"concurrency"`
}

// SourceConfig describes a news source in the config file. The live source
// registry lives in the DB; this is only a hand-editing convenience.
type SourceConfig struct {
	Name  string `json:"name"`
	Type  string `json:"type"` // rss, hackernews, arxiv, gmail
	URL   string `json:"url"`
	Group string `json:"group,omitempty"`
}

// DefaultConfig returns a fully-populated config with sensible defaults.
func DefaultConfig() *Config {
	return &Config{
		Version:   1,
		DBPath:    "",
		VaultPath: DefaultVaultPath,
		LLM: LLMConfig{
			Enabled:        false,
			Provider:       "ollama",
			Model:          "llama3.2",
			EmbeddingModel: "nomic-embed-text",
			Endpoint:       "http://localhost:11434",
			Timeout:        "120s",
			Language:       "en",
		},
		Gmail: GmailConfig{
			Enabled:         false,
			CredentialsPath: "~/.config/giztui/credentials.json",
			TokenPath:       "~/.config/giztui/token.json",
			MaxAge:          "168h",
			Queries:         []string{},
		},
		Digest: DigestConfig{
			Schedule:            "0 8 * * *",
			MaxArticlesPerTheme: 5,
		},
		Classify: ClassifyConfig{
			UseLLM:              true,
			BatchSize:           20,
			Concurrency:         2,
			ImportanceThreshold: 2,
			CoverageSources:     3,
			CoverageFloor:       2,
			Learn: LearnConfig{
				Enabled:    true,
				WindowDays: 90,
				MinSamples: 20,
				MaxDelta:   1,
			},
		},
		Fetch: FetchConfig{
			MaxAgeDays: 30,
		},
		KB: KBConfig{
			MinOccurrences: 2,
			AgeDays:        30,
			Limit:          200,
			ThemeDays:      90,
		},
		Extract: ExtractConfig{
			OnFetch:     true,
			Limit:       20,
			Concurrency: 4,
		},
		Sources: []SourceConfig{},
	}
}

// LoadConfig reads the config file at path (or the default location when path
// is empty) and merges any missing fields with defaults. If the file does not
// exist, a default config is returned without error.
func LoadConfig(path string) (*Config, error) {
	if path == "" {
		path = filepath.Join(DefaultConfigDir, "config.json")
	}
	path = expandHome(path)

	b, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return DefaultConfig(), nil
		}
		return nil, fmt.Errorf("read config %s: %w", path, err)
	}

	cfg := DefaultConfig()
	if err := json.Unmarshal(b, cfg); err != nil {
		return nil, fmt.Errorf("parse config %s: %w", path, err)
	}
	return cfg, nil
}

// Save writes the config as pretty JSON to path.
func (c *Config) Save(path string) error {
	path = expandHome(path)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}
	b, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}
	if err := os.WriteFile(path, b, 0o644); err != nil {
		return fmt.Errorf("write config %s: %w", path, err)
	}
	return nil
}

// ResolveDBPath returns the absolute SQLite database path.
func (c *Config) ResolveDBPath() string {
	p := c.DBPath
	if p == "" {
		p = filepath.Join(DefaultConfigDir, DefaultDBName)
	}
	return expandHome(p)
}

// ResolveVaultPath returns the absolute knowledge-vault path.
func (c *Config) ResolveVaultPath() string {
	return expandHome(c.VaultPath)
}

// ResolveGmailCredentialsPath returns the absolute credentials path.
func (c *Config) ResolveGmailCredentialsPath() string {
	return expandHome(c.Gmail.CredentialsPath)
}

// ResolveGmailTokenPath returns the absolute token path.
func (c *Config) ResolveGmailTokenPath() string {
	return expandHome(c.Gmail.TokenPath)
}

// LLMTimeout returns the parsed LLM timeout, or 120s as a fallback.
func (c *Config) LLMTimeout() time.Duration {
	d, err := time.ParseDuration(c.LLM.Timeout)
	if err != nil || d <= 0 {
		return 120 * time.Second
	}
	return d
}

// expandHome expands a leading "~" to the user's home directory.
func expandHome(p string) string {
	if p == "~" || strings.HasPrefix(p, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return p
		}
		if p == "~" {
			return home
		}
		return filepath.Join(home, p[2:])
	}
	return p
}
