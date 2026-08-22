# GizNews — arquitectura

Tres capas, espejo del desktop de giztui. El módulo principal es Go puro (sin
CGO); el desktop es un módulo anidado que solo cruza la frontera pública.

```
internal/services/   ← lógica de negocio (sources, fetch, classify, digest, kb, search, extract, llm)
      ▲
pkg/desktop/         ← API + DTOs JSON (context.Context, sin deps de UI)
      ▲
desktop/             ← módulo Wails anidado (replace → ../) + frontend React/Vite/TS
```

## Pipeline de noticias

```mermaid
flowchart LR
  RSS[RSS] --> FETCH
  HN[Hacker News] --> FETCH
  ARX[arXiv] --> FETCH
  GMAIL[Gmail] --> FETCH
  URL[":url manual"] --> FETCH
  FETCH[<b>Fetch</b><br/>dedup URL/simhash<br/>extract readability] --> CLASSIFY
  CLASSIFY[<b>Classify</b><br/>reglas + LLM en lotes<br/>categoría/importancia/tags] --> LIST[Lista: ★ + chips]
  CLASSIFY --> KB[<b>KB build</b><br/>atoms + electrons + molecules]
  KB --> VAULT[Obsidian vault<br/>02-Atoms · 01-Electrons · 03-Molecules]
  CLASSIFY --> DIGEST[<b>Digest</b><br/>agrupado por tema]
  KB --> SEARCH[<b>Search</b><br/>FTS5 ⊕ embeddings]
  JOBS[z: jobs en segundo plano] -.-> FETCH
  JOBS -.-> CLASSIFY
  JOBS -.-> KB
  JOBS -.-> SEARCH
```

```
fetch ──► normalize + agrupar historias (titular/simhash/URL) ──► SQLite
   │
   ├──► extract (batch, readability → markdown, cacheado en content_md)
   │
classify ──► reglas deterministas ⚡ (archive/keep/clasificar) → LLM en batch
             (categoría, importancia, tags, entidades, resumen)
   │
kb build ──► atoms (artículos) + electrons (conceptos, promovidos al superar N
             menciones históricas) + molecules (temas, agrupados por los
             conceptos que las notas nombran juntos) en el vault
   │         y refresca las puertas de entrada: Index.md, Unresolved concepts.md
   │         y la nota del día en 00-Inbox (vistas generadas, no notas del grafo)
   │
digest ──► agrupado por tema + overview LLM (se guarda en la tabla `digests`, uno por día)
   │
search ──► FTS5 (keyword) ⊕ embeddings (Ollama, coseno) con RRF
```

## Preguntarle al vault

La mitad de recuperación ya estaba —FTS5, embeddings, RRF— y devolvía una lista
ordenada de filas, que es un buscador, no una respuesta. `Ask` añade el último
paso: leer las notas y decir lo que dicen.

Tres decisiones:

- **Una pregunta no es una frase.** La caja de búsqueda manda un *phrase match*,
  que allí está bien: si escribes tres palabras esperas esas tres juntas. Una
  pregunta es lo contrario — `"what is sparse attention?"` como frase solo casa
  con una nota que literalmente lo pregunte. `questionQuery` quita las palabras
  de pregunta y casa el resto como alternativas, dejando que bm25 ordene. Los
  dos caminos comparten `retrieve`, que recibe la expresión ya construida.
- **Las notas van primero y son lo único citable.** Un artículo puede sostener
  una respuesta pero no tiene slug al que volver; el prompt se lo dice.
- **Toda cita se valida contra `kb_notes`** (`checkCitations`). Una cita
  inventada se parece exactamente a una real, así que pierde los corchetes y se
  reporta en `Dropped`. Y si no se recuperó nada, o no hay modelo, la respuesta
  vuelve con `Grounded: false` y las fuentes: un ranking vale más que una
  disculpa, y desde luego más que una respuesta sacada de la memoria del modelo.

Además, la primera pregunta construye el índice si estaba vacío: "no hay nada
sobre eso" tiene que significar eso y no "nadie ha indexado todavía".

## Vigilancias

Una vigilancia no es un concepto nuevo: es una regla con la acción `notify`. Se
escribe, se prueba con `rules test` y se importa igual que las demás, y —como
`boost`— **anota** el artículo en vez de reclamarlo, así que la cadena sigue y un
artículo puede estar vigilado y subido a la vez.

