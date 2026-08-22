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
	migrations := []string{schemaV1, schemaV2, schemaV3, schemaV4, schemaV5, schemaV6, schemaV7, schemaV8, schemaV9, schemaV10, schemaV11, schemaV12, schemaV13, schemaV14, schemaV15, schemaV16, schemaV17}

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

// schemaV4 soft-deletes sources: a hidden source stops appearing in the UI but
// its articles are preserved (no FK violation, fully reversible).
const schemaV4 = `
ALTER TABLE sources ADD COLUMN hidden INTEGER NOT NULL DEFAULT 0;
CREATE INDEX IF NOT EXISTS idx_sources_hidden ON sources(hidden);
`

// schemaV5 tracks which articles have had their full content extracted, so
// batch extraction during fetch only targets short, un-extracted bodies.
const schemaV5 = `
ALTER TABLE articles ADD COLUMN extracted INTEGER NOT NULL DEFAULT 0;
CREATE INDEX IF NOT EXISTS idx_articles_extracted ON articles(extracted);
`

// schemaV6 stores generated daily digests (one per day) so past digests can be
// re-opened later without re-running the LLM.
const schemaV6 = `
CREATE TABLE IF NOT EXISTS digests (
	id         INTEGER PRIMARY KEY AUTOINCREMENT,
	date       TEXT    NOT NULL UNIQUE,        -- YYYY-MM-DD
	overview   TEXT    NOT NULL DEFAULT '',
	themes     TEXT    NOT NULL DEFAULT '[]',  -- JSON array of DigestThemeDTO
	created_at TEXT    NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_digests_date ON digests(date);
`

// schemaV7 makes "starred" an orthogonal triage flag (separate from the
// unread/read/archived status), so a read article can also be starred. Any
// existing "starred" rows become "read + starred".
const schemaV7 = `
ALTER TABLE articles ADD COLUMN starred INTEGER NOT NULL DEFAULT 0;
UPDATE articles SET starred = 1, status = 'read' WHERE status = 'starred';
CREATE INDEX IF NOT EXISTS idx_articles_starred ON articles(starred);
`

// schemaV8 turns the knowledge graph into first-class relational data.
//
// Until now edges lived inside the kb_notes.wikilinks JSON blob, which could
// only be queried with substring LIKE (imprecise, unindexed), and concepts had
// no existence of their own: a build aggregated them in memory and forgot them,
// so a concept mentioned once per run never reached the promotion threshold.
//
// kb_links stores one row per edge; concepts and concept_mentions accumulate
// every mention across runs. The backfill recovers both from what the vault
// already wrote — including the concepts that were only ever dangling links,
// which is where the lost mentions were hiding.
const schemaV8 = `
CREATE TABLE IF NOT EXISTS kb_links (
	from_note  INTEGER NOT NULL REFERENCES kb_notes(id) ON DELETE CASCADE,
	to_slug    TEXT    NOT NULL,               -- may not exist yet (dangling link)
	kind       TEXT    NOT NULL DEFAULT 'wikilink',
	created_at TEXT    NOT NULL,
	PRIMARY KEY (from_note, to_slug)
);
CREATE INDEX IF NOT EXISTS idx_kb_links_to ON kb_links(to_slug);

CREATE TABLE IF NOT EXISTS concepts (
	slug       TEXT    PRIMARY KEY,
	name       TEXT    NOT NULL,
	note_id    INTEGER REFERENCES kb_notes(id) ON DELETE SET NULL, -- electron once promoted
	mentions   INTEGER NOT NULL DEFAULT 0,
	first_seen TEXT    NOT NULL,
	last_seen  TEXT    NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_concepts_mentions ON concepts(mentions DESC);

CREATE TABLE IF NOT EXISTS concept_mentions (
	concept_slug TEXT    NOT NULL,
	note_id      INTEGER NOT NULL REFERENCES kb_notes(id) ON DELETE CASCADE,
	created_at   TEXT    NOT NULL,
	PRIMARY KEY (concept_slug, note_id)
);
CREATE INDEX IF NOT EXISTS idx_concept_mentions_slug ON concept_mentions(concept_slug);

-- Backfill: every wikilink already stored becomes an edge.
INSERT OR IGNORE INTO kb_links (from_note, to_slug, kind, created_at)
SELECT n.id, je.value, 'wikilink', n.created_at
FROM kb_notes n, json_each(n.wikilinks) je
WHERE je.value != '';

-- Backfill: existing electrons are promoted concepts.
INSERT OR IGNORE INTO concepts (slug, name, note_id, mentions, first_seen, last_seen)
SELECT n.slug, n.title, n.id, 0, n.created_at, n.updated_at
FROM kb_notes n WHERE n.note_type = 'electron';

-- Backfill: a link to a slug no note owns is a concept that never graduated.
INSERT OR IGNORE INTO concepts (slug, name, note_id, mentions, first_seen, last_seen)
SELECT l.to_slug, l.to_slug, NULL, 0, MIN(l.created_at), MAX(l.created_at)
FROM kb_links l
LEFT JOIN kb_notes n ON n.slug = l.to_slug
WHERE n.id IS NULL
GROUP BY l.to_slug;

-- Backfill: mentions are the atoms pointing at each concept.
INSERT OR IGNORE INTO concept_mentions (concept_slug, note_id, created_at)
SELECT l.to_slug, l.from_note, l.created_at
FROM kb_links l
JOIN concepts c ON c.slug = l.to_slug
JOIN kb_notes n ON n.id = l.from_note
WHERE n.note_type = 'atom';

UPDATE concepts SET mentions = (
	SELECT COUNT(*) FROM concept_mentions m WHERE m.concept_slug = concepts.slug
);
`

