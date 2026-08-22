# AGENTS.md — conventions for AI agents working on giznews

This guide defines how to work on this repo consistently. Read it before
touching code.

## Structure

- **`internal/`** — business logic in pure Go (no CGO). Domain packages:
  `sources`, `fetch`, `classify`, `digest`, `kb`, `search`, `extract`, `db`,
  `llm`, `config`.
- **`pkg/desktop/`** — public API (DTOs + methods with `context.Context`).
  The only boundary the desktop module may import.
- **`desktop/`** — nested Wails Go module (`replace → ../`). **Never imports
  `internal/`** (Go rule). React + Vite + TS frontend in `desktop/frontend/`.

## Verification commands (always run when done)

```sh
cd /Users/ajramos/Documents/dev/giznews
go build ./... && go vet ./... && go test ./...        # backend (10+ packages)
cd desktop && go build ./...                           # Wails module
cd desktop/frontend && npx tsc --noEmit                # frontend types
cd desktop/frontend && npx playwright test             # e2e (includes mock)
cd desktop && wails build                              # native packaging
```

Everything must be green before committing.

## Architecture rules

- Business logic **always** in Go (`internal/`); the frontend only consumes
  `pkg/desktop` via Wails bindings.
- `pkg/desktop` exposes methods with `context.Context`; the `desktop/` module
  wraps them **without ctx** for Wails (binding quirk) in `desktop/app.go`.
- The frontend uses `src/api.ts` as the bridge: it converts Wails' wire shape
  (**snake_case**, e.g. `content_md`, `llm_enabled`) to camelCase
  (`contentMD`, `llmEnabled`). **Do not change `camel()`** without updating the
  contract too; it is covered by `e2e/real-shape.spec.ts`.
- SQLite with `modernc.org/sqlite` (no CGO). Incremental migrations via
  `PRAGMA user_version` in `internal/db/db.go`. Update `TestMigrateFromV1`
  when adding a migration.
- Internal scrolling: scrollable containers must have `min-height: 0`
  (flex/grid) — see `desktop/frontend/src/styles.css`.

## Frontend (desktop/frontend)

- **`src/api.ts`** — typed bridge + `apiMock.ts` for dev/e2e without Wails
  (`isWails()` picks the real backend or the mock).
- **e2e (Playwright)** — `e2e/*.spec.ts`. `gotoApp` skips the welcome tour.
  Use `?dense=1` for long lists (scroll/wheel). The real wire shape is tested
  in `real-shape.spec.ts`.
- Keyboard shortcuts and help live in `src/keys.ts`; new shortcuts go there
  and are documented in `?`.
- Article states: `unread | read | archived | starred` — all logical, nothing
  is physically deleted. Archiving offers undo.

## Asking the vault

`search.Ask` answers from the notes only. Non-negotiables: every `[[slug]]` is
checked against `kb_notes` before the answer is shown (invented ones lose their
brackets and land in `Dropped`); articles are context but never citable; and
with nothing retrieved or no model the answer comes back `Grounded: false` with
the ranked sources — never filled in from the model's own knowledge.

Questions retrieve differently from the search box: `questionQuery` strips
question words and matches the rest as alternatives, because a phrase match on
a natural question finds nothing. Both go through `retrieve`.

## Retention

`internal/prune` is the only thing in the repo that deletes articles, and it is
staged: bodies at `prune.body_days`, whole rows at `prune.row_days`, then
VACUUM. Never prunable at any age: starred, still unread, or with a note in the
vault — and a story is pruned as a unit, since deleting its anchor would strand
the copies. Dropping a body sets `extracted = 1` so the extractor does not
re-download it, and `articles_fts` rows are deleted explicitly because the index
keeps its own copy of the text.

## Unattended runs

Stages live in `internal/pipeline` (`Runner`), so `serve` and the CLI run the
same code. `serve` schedules them (`serve.*_every`, `digest_at` in local time);
`--once` does one pass for cron and returns an error when it could not work.

Invariants: a failing stage never stops the loop, cancellation lands between
stages, and only one pipeline runs at a time — `locks` is an advisory lock that
expires on its own, with an owner unique per holder rather than per process.
Commands that mutate (`fetch`, `classify`, `kb build`) take the same lock via
`pipeline.WithLock`.

## Learning from the reader

