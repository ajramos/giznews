# GizNews

Un lector de noticias de IA que se maneja como **vim** y construye tu
**knowledge graph** sobre la marcha. Desktop nativo (macOS) que agrega,
clasifica, digiere y conecta las noticias de IA, y escribe el conocimiento
como notas Zettelkasten en un vault compatible con Obsidian.

## Lo que hace

- **Agrega** noticias de RSS/Atom, Hacker News, arXiv y newsletters de Gmail,
  agrupando en **una historia** lo que varios medios cuentan a la vez — cuántos
  la cubrieron es la señal de importancia más fuerte del feed.
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
go run ./cmd/giznews fetch      # + extrae cuerpos en batch, agrupa historias
go run ./cmd/giznews rules import docs/rules/noise.json       # prefiltro: mata el ruido
go run ./cmd/giznews rules import docs/rules/high-value.json  # y marca lo que va a ★3
go run ./cmd/giznews classify --dry-run  # qué reclama cada regla, sin clasificar nada
go run ./cmd/giznews classify   # reglas ⚡ + LLM
go run ./cmd/giznews learn --dry-run     # qué dice tu forma de leer, sin guardar nada
go run ./cmd/giznews learn               # lo guarda: ajusta importancia de ahí en adelante
go run ./cmd/giznews rules suggest       # reglas propuestas desde tu historial (llegan apagadas)
go run ./cmd/giznews kb build --dry-run  # qué haría, sin tocar el vault
go run ./cmd/giznews kb build   # genera atoms/electrons/molecules + Index.md en el vault
go run ./cmd/giznews kb themes          # reagrupa los temas (lo hace también el build)
go run ./cmd/giznews kb sync            # importa al grafo tus notas del vault
go run ./cmd/giznews kb concepts        # conceptos por menciones (electron | pending)
go run ./cmd/giznews kb merge gpt4 gpt-5  # funde dos conceptos y reescribe los enlaces
go run ./cmd/giznews digest     # digest diario

# 4. Desatendido
go run ./cmd/giznews serve          # bucle: fetch → classify → kb → index → digest
go run ./cmd/giznews serve --once   # una pasada y sale (para cron)

# 5. Búsqueda semántica y preguntas
go run ./cmd/giznews search "watermarking"
go run ./cmd/giznews ask "¿qué sé de sparse attention?"   # responde citando tus notas

# 6. App desktop
cd desktop && wails build && open build/bin/giznews.app
```

El vault se abre con Obsidian en `~/Documents/obsidian/chronicles-ai`
(configurable en `~/.config/giznews/config.json`).

## Dejarlo funcionando solo

`giznews serve` mantiene el pipeline al día: cada etapa tiene su cadencia y el
digest su hora del día (`serve` en `config.json`; una cadencia vacía apaga esa
etapa). Una etapa que falla se registra y **el bucle sigue**: que el modelo esté
caído no puede impedir que el feed siga trayendo noticias. `Ctrl-C` o `SIGTERM`
salen entre dos etapas, nunca a mitad de una escritura.

Un solo proceso a la vez: el pipeline toma un lock (que caduca solo, así que un
proceso muerto no bloquea nada) y los comandos manuales lo respetan.

```jsonc
"serve": {
  "fetch_every": "30m",
  "classify_every": "30m",
  "kb_every": "6h",
  "index_every": "12h",
  "digest_at": "08:00"      // hora local
}
```

Si prefieres cron o launchd, `--once` hace una pasada y sale:

```sh
# cron: cada media hora
*/30 * * * * /usr/local/bin/giznews serve --once >> ~/.local/state/giznews.log 2>&1
```

```xml
<!-- launchd: ~/Library/LaunchAgents/com.giznews.pipeline.plist -->
<dict>
  <key>Label</key><string>com.giznews.pipeline</string>
  <key>ProgramArguments</key>
  <array><string>/usr/local/bin/giznews</string><string>serve</string><string>--once</string></array>
  <key>StartInterval</key><integer>1800</integer>
</dict>
```

## Preguntarle a tus notas

`giznews ask "…"` (y `:ask` en la app) responde con la prosa del modelo pero
**solo con lo que dicen tus notas**, citando cada afirmación con `[[slug]]`. En
la app cada cita es un botón que abre esa nota.

Dos reglas que son la razón de que se pueda confiar en la respuesta:

- **Toda cita se comprueba contra la base de datos.** Una cita inventada se
  parece exactamente a una real, así que se le quitan los corchetes antes de que
  llegue a nadie y se informa de cuáles fueron. La promesa entera es poder
  seguir una afirmación hasta la nota de la que salió.
- **Si no hay nada, no responde.** Sin notas relevantes, o sin modelo, devuelve
  el ranking y lo dice — nunca rellena el hueco con lo que el modelo sabe por su
  cuenta.

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