Los avisos se registran durante `classify`, que es donde corren las reglas, y
`watch_hits` tiene el artículo como clave primaria: **una vez y ya**. Ser avisado
dos veces del mismo artículo es peor que no serlo, porque enseña a ignorar el
aviso.

La entrega va por tres caminos para no depender de que la app esté abierta:
notificación del sistema (macOS, vía `osascript`; en otros sistemas no hace nada
en vez de fallar), el bloque `## Watchlist` **arriba** de la nota del día —el
vault es la entrega que nunca falla— y una sección en el digest, que es la copia
que sale de la máquina. `:watch` lista las vigilancias y lo cazado, y verlas
cuenta como que te lo han dicho.

## El digest, fuera de la base de datos

`internal/digest` renderiza a markdown y a HTML. Las dos son funciones puras del
digest: **nada lee el reloj**, porque exportar dos veces tiene que dar los mismos
bytes o no se distingue un re-export de un cambio.

El HTML es autocontenido a propósito —estilos en línea, sin `<link>`, sin `src=`,
sin script— porque tiene que renderizar igual en un móvil sin red que en la
máquina que lo escribió, y un cliente de correo tampoco va a ir a buscar una
hoja de estilos. Todo el texto pasa por `html.EscapeString`: un titular es
texto, nunca marcado.

`Save`/`Load` son ahora el camino compartido: antes solo la app de escritorio
persistía digests, y lo hacía con las claves de su propio DTO (`theme` en vez de
`name`). `Load` acepta las dos formas, así que un digest exportado hoy puede ser
uno que escribió la app hace meses; y el CLI también guarda lo que genera, que
es lo que hace que `--date` signifique algo.

El envío por SMTP vive en `mail.go` y es lo único del repo que sale de la
máquina: sin `host` y `to` configurados se niega, y nunca ocurre sin que alguien
lo pida explícitamente. El digest se guarda antes de intentar enviarlo, así que
un fallo de entrega no cuesta nada más.

## Retención

No había un solo `DELETE FROM articles` en el repo. Archivar es lógico, que es
el comportamiento correcto, y exactamente por eso el fichero solo crecía.

`internal/prune` va en etapas, de lo barato a lo destructivo:

1. **El cuerpo** (`body_days`, 180): un artículo leído pierde `content_md` y
   `content_html` y conserva fila, título y clasificación. Se le deja
   `extracted = 1` a propósito: sin eso el extractor vería un cuerpo corto y
   volvería a descargar el artículo, deshaciendo la poda y bajándose un año de
   noticias para hacerlo.
2. **La fila** (`row_days`, 365): se borra entera. `article_events` cae por
   cascada; las filas de `articles_fts` se borran a mano, porque el índice
   guarda su **propia copia del texto** y si no se quedaría con todos los bytes
   que la poda venía a liberar.
3. **`VACUUM`**, y se informa del tamaño antes y después.

Intocable a cualquier edad: marcado, sin leer, o con una nota en el vault
(`ingests`). Y una historia se poda como unidad: borrar el ancla dejando sus
copias las dejaría apuntando a una fila que ya no existe, y `storyAnchor` no
volvería a mostrarlas nunca.

## Desatendido

`giznews serve` es el bucle: cada etapa lleva su cadencia (`serve.*_every`) y el
digest una hora del día en la zona horaria de la máquina, porque un digest se lee
por la mañana y no a la hora que decida UTC. Una cadencia vacía —o ilegible— apaga
la etapa: un typo no puede convertirse en una etapa corriendo cada nanosegundo.

Tres propiedades que son la razón de que exista:

- **Una etapa que falla no se lleva a las demás.** Un feed que deja de
  actualizarse porque el modelo estaba caído es peor que un feed con un hueco en
  sus notas.
- **Parar es limpio.** `SIGINT`/`SIGTERM` cancelan el contexto y el bucle
  termina entre dos etapas, nunca en mitad de una escritura.
- **Un solo pipeline a la vez.** SQLite en WAL admite varios procesos, así que un
  daemon y un `giznews fetch` a mano ingerirían los mismos feeds. `locks` es un
  lock consultivo que **caduca solo**: un proceso que muere no deja nada
  bloqueado, y el siguiente lo toma pasado el plazo. El dueño es único por
  *titular*, no por proceso, así que dos trabajos dentro del mismo programa se
  excluyen igual que dos programas.

