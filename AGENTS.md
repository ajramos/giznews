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

## Database / user config

- Config: `~/.config/giznews/config.json` (schema in `internal/config`).
  Field names: `lenientInt`, `llm.enabled`, `extract.on_fetch/limit/
  concurrency`, `gmail.credentials_path` (shared with giztui).
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

## Icons

- **No emojis as UI icons.** Use the `lucide-react` line-icon set (same visual
  language as the Heroicons giztui uses). Import the icon component and render
  it with a `size` prop. Emojis are only acceptable inside user-authored note
  *content*, never in chrome/status/buttons/tabs.
