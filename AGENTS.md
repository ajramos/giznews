# AGENTS.md — convenciones para agentes de IA en giznews

Esta guía define cómo trabajar en este repo de forma consistente. Léela antes
de tocar código.

## Estructura

- **`internal/`** — lógica de negocio en Go puro (sin CGO). Paquetes por
  dominio: `sources`, `fetch`, `classify`, `digest`, `kb`, `search`,
  `extract`, `db`, `llm`, `config`.
- **`pkg/desktop/`** — API pública (DTOs + métodos con `context.Context`).
  Es la única frontera que el módulo desktop puede importar.
- **`desktop/`** — módulo Go anidado Wails (`replace → ../`). **Nunca importa
  `internal/`** (regla de Go). Frontend React + Vite + TS en
  `desktop/frontend/`.

## Comandos de verificación (siempre al terminar)

```sh
cd /Users/ajramos/Documents/dev/giznews
go build ./... && go vet ./... && go test ./...        # backend (10+ paquetes)
cd desktop && go build ./...                           # módulo Wails
cd desktop/frontend && npx tsc --noEmit                # tipos del frontend
cd desktop/frontend && npx playwright test             # e2e (44+ specs, incluye mock)
cd desktop && wails build                              # empaquetado nativo
```

Todo debe quedar verde antes de commitear.

## Reglas de arquitectura

- La lógica de negocio **siempre** en Go (`internal/`); el frontend solo
  consume `pkg/desktop` vía bindings Wails.
- `pkg/desktop` expone métodos con `context.Context`; el módulo `desktop/`
  los envuelve **sin ctx** para Wails (quirk de binding) en `desktop/app.go`.
- El frontend usa `src/api.ts` como bridge: convierte el wire shape de Wails
  (**snake_case**, p. ej. `content_md`, `llm_enabled`) a camelCase
  (`contentMD`, `llmEnabled`). **No cambies `camel()`** sin tocar también el
  contrato; está cubierto por `e2e/real-shape.spec.ts`.
- SQLite con `modernc.org/sqlite` (sin CGO). Migraciones incrementales por
  `PRAGMA user_version` en `internal/db/db.go`. Actualiza
  `TestMigrateFromV1` al añadir una migración.
- Scroll interno: los contenedores scrollables deben tener `min-height: 0`
  (flex/grid) — ver `desktop/frontend/src/styles.css`.

## Frontend (desktop/frontend)

- **`src/api.ts`** — bridge tipado + `apiMock.ts` para dev/e2e sin Wails
  (`isWails()` elige backend real o mock).
- **e2e (Playwright)** — `e2e/*.spec.ts`. `gotoApp` salta el tour de
  bienvenida. Usa `?dense=1` para listas largas (scroll/wheel). El wire shape
  real se testea en `real-shape.spec.ts`.
- Atajos de teclado y ayuda en `src/keys.ts`; nuevos atajos van ahí y se
  documentan en `?`.
- Estados de un artículo: `unread | read | archived | starred` — todos lógicos,
  nada se borra físicamente. El archivo ofrece undo.

## Base de datos / config del usuario

- Config: `~/.config/giznews/config.json` (schema en `internal/config`).
  Nombres de campo: `lenientInt`, `llm.enabled`, `extract.on_fetch/limit/
  concurrency`, `gmail.credentials_path` (compartido con giztui).
- Vault de conocimiento: `~/Documents/obsidian/chronicles-ai` (Obsidian).

## Convenciones de código

- Go: `gofmt` + `go vet` limpios; errores envueltos (`fmt.Errorf("ctx: %w", err)`).
- TS: strict; `npx tsc --noEmit` sin errores.
- Sin comentarios de relleno; los pocos comentarios deben explicar el *por qué*.