// schemaV9 lets one concept answer to several spellings.
//
// The news stream names the same thing many ways — "OpenAI" and "Open AI",
// "GPT-5" and "GPT5" — and each spelling used to become its own concept, so a
// topic's mentions were split across near-duplicate slugs and none of them
// reached the promotion threshold. canon_key is the slug with its separators
// removed, which collapses those variants automatically; concept_aliases holds
// the merges that no rule can derive, made explicit by the user.
const schemaV9 = `
ALTER TABLE concepts ADD COLUMN canon_key TEXT NOT NULL DEFAULT '';
UPDATE concepts SET canon_key = replace(
	CASE WHEN slug LIKE '%-concept' THEN substr(slug, 1, length(slug) - 8) ELSE slug END, '-', '');
CREATE INDEX IF NOT EXISTS idx_concepts_canon ON concepts(canon_key);

CREATE TABLE IF NOT EXISTS concept_aliases (
	alias      TEXT PRIMARY KEY,               -- slug that redirects
	canonical  TEXT NOT NULL,                  -- slug it redirects to
	created_at TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_concept_aliases_canonical ON concept_aliases(canonical);
`

// schemaV10 remembers what giznews last wrote to each note file.
//
// The vault is a directory the user also writes in. Until now every rebuild
// overwrote the file whole, so a paragraph added in Obsidian was gone the next
// time the article was re-classified. Comparing the file on disk against this
// hash is what tells an untouched note (safe to replace) from an edited one
// (whose text has to be preserved).
const schemaV10 = `
ALTER TABLE kb_notes ADD COLUMN content_hash TEXT NOT NULL DEFAULT '';
`

// schemaV11 lets a concept keep the definition written for it.
//
// An Electron was a list of backlinks under a fixed sentence; a definition is
// worth asking a model for, but not worth asking again on every build. The key
// records what the definition was written from, so it is regenerated when the
// notes behind it change and reused when they have not.
const schemaV11 = `
ALTER TABLE concepts ADD COLUMN definition TEXT NOT NULL DEFAULT '';
ALTER TABLE concepts ADD COLUMN definition_key TEXT NOT NULL DEFAULT '';
`

// schemaV12 remembers the themes a build found.
//
// A molecule is a cluster of notes that keep naming the same concepts
// together. The cluster is recomputed on every run, but its identity — the
// concept it is anchored on — has to survive one, or a theme would get a new
// note every time its second-largest concept overtook its third. The summary
// is kept for the same reason a definition is: it costs a model call, and the
// key says what it was written from.
const schemaV12 = `
CREATE TABLE IF NOT EXISTS kb_themes (
	slug        TEXT    PRIMARY KEY,
	title       TEXT    NOT NULL,
	seed        TEXT    NOT NULL DEFAULT '',   -- the concept the theme is anchored on
	concepts    TEXT    NOT NULL DEFAULT '[]', -- JSON array of concept slugs
	summary     TEXT    NOT NULL DEFAULT '',
	summary_key TEXT    NOT NULL DEFAULT '',
	note_id     INTEGER REFERENCES kb_notes(id) ON DELETE SET NULL,
	created_at  TEXT    NOT NULL,
	updated_at  TEXT    NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_kb_themes_seed ON kb_themes(seed);
`

