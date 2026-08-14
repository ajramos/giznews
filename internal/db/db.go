// Package db provides the SQLite persistence layer for giznews.
//
// It uses modernc.org/sqlite (pure Go, no CGO), the same choice as giztui, and
// runs in WAL mode so reads are never blocked by the single background writer.
package db

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"
)

// DB wraps the underlying SQL handle and applies migrations on open.
type DB struct {
	sql *sql.DB
}

// Open opens (creating if needed) the SQLite database at path, sets WAL mode
// and applies the schema migrations.
func Open(path string) (*DB, error) {
	if dir := filepath.Dir(path); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, fmt.Errorf("create db dir: %w", err)
		}
	}

	sqlDB, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	sqlDB.SetMaxOpenConns(1)

	d := &DB{sql: sqlDB}
	if err := d.configure(ctxNoTimeout()); err != nil {
		sqlDB.Close()
		return nil, err
	}
	if err := d.migrate(ctxNoTimeout()); err != nil {
		sqlDB.Close()
		return nil, err
	}
	return d, nil
}

func ctxNoTimeout() context.Context {
	return context.Background()
}

func (d *DB) configure(ctx context.Context) error {
	for _, stmt := range []string{
		"PRAGMA journal_mode=WAL;",
		"PRAGMA foreign_keys=ON;",
		"PRAGMA busy_timeout=5000;",
		"PRAGMA synchronous=NORMAL;",
	} {
		if _, err := d.sql.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("configure sqlite (%q): %w", stmt, err)
		}
	}
	return nil
}

// migrate applies any pending schema migrations using PRAGMA user_version.
func (d *DB) migrate(ctx context.Context) error {
	var version int
	if err := d.sql.QueryRowContext(ctx, "PRAGMA user_version;").Scan(&version); err != nil {
		return fmt.Errorf("read user_version: %w", err)
	}

	// Each entry is a full DDL block; the migration runner executes all blocks
	// with index > version inside a transaction.
	migrations := []string{schemaV1, schemaV2, schemaV3}

	for i := version; i < len(migrations); i++ {
		tx, err := d.sql.BeginTx(ctx, nil)
		if err != nil {
			return fmt.Errorf("begin migration %d: %w", i+1, err)
		}
		if _, err := tx.Exec(migrations[i]); err != nil {
			tx.Rollback()
			return fmt.Errorf("apply migration %d: %w", i+1, err)
		}
		if _, err := tx.Exec(fmt.Sprintf("PRAGMA user_version = %d;", i+1)); err != nil {
			tx.Rollback()
			return fmt.Errorf("set user_version %d: %w", i+1, err)
		}
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("commit migration %d: %w", i+1, err)
		}
	}
	return nil
}

// Close closes the underlying connection.
func (d *DB) Close() error {
	return d.sql.Close()
}

// SQL exposes the raw handle for repositories.
func (d *DB) SQL() *sql.DB {
	return d.sql
}

// Now is a convenience for a stable timestamp format across the codebase.
func Now() string {
	return time.Now().UTC().Format(time.RFC3339)
}