`article_events` records what happens to an article **and who did it**
(`db.ActorUser` / `db.ActorSystem`). Anything a rule or the pipeline decides is
`system` and must never come back as taste — `SetStatus` takes the actor
explicitly for exactly that reason.

`giznews learn` turns that history into a bounded (±1) delta per source and per
tag, stored in `signals`; `classify` applies it in `settleImportance`, in this
order: model → learned delta → rule/coverage floors, so an explicit rule always
outranks an inferred habit. Read rate is computed but never acted on (the list
auto-opens what the cursor lands on). Nothing applies until `learn` has run;
`classify.learn.enabled` turns application off without discarding what was
learned. `rules suggest` proposes rules from the same data, always switched off.

## Stories

Near-duplicate articles are **grouped, not dropped**: `articles.story_id` points
at the first copy that arrived (0 = nobody else covered it), and everything
downstream works on that anchor (`storyAnchor` in `internal/db/articles.go`) —
one list row, one classification, one atom, however many outlets ran it. How
many did is an importance signal in its own right (`classify.coverage_sources`
/ `coverage_floor`), and the atom cites every outlet.

Matching is by headline tokens (`fetch.SameStory`), not simhash: one extra word
moves a simhash 11-14 bits on a headline. Simhash stays for the same document
republished. The matcher errs towards *not* grouping — a missed pair is a
visible duplicate row, a false pair hides an article.

## Classification prefilter

Rules run before the LLM: regex over `title + author + URL`, **first match
wins** (ordered by id, i.e. creation order). Actions: `category`, `importance`,
`tag`, `archive`, `keep`, `boost`.

`category`/`importance`/`tag`/`archive` resolve the article and skip the model,
which also means it gets **no summary and no entities** — so prefer `archive`
for noise over pre-classifying good articles.

`keep` and `boost` do not resolve anything. `keep` is the shield placed above
broad noise rules. `boost` is an importance **floor** applied *after* the model
(`applyFloors`), so a ★3 article keeps its summary and its entities; boosts are
collected from every matching rule (highest wins) rather than first-match, and a
boosted article is never archived. See `classify.Decide`.

Ship rules in `docs/rules/*.json` (`noise.json`, `high-value.json`) and load
them with `giznews rules import` (matched by name, idempotent); never
hand-write rows. `giznews rules test "<regex>"` and `giznews classify
--dry-run` before enabling anything that archives or boosts.

## Database / user config

- Config: `~/.config/giznews/config.json` (schema in `internal/config`).
  Field names: `lenientInt`, `llm.enabled`, `extract.on_fetch/limit/
  concurrency`, `gmail.credentials_path` (shared with giztui),
  `kb.min_occurrences/age_days/limit/theme_days` (what a graph build selects,
  when a concept earns a note, and how far back themes are clustered).
- Knowledge vault: `~/Documents/obsidian/chronicles-ai` (Obsidian). The user
  writes there too: every generated note delimits its own part with
  `<!-- giznews:begin -->` / `<!-- giznews:end -->`, and `kb_notes.content_hash`
  tells an untouched file from an edited one. Always write notes through
  `Vault.Sync` — never straight to disk, or you will destroy someone's edits.
  Notes the user wrote (no markers, no `status: generated`) are imported by
  `SyncVault` and never written back; their row carries `{"origin":"vault"}`.

## Code conventions

- Go: clean `gofmt` + `go vet`; errors wrapped (`fmt.Errorf("ctx: %w", err)`).
- TS: strict; `npx tsc --noEmit` with no errors.
- No filler comments; the few comments must explain the *why*.

## Language

- **All documentation and UI text go in English.** Never mix commands/labels
  in Spanish: `docs/`, comments, `keys.ts`, component strings, toasts, palette
  hints, and `AGENTS.md`-level notes. Spanish is only allowed when talking to
  the user, never in the repo.
- **Anything published to GitHub is English too**: commit messages, pull
  request titles and bodies, issue text, and review comments. The chat can be
  in Spanish; the permanent record cannot.

## Icons

- **No emojis as UI icons.** Use the `lucide-react` line-icon set (same visual
  language as the Heroicons giztui uses). Import the icon component and render
  it with a `size` prop. Emojis are only acceptable inside user-authored note
  *content*, never in chrome/status/buttons/tabs.