Las etapas viven en `internal/pipeline`, no en `cmd/`: el bucle y la persona en
la terminal ejecutan exactamente lo mismo. `--once` hace una pasada y sale, que
es lo que cron necesita —y devuelve error si no pudo hacer el trabajo, para que
cron no informe de un éxito que no ocurrió—.

## Lo que aprende de ti

Hasta ahora un artículo solo guardaba su estado final, sobreescrito: uno
archivado podía haberse leído antes o haberse tirado sin abrir, y después no
había forma de saberlo. Son veredictos opuestos, y son la señal entera. Ahora
`article_events` guarda el historial —con **quién** lo hizo: un artículo que
archivó una regla no dice nada de los gustos de nadie, y contarlo enseñaría a
las reglas a que les guste lo que ya hacen—.

`giznews learn` lee ese historial y calcula, por fuente y por tag, tres tasas:
cuánto abres, cuánto tiras sin abrir y cuánto marcas. De ahí sale un **delta
acotado (±1)** que se guarda en `signals`. No hay modelo: es una tabla que
puedes leer, discutir y apagar (`classify.learn.enabled`), y no pasa nada hasta
que ejecutas el comando al menos una vez.

Dos decisiones que conviene conocer:

- **La tasa de lectura se muestra pero no se usa.** La lista abre sola el
  artículo bajo el cursor, así que "leído" habla tanto del orden de la lista
  como del artículo. Marcar y tirar sí son actos deliberados; esos deciden.
- **El orden de la última palabra sobre la importancia** (`settleImportance`):
  primero lo que el modelo decidió, luego el delta aprendido —como mucho un
  escalón, arriba o abajo—, y por último los suelos de una regla o de la
  cobertura. Así una regla escrita a propósito siempre gana a una costumbre
  inferida, y la costumbre solo gana a la conjetura del modelo.

`giznews rules suggest` va un paso más allá: una fuente que tiras sin abrir el
85% de las veces se propone como regla `archive` —**apagada**— para que pase por
el mismo camino de revisión que una escrita a mano.

## Historias, no recortes

Seis medios cubriendo el mismo lanzamiento no son seis artículos, y tampoco uno:
son **una historia con seis copias**, y cuántos la recogieron es la señal de
importancia más fuerte que produce el feed. Antes las copias se descartaban en el
ingest, así que esa señal se destruía antes de que nadie pudiera verla.

Ahora se guardan todas. `articles.story_id` apunta a la primera copia que llegó
—0 significa que nadie más la cubrió— y todo lo de aguas abajo trabaja sobre esa
primera copia (`storyAnchor`): la lista muestra una fila, el clasificador ve un
artículo, el vault escribe un atom. Una historia cuesta una unidad de atención,
la cubran los medios que la cubran, pero el atom cita a todos.

El emparejado **no** usa el simhash. El simhash es para documentos: en un titular
de nueve palabras, añadir una sola (`… today`, `… rules`) lo mueve 11-14 bits,
mucho más allá de cualquier umbral que no junte también historias distintas — y
dos redacciones jamás escriben las mismas palabras. Lo que sí comparten dos
crónicas del mismo hecho son los sustantivos, así que `SameStory` compara los
*conjuntos de palabras* del titular (Jaccard, sin las palabras que todo titular
tiene) con dos guardas: un mínimo de palabras compartidas —"Apple lleva la IA al
iPhone" y "…al iPad" se solapan igual que una reescritura real y difieren en la
única palabra que importa— y que las versiones coincidan (GPT-5 no es GPT-4.5).
El simhash se queda para lo que sí sabe hacer: el mismo documento republicado.

Ante la duda, no agrupa: perder un par cuesta una fila duplicada que el lector ve
e ignora; agrupar de más esconde un artículo detrás de otro, donde nadie lo va a
buscar.

## El prefiltro determinista (⚡)

Antes del clasificador caro, cada artículo pasa por las reglas: un regex —
case-insensitive, casado contra `título + autor + URL`— y unas acciones. **Gana
la primera regla que casa**, en orden de id, o sea de creación: el orden *es*
la lógica.

