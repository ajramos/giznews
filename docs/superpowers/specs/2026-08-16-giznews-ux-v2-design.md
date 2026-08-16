# GizNews UX v2 — Three-pane master-detail + sustainable knowledge loop

**Date:** 2026-08-16
**Status:** draft (approved for mockup iteration)

## 1. Goal

GizNews is a personal AI-intelligence system: read the signal with minimal
friction, capture what matters, and watch a knowledge graph grow from it —
sustainably, with little effort. This design reworks the user experience around
that loop with a single visual language across the two worlds (news and
knowledge).

## 2. Current problems (diagnosis)

1. **Typography is inconsistent.** Body is sans `14px/1.5`, but much of the
   chrome (`cmd-name`, `sp-meta`, `job-status`, `help-row`, `vd-flow`,
   `flow-line`) uses `--font-mono` arbitrarily, and font sizes are ad-hoc (11,
   12, 12.5, 13, 13.5, 14, 15, 19, 21, 22). No type scale.
2. **The vault does not auto-load the reader.** Entering the vault leaves the
   detail column empty or stale; the selected item is not opened.
3. **No keyboard scroll in the vault reader.** `VaultBrowser` captures `j/k`
   and arrows for the list; the note can only be paged with `Space` or the
   mouse. There is no reader-focus model like the news world has.
4. **The vault does not explain its organization.** Three stage tabs with no
   legend of what each stage means.
5. **Top-bar stat chips** (`unread`, `notes`) have no clear role and sit
   awkwardly beside the brand.
6. **Inconsistent iconography.** In the news list icons are "contained"
   (framed chips), in the top bar they are bare pills.
7. **Bulk mode is poor.** `v` auto-selects the current item and there is no
   per-row checkbox — only a background tint, so selection state is unclear.

## 3. Target structure: three panes

Inspired by Feedly (feed → list → reader), Zotero (collections → items →
detail), and Obsidian (graph, backlinks, atomic notes).

```
┌──────────────┬─────────────────────────────┬────────────────┐
│ Pane 1       │ Pane 2                      │ Pane 3         │
│ master list  │ reader (article or note)    │ context        │
│ ~320px       │ flexible, reading measure   │ ~300px,        │
│              │ (~68ch)                     │ collapsible    │
└──────────────┴─────────────────────────────┴────────────────┘
```

- **Pane 1 (list)** is the master: in news, the article list with views +
  filters; in vault, the stages + note list.
- **Pane 2 (reader)** is the detail: the article reader or the note reader,
  with a bounded reading measure.
- **Pane 3 (context)** shows the current item's relationships — the bridge
  between news and knowledge ("assimilation" made visible).
- Panes are resizable (splitters); the context pane is collapsible via a key.

## 4. Design system (tokens)

### 4.1 Type scale

| Token | Size | Use |
|-------|------|-----|
| `xs`  | 11px | metadata, chips, timestamps, counts |
| `sm`  | 12px | secondary text |
| `md`  | 13px | list item titles |
| `base`| 14px | reading body |
| `lg`  | 16px | reader section heads |
| `xl`  | 20px | article title |
| `2xl` | 24px | world / digest titles |

Rule: **sans** for reading, titles and UI; **mono only for data** (counts,
dates, ids, shortcuts, code). Never mono for content titles.

### 4.2 Icons

- `lucide-react` only, fixed sizes (12 / 13 / 15).
- One treatment: icons are "contained" (subtle rounded background) inside
  interactive elements (chips, tabs, action buttons); bare elsewhere. No
  emojis as icons (see `AGENTS.md`).

## 5. Top bar

- Left: brand.
- Right: world indicator + actions (search, graph, jobs, palette, help, theme).
- The `unread` / `notes` counts move out: `unread` → list header; `notes` →
  vault world.

## 6. List (Pane 1)

- Views (`unread/read/archived/starred`) + classification filters (category /
  importance / unclassified) unified under the list header.
- Consistent density and hierarchy: importance stars, category color
  (`--cat-*`), source + time as `xs` metadata.
- Per-row bulk **checkbox** (filled when selected).

## 7. Reader (Pane 2)

- Reading typography (`base`, ~68ch measure), clear metadata row.
- **Focus model**: `Enter` focuses the reader → `j/k`/`Space`/`PageUp/Down`
  scroll; `Esc` returns focus to the list.

## 8. Context (Pane 3)

- **Article**: category, importance, tags, source · its atom note (if any) with
  "open" · related notes (shared category/tags) · backlinks · if no note, a
  **"Create note"** affordance.
- **Note**: navigable `[[wikilinks]]` in/out · tags · neighbors · mini-graph
  (or link to full graph) · source article.

This is the "assimilation" surface: while reading, the user sees how the item
connects to their growing knowledge.

## 9. Keyboard / navigation (three panes)

- List focus: `j/k` navigate, `Enter` open (loads reader + context), `L` links.
- **`Tab` cycles focus** list → reader → context.
- Reader focus: `j/k`/`Space`/`PageUp/Down` scroll.
- Context focus: `j/k` navigate items, `Enter` open.
- Status bar shows the focused pane + contextual keys + the mode (`NEWS` /
  `VAULT`); the same grammar in both worlds.

## 10. Vault world

- Stages **Electrons · Atoms · Molecules** (no inbox — the news list is the
  inbox; no numeric prefixes; lucide icons).
- **Auto-load**: entering the vault or switching stage opens the first note in
  the reader.
- Reader-focus keyboard scroll (Section 7).
- **Organization legend**: a subtle one-liner ("Electrons = concepts cited by
  ≥2 notes · Atoms = one article → one note · Molecules = category synthesis")
  plus teaching empty states ("No atoms yet — run `:process`").

## 11. Bulk mode (giztui parity)

- Per-row checkbox.
- `v` enters (selecting the current item), `Space` toggles explicitly, `j/k`
  move without changing, `a/t/m/c` apply, `Esc` exits.
- Status bar: `BULK · N selected`.

## 12. Sustainable loop

- **Read** (list → reader + context) → **capture** (`m` read-later = starred,
  `a` done with undo, `y` summarize) → **materialize** (`:process` / bulk
  classify / "Create note") → **synthesize** (weekly digest, emergent graph).
- **Machine state visible without digging**: pending classification, running
  jobs, digest — so assimilating the AI world stays low-effort and sustained.

## 13. Non-goals (this iteration)

- No backend changes (this is a frontend/UX pass).
- No new data model / migrations.
- No responsive mobile layout (desktop app only).

## 14. Verification

- `npx tsc --noEmit`, `npx playwright test` (update affected specs).
- Backend untouched; `go build/vet/test` must stay green.
- `wails build` + manual pass through the three panes in both worlds.