// schemaV1 is the initial schema.
const schemaV1 = `
CREATE TABLE IF NOT EXISTS sources (
	id          INTEGER PRIMARY KEY AUTOINCREMENT,
	name        TEXT    NOT NULL UNIQUE,
	type        TEXT    NOT NULL,             -- rss | hackernews | arxiv | gmail
	url         TEXT    NOT NULL DEFAULT '',
	params      TEXT    NOT NULL DEFAULT '{}', -- JSON, e.g. {"query":"..."} for gmail
	group_name  TEXT    NOT NULL DEFAULT 'general',
	enabled     INTEGER NOT NULL DEFAULT 1,
	last_fetch  TEXT,                          -- RFC3339
	created_at  TEXT    NOT NULL,
	updated_at  TEXT    NOT NULL
);

CREATE TABLE IF NOT EXISTS articles (
	id          INTEGER PRIMARY KEY AUTOINCREMENT,
	source_id   INTEGER NOT NULL REFERENCES sources(id),
	guid        TEXT    NOT NULL,              -- unique per source (RSS guid, HN id, …)
	url         TEXT    NOT NULL,
	title       TEXT    NOT NULL,
	author      TEXT    NOT NULL DEFAULT '',
	content_html TEXT   NOT NULL DEFAULT '',
	content_md  TEXT    NOT NULL DEFAULT '',
	summary     TEXT    NOT NULL DEFAULT '',
	category    TEXT    NOT NULL DEFAULT '',
	tags        TEXT    NOT NULL DEFAULT '[]', -- JSON array of strings
	entities    TEXT    NOT NULL DEFAULT '[]', -- JSON array of {name,type}
	importance  INTEGER NOT NULL DEFAULT 0,    -- 0 (low) .. 3 (critical)
	simhash     INTEGER NOT NULL DEFAULT 0,
	status      TEXT    NOT NULL DEFAULT 'unread', -- unread | read | archived | starred
	published   TEXT,                          -- RFC3339
	fetched_at  TEXT    NOT NULL,
	updated_at  TEXT    NOT NULL,
	UNIQUE (source_id, guid)
);
CREATE INDEX IF NOT EXISTS idx_articles_status     ON articles(status);
CREATE INDEX IF NOT EXISTS idx_articles_published  ON articles(published);
CREATE INDEX IF NOT EXISTS idx_articles_fetched    ON articles(fetched_at);
CREATE INDEX IF NOT EXISTS idx_articles_simhash    ON articles(simhash);
CREATE INDEX IF NOT EXISTS idx_articles_importance ON articles(importance);

CREATE TABLE IF NOT EXISTS kb_notes (
	id          INTEGER PRIMARY KEY AUTOINCREMENT,
	note_type   TEXT    NOT NULL,              -- atom | electron | molecule | inbox
	title       TEXT    NOT NULL,
	slug        TEXT    NOT NULL UNIQUE,       -- file name without extension
	path        TEXT    NOT NULL,              -- absolute path in the vault
	frontmatter TEXT    NOT NULL DEFAULT '{}', -- JSON (mirrors YAML frontmatter)
	content     TEXT    NOT NULL DEFAULT '',
	tags        TEXT    NOT NULL DEFAULT '[]',
	wikilinks   TEXT    NOT NULL DEFAULT '[]',
	embedding   BLOB,                          -- serialized []float32 (semantic search)
	created_at  TEXT    NOT NULL,
	updated_at  TEXT    NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_kb_notes_type ON kb_notes(note_type);
CREATE INDEX IF NOT EXISTS idx_kb_notes_slug ON kb_notes(slug);

CREATE TABLE IF NOT EXISTS rules (
	id         INTEGER PRIMARY KEY AUTOINCREMENT,
	name       TEXT    NOT NULL,
	query      TEXT    NOT NULL,               -- deterministic matcher expression
	actions    TEXT    NOT NULL DEFAULT '[]',  -- JSON: [{type,value}]
	enabled    INTEGER NOT NULL DEFAULT 1,
	created_at TEXT    NOT NULL,
	updated_at TEXT    NOT NULL
);

CREATE TABLE IF NOT EXISTS ingests (
	id         INTEGER PRIMARY KEY AUTOINCREMENT,
	ref_type   TEXT    NOT NULL,               -- article | note | email
	ref_id     TEXT    NOT NULL,               -- external id (url/guid)
	note_id    INTEGER,
	status     TEXT    NOT NULL DEFAULT 'new', -- new | processed | failed
	created_at TEXT    NOT NULL,
	UNIQUE (ref_type, ref_id)
);
`

// schemaV2 marks articles that have passed through classification so the
// pipeline does not re-classify the same batch every run.
const schemaV2 = `
ALTER TABLE articles ADD COLUMN classified INTEGER NOT NULL DEFAULT 0;
CREATE INDEX IF NOT EXISTS idx_articles_classified ON articles(classified);
`

// schemaV3 adds a semantic-search embedding to articles (notes already carry
// one from the V1 schema).
const schemaV3 = `
ALTER TABLE articles ADD COLUMN embedding BLOB;
`
