# GizNews

Un lector de noticias de IA que se maneja como **vim** y construye tu
**knowledge graph** sobre la marcha. Desktop nativo (macOS) que agrega,
clasifica, digiere y conecta las noticias de IA, y escribe el conocimiento
como notas Zettelkasten en un vault compatible con Obsidian.

## Lo que hace

- **Agrega** noticias de RSS/Atom, Hacker News, arXiv y newsletters de Gmail
  con dedupe por simhash (una historia repetida en varias fuentes cuenta una).
- **Clasifica** con reglas deterministas ⚡ (instantáneas) + LLM en batch:
  categoría, importancia (0-3), tags, entidades y resumen por artículo.
- **Digiere**: un *digest diario* por temas con overview escrito por la IA.
- **Construye el knowledge graph**: cada artículo destacado se convierte en una
  nota **Atom**, los conceptos recurrentes en **Electrons**, y las síntesis en
  **Molecules**, con `[[wikilinks]]` y frontmatter — listo para Obsidian.
- **Busca semánticamente**: embeddings locales (Ollama) + FTS5 fusionados con
  RRF, 100% sin servicios en la nube.
- **Extrae el contenido completo** de cada artículo (readability → markdown) en
  batch durante `fetch`, cacheado para lectura instantánea.

## Interfaz

App desktop nativa (Go + Wails + React), keyboard-first, con la gramática de
giztui:

- `j/k` navegar (con conteo: `5j`) · el artículo se **carga solo** al pasar.
- `y` resumen IA · `a` archivar (con deshacer) · `m` destacar · `t` leído.
- `s` búsqueda · `g` grafo · `d` digest · `:` command palette · `?` ayuda.
- `v` selección múltiple · `u/r/x/*` vistas por estado · `q` salir.
- **Pickers** keyboard-first para fuentes (`:sources`), conceptos
  (`:concepts` — promover uno pendiente con `Enter`, fundir dos grafías con
  `m`) y temas.
- 5 temas (GizNews Dark, Dracula, Slate Blue, Nord, Light).

## Quick start

Requisitos: Go 1.25+, Node 20+, [Wails](https://wails.io) CLI, y
[Ollama](https://ollama.com) con un modelo (`gemma4:12b-mlx` o similar) y
`nomic-embed-text`.

```sh
# 1. Inicializa config, DB y el vault de conocimiento
go run ./cmd/giznews init

# 2. Añade fuentes (RSS, HN, arXiv, gmail)
go run ./cmd/giznews sources add --name "HN RSS" --url https://news.ycombinator.com/rss
go run ./cmd/giznews sources add --name "DeepMind" --url https://deepmind.google/blog/rss.xml --group labs

# 3. Pipeline completo
go run ./cmd/giznews fetch      # + extrae cuerpos en batch
go run ./cmd/giznews classify   # reglas ⚡ + LLM
go run ./cmd/giznews kb build --dry-run  # qué haría, sin tocar el vault
go run ./cmd/giznews kb build   # genera atoms/electrons + Index.md en el vault
go run ./cmd/giznews kb sync            # importa al grafo tus notas del vault
go run ./cmd/giznews kb concepts        # conceptos por menciones (electron | pending)
go run ./cmd/giznews kb merge gpt4 gpt-5  # funde dos conceptos y reescribe los enlaces
go run ./cmd/giznews digest     # digest diario

# 4. Búsqueda semántica
go run ./cmd/giznews search "watermarking"

# 5. App desktop
cd desktop && wails build && open build/bin/giznews.app
```

El vault se abre con Obsidian en `~/Documents/obsidian/chronicles-ai`
(configurable en `~/.config/giznews/config.json`).

## Arquitectura

```
internal/services/   ← lógica de negocio (fetch, classify, digest, kb, search, extract)
      ▲
pkg/desktop/         ← API + DTOs JSON (capa pura, unit-tested)
      ▲
desktop/             ← módulo Wails anidado (replace → ../) + frontend React/Vite
```

- Backend en **Go puro** (sin CGO: `modernc.org/sqlite`), testable.
- El frontend usa un **mock backend** para desarrollo y e2e (Playwright) sin
  Wails; el wire shape real de Wails (snake_case) se verifica aparte.
- Ver [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) para el detalle.

## Desarrollo

```sh
make test        # go test ./...
make test-ui     # tsc + e2e (Playwright, vite dev + mock)
make build       # wails build → desktop/build/bin/GizNews.app
make install     # build + copia a /Applications
```

## Licencia

MIT — ver [LICENSE](LICENSE).