// schemaV13 groups near-duplicate articles into stories.
//
// Cross-source duplicates used to be dropped at ingest, which threw away the
// strongest importance signal the feed produces: six outlets covering the same
// launch within two hours *is* the news. Now every copy is kept and points at
// the article that arrived first — story_id 0 means an article nobody else
// covered, a story of one.
const schemaV13 = `
ALTER TABLE articles ADD COLUMN story_id INTEGER NOT NULL DEFAULT 0;
CREATE INDEX IF NOT EXISTS idx_articles_story ON articles(story_id);
`

// schemaV14 records what the reader does, and what a run of the pipeline
// decided on its own.
//
// Until now an article kept only its final state, overwritten in place: an
// archived one could have been read first or thrown away unopened, and there
// was no way to tell afterwards. Those are opposite verdicts, and they are the
// whole signal. The actor column keeps them honest — an article a prefilter
// rule archived says nothing about anyone's taste.
//
// signals holds what has been learned from those events, so a classification
// run reads a number instead of recomputing a history.
const schemaV14 = `
CREATE TABLE IF NOT EXISTS article_events (
	id         INTEGER PRIMARY KEY AUTOINCREMENT,
	article_id INTEGER NOT NULL REFERENCES articles(id) ON DELETE CASCADE,
	event      TEXT    NOT NULL,              -- read | archived | unread | starred | unstarred
	actor      TEXT    NOT NULL DEFAULT 'user', -- user | system
	created_at TEXT    NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_article_events_article ON article_events(article_id);
CREATE INDEX IF NOT EXISTS idx_article_events_event ON article_events(event, actor);

CREATE TABLE IF NOT EXISTS signals (
	kind       TEXT    NOT NULL,              -- source | tag
	key        TEXT    NOT NULL,              -- source id, or the tag itself
	label      TEXT    NOT NULL DEFAULT '',   -- what to call it in a report
	samples    INTEGER NOT NULL DEFAULT 0,
	read_rate  REAL    NOT NULL DEFAULT 0,
	drop_rate  REAL    NOT NULL DEFAULT 0,    -- archived without ever opening it
	star_rate  REAL    NOT NULL DEFAULT 0,
	delta      INTEGER NOT NULL DEFAULT 0,    -- importance adjustment, bounded
	updated_at TEXT    NOT NULL,
	PRIMARY KEY (kind, key)
);
`

// schemaV15 keeps two copies of giznews from doing the same work twice.
//
// The database allows several processes at once (WAL), so a daemon fetching on
// a schedule and someone running `giznews fetch` by hand would both ingest the
// same feeds and both write the same notes. The lock is advisory and expires on
// its own: a process that dies takes nothing with it.
const schemaV15 = `
CREATE TABLE IF NOT EXISTS locks (
	name       TEXT    PRIMARY KEY,
	owner      TEXT    NOT NULL,
	acquired_at TEXT   NOT NULL,
	expires_at TEXT    NOT NULL
);
`

// schemaV16 remembers how each source's last few fetches went.
//
// Until now `last_fetch` recorded a completed fetch and nothing about whether it
// worked. A feed that starts 404ing or quietly returns an empty document just
// stops contributing, and nobody is told. `last_error` is the message of the
// last failed fetch, `last_ok` when it last brought something in, and the two
// counters say whether the streak is worth acting on: consecutive HTTP failures,
// and cycles that succeeded but returned nothing.
const schemaV16 = `
ALTER TABLE sources ADD COLUMN last_error TEXT NOT NULL DEFAULT '';
ALTER TABLE sources ADD COLUMN last_ok TEXT;
ALTER TABLE sources ADD COLUMN consecutive_failures INTEGER NOT NULL DEFAULT 0;
ALTER TABLE sources ADD COLUMN empty_cycles INTEGER NOT NULL DEFAULT 0;
`

// schemaV17 remembers what a watch caught.
//
// Everything else in giznews is pull: the news is there when you next open it,
// whether it arrived a minute or three days ago. For the handful of things a
// reader is actually waiting on, that is the wrong shape. A hit is recorded
// once per article and never again — the primary key is the article, so a
// re-fetch or a second classify cannot re-announce something you have already
// been told about.
const schemaV17 = `
CREATE TABLE IF NOT EXISTS watch_hits (
	article_id INTEGER PRIMARY KEY REFERENCES articles(id) ON DELETE CASCADE,
	rule       TEXT    NOT NULL,
	seen       INTEGER NOT NULL DEFAULT 0, -- the reader has been shown this one
	created_at TEXT    NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_watch_hits_seen ON watch_hits(seen, created_at DESC);
`