Acciones: `category`, `importance`, `tag`, `archive`, `keep` y `boost`. Las
cuatro primeras resuelven el artículo y **se saltan el LLM**, lo cual tiene un
coste que conviene tener presente: ese artículo se queda sin `summary` y sin
entidades, así que si acaba en el vault su atom no tiene resumen y aporta menos
conceptos. Por eso el set de ruido es casi todo `archive`: matar ruido es
ganancia pura; pre-clasificar es un intercambio.

`keep` existe justamente por el "gana la primera": no aplica nada y manda el
artículo al modelo igual. Es el escudo que se pone **por encima** de las reglas
de ruido para decir qué no pueden tocar — sin él, un regex amplio ("cualquier
cosa sobre cripto") se lleva por delante el único artículo de cripto que
importaba.

`boost` es lo contrario y por el mismo motivo: un suelo de importancia que se
aplica **después** de que el modelo haya clasificado (`applyFloors`), no en su
lugar. Marcar lo bueno con un `importance` normal sería justo al revés —
precisamente los artículos que merecen ★3 son los que más falta les hace un
resumen y unas entidades, porque son los que acaban en la base de conocimiento.
Los boosts no son "primera que gana": se recogen de todas las reglas que casan
(gana el más alto), y un artículo con boost **nunca se archiva**. Todo esto vive
en `classify.Decide`.

Las reglas viven en ficheros versionados (`docs/rules/noise.json`,
`docs/rules/high-value.json`) y se cargan con `giznews rules import <fichero>`,
que empareja por nombre: importar dos veces no duplica nada. `giznews rules test "<regex>"` dice qué artículos de tu
base casarían —la única forma honesta de escribir un regex que archiva— y
`giznews classify --dry-run` dice qué reclamaría cada regla y cuántos quedan
para el modelo, sin tocar nada.

## El flujo de lectura

- **Lista** no leídos con importancia (★ 0-3), chips por categoría, estados.
- **Lazy loading**: al navegar con `j/k` el artículo bajo el cursor carga solo
  (debounce 120 ms) y se prefetchan los adyacentes.
- Extracción **a demanda** si el cuerpo no se extrajo en batch.
- Grafo SVG force-directed con la nota del artículo y sus vecinos (2-hop).

## Contrato de serialización (importante)

Wails serializa las respuestas con `encoding/json` → los **json tags**
(snake_case): `content_md`, `source_name`, `llm_enabled`. El frontend mapea a
camelCase en `src/api.ts#camel()` (acrónimos incluidos: `content_md` →
`contentMD`, `content_html` → `contentHTML`). Los argumentos struct (p. ej.
`ListArticlesOptions`) se envían de vuelta a snake_case (`toSnakeArgs`).

`e2e/real-shape.spec.ts` simula ese wire shape y valida el contrato.

## Base de datos

SQLite con `modernc.org/sqlite` (sin CGO), WAL, `MaxOpenConns(1)`. Migraciones
incrementales por `PRAGMA user_version` en `internal/db/db.go`:
1. schema base (sources, articles, kb_notes, rules, ingests)
2. `articles.classified`
3. `articles.embedding`
4. `sources.hidden` (borrado lógico de fuentes)
5. `articles.extracted` (extracción en batch)
6. `digests` (digest diario persistido, uno por fecha)
7. `kb_links` + `concepts`/`concept_mentions` (grafo relacional: una fila por
   arista y un concepto con sus menciones acumuladas entre ejecuciones; la
   migración rellena ambas desde los `wikilinks` ya escritos)
8. `concepts.canon_key` + `concept_aliases` (una misma idea escrita de varias
   formas —"Open AI"/"OpenAI"— es un solo concepto; los alias explícitos
   cubren lo que la regla no deduce)
9. `kb_notes.content_hash` (huella del fichero que giznews escribió por última
   vez; es lo que distingue una nota intacta de una editada por el usuario)
10. `concepts.definition` + `definition_key` (la prosa del concepto y de qué se
    escribió, para no volver a pedirla en cada build)
11. `kb_themes` (el tema encontrado por el clustering: su concepto semilla, sus
    conceptos, su nota y el resumen con la key de lo que se escribió)

Nota: la numeración real en `db.go` es v7 `articles.starred`, v8 el grafo
relacional, v9 los alias, v10 el hash, v11 las definiciones y v12 los temas — el
orden de merge, no el de diseño.

## El contrato del vault

El vault es un directorio en el que el usuario también escribe, así que una nota
generada delimita la parte que giznews mantiene:

```
---
<frontmatter: las claves de giznews + las que añadas tú>
---
<lo que escribas aquí se respeta>
<!-- giznews:begin -->
… región regenerada en cada build …
<!-- giznews:end -->
<y lo que escribas aquí también>
```

`kb_notes.content_hash` guarda la huella del fichero que giznews escribió. Al
reescribir (`Vault.Sync`):

- fichero ausente → se crea;
- huella igual a la guardada (nadie lo tocó) → se reemplaza entero;
- huella distinta pero con marcadores → **merge**: se refresca solo la región y
  se conservan tu texto, tus propiedades del frontmatter (copiadas literales) y
  tus tags;
- huella distinta y sin marcadores → no se toca; se registra en el log.

Los atoms se refrescan cuando su artículo cambia (`ListStaleNotes`), no una sola
vez al crearse.

Un **electron** no es una lista de backlinks: lleva una definición (escrita por
el LLM cuando está disponible, cacheada en `concepts.definition` y regenerada
solo cuando cambian las notas que la sustentan), la línea temporal de menciones
por mes, los conceptos con los que comparte notas —co-ocurrencia real, no tags—
y sus fuentes.

Una **molecule** es un tema: el grupo de notas que insisten en nombrar los
mismos conceptos juntos. El clustering se hace sobre el grafo de conceptos, no
sobre embeddings —dos notas están juntas porque comparten conceptos, y eso el
grafo ya lo sabe exacto, offline y gratis—. Cada tema se ancla en un concepto
semilla (`kb_themes.seed`), que es lo que le da continuidad: la pertenencia se
recalcula en cada run, pero la nota conserva su slug (`theme-<semilla>`) y su
fecha de creación, así que un rebuild sin cambios no reescribe nada. Una nota
entra en el tema si nombra **al menos dos** de sus conceptos; con uno solo ya
está listada en el electron de ese concepto y la molecule no añadiría nada. La
nota lleva idea central (LLM cuando hay, cacheada en `kb_themes.summary` con su
key), el hilo cronológico de las notas y los conceptos que lo sostienen — nunca
una sección vacía. `kb synth <categoría>` sigue existiendo para el corte manual
por categoría.

Los parámetros del build viven en `kb` dentro de `config.json`
(`min_occurrences`, `age_days`, `limit`, `theme_days`; la importancia mínima
sigue en `classify.importance_threshold`), y `kb build --dry-run` los imprime
junto con lo que el run escribiría — atoms, conceptos que graduarían, temas que
agruparía y notas que refrescaría — sin tocar nada.

Orden dentro de un build, que importa: primero se importan tus notas, luego se
escriben los atoms (que crean conceptos), luego se cuentan las menciones de tus
notas —así una nota tuya puede nombrar algo antes que ningún artículo—, luego se
promueve, y solo al final se agrupan los temas, sobre el grafo ya movido.

Y el camino de vuelta: `kb sync` (también al principio de cada `kb build`) lee
las notas que escribes tú. Un fichero sin marcadores y sin `status: generated`
no es de giznews, así que se importa a `kb_notes` — título, tags, wikilinks — y
sus enlaces a conceptos que ya existen cuentan como menciones, de modo que tus
notas pesan en la promoción igual que los artículos. **Nunca se reescribe**: tu
fichero manda y la base de datos solo guarda una copia y su huella. Un concepto
puede cruzar el umbral sin que ningún artículo del run lo nombre, así que el
build también barre la cola de pendientes.

## Desktop (Wails)

- `desktop/main.go` — bootstrap vía `pkg/desktop.OpenApp()` (nunca `internal/`).
- `desktop/app.go` — wrappers **sin ctx** (Wails no inyecta ctx de forma fiable)
  sobre los métodos de `pkg/desktop`.
- `desktop/frontend/` — React 18 + Vite + TS. `apiMock.ts` da un backend en
  memoria para dev/e2e; `isWails()` elige real vs mock.
- Drag de ventana: `--wails-draggable: drag` en la topbar (custom property de
  Wails v2, no `-webkit-app-region`). Popovers con `position: fixed` via portal
  para no heredar la región drag.

## Temas

Tokens CSS (`--bg`, `--accent`, `--sel-bg`, `--cat-*`, …) por
`[data-theme]` en `desktop/frontend/src/theme.ts` y `styles.css`:
GizNews Dark, Dracula, Slate Blue, Nord, Light.
