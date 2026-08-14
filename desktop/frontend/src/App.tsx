import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { api } from "./api";
import { applyTheme, currentTheme, THEMES } from "./theme";
import type {
  ArticleDTO,
  DigestDTO,
  ListArticlesOptions,
  NoteDTO,
  SearchResultDTO,
  SourceDTO,
  StatusDTO,
} from "./types";
import { SourceList } from "./components/SourceList";
import { SourceForm } from "./components/SourceForm";
import { ArticleList, type ViewFilter } from "./components/ArticleList";
import { Reader } from "./components/Reader";
import { DigestView } from "./components/DigestView";
import { SearchPanel } from "./components/SearchPanel";
import { GraphPanel } from "./components/GraphPanel";
import { CommandPalette, type PaletteCommand } from "./components/CommandPalette";
import { HelpOverlay } from "./components/HelpOverlay";
import { StatusBar } from "./components/StatusBar";
import { Markdown } from "./components/Markdown";
import { WelcomeOverlay } from "./components/WelcomeOverlay";
import { CircleHelp, Command, RefreshCw } from "lucide-react";

type Panel = "none" | "search" | "graph";

interface Toast {
  msg: string;
  undo?: () => void;
}

export default function App() {
  // ---- data ----
  const [sources, setSources] = useState<SourceDTO[]>([]);
  const [articles, setArticles] = useState<ArticleDTO[]>([]);
  const [selectedIndex, setSelectedIndex] = useState(0);
  const [loadingList, setLoadingList] = useState(false);
  const [view, setView] = useState<ViewFilter>("unread");
  const [filterSource, setFilterSource] = useState<number | null>(null);
  const [status, setStatus] = useState<StatusDTO | null>(null);

  // ---- ui chrome ----
  const [theme, setTheme] = useState(currentTheme());
  const [digestOpen, setDigestOpen] = useState(false);
  const [panel, setPanel] = useState<Panel>("none");
  const [paletteOpen, setPaletteOpen] = useState(false);
  const [helpOpen, setHelpOpen] = useState(false);
  const [sourceForm, setSourceForm] = useState<{ initial: SourceDTO | null } | null>(null);
  const [deleteSource, setDeleteSource] = useState<SourceDTO | null>(null);
  const [toast, setToast] = useState<Toast | null>(null);
  const [countBuf, setCountBuf] = useState("");
  const [welcome, setWelcome] = useState(() => {
    try { return !localStorage.getItem("giznews-welcomed"); } catch { return false; }
  });

  // ---- reader / panels ----
  const [reader, setReader] = useState<ArticleDTO | null>(null);
  const [noteReader, setNoteReader] = useState<NoteDTO | null>(null);
  const [summarizing, setSummarizing] = useState(false);
  const [contentLoading, setContentLoading] = useState(false);
  const [digest, setDigest] = useState<DigestDTO | null>(null);
  const [digestLoading, setDigestLoading] = useState(false);
  const [digestFocusId, setDigestFocusId] = useState<number | null>(null);
  const [searchQuery, setSearchQuery] = useState("");
  const [searchResults, setSearchResults] = useState<SearchResultDTO[]>([]);
  const [searching, setSearching] = useState(false);
  const [searchFocus, setSearchFocus] = useState(0);
  const [graphNoteId, setGraphNoteId] = useState<number | null>(null);

  // bulk (vim visual) mode
  const [bulk, setBulk] = useState(false);
  const [bulkAnchor, setBulkAnchor] = useState<number | null>(null);
  const [autoRefresh, setAutoRefresh] = useState(true);

  const bulkRange = useMemo(() => {
    if (!bulk || bulkAnchor == null) return null;
    return [Math.min(bulkAnchor, selectedIndex), Math.max(bulkAnchor, selectedIndex)] as const;
  }, [bulk, bulkAnchor, selectedIndex]);

  const bulkIds = useMemo(() => {
    if (!bulkRange) return [];
    return articles.slice(bulkRange[0], bulkRange[1] + 1).map((a) => a.id);
  }, [bulkRange, articles]);

  const lastGRef = useRef(0);
  const graphTimer = useRef<number | null>(null);
  const busyRef = useRef(false);

  const notify = useCallback((msg: string, undo?: () => void) => {
    setToast({ msg, undo });
    window.setTimeout(() => setToast(null), 4200);
  }, []);

  // ---- loaders ----
  const loadSources = useCallback(async () => {
    try { setSources(await api.listSources()); } catch (e) { notify(String(e)); }
  }, [notify]);

  const loadStatus = useCallback(async () => {
    try { setStatus(await api.status()); } catch { /* ignore */ }
  }, []);

  const loadArticles = useCallback(async (opts: ListArticlesOptions = {}) => {
    setLoadingList(true);
    try {
      const list = await api.listArticles({ status: view, limit: 400, ...opts });
      setArticles(list);
      setSelectedIndex((i) => Math.min(i, Math.max(0, list.length - 1)));
    } catch (e) {
      notify(String(e));
    } finally {
      setLoadingList(false);
    }
  }, [view, notify]);

  const reloadAll = useCallback(() => {
    void loadSources();
    void loadArticles();
    void loadStatus();
  }, [loadSources, loadArticles, loadStatus]);

  // load once on mount; view changes reload articles via the effect below.
  useEffect(() => {
    void loadSources();
    void loadArticles();
    void loadStatus();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);
  useEffect(() => { void loadArticles(); }, [view, loadArticles]);

  // auto-refresh: quietly fetch new articles every 15 minutes.
  useEffect(() => {
    if (!autoRefresh) return;
    const iv = window.setInterval(async () => {
      try {
        const r = await api.fetch();
        if (r.newArticles > 0) notify(`${r.newArticles} nuevos artículos`);
        void loadArticles();
        void loadStatus();
      } catch { /* network hiccup */ }
    }, 15 * 60 * 1000);
    return () => window.clearInterval(iv);
  }, [autoRefresh, loadArticles, loadStatus, notify]);

  const selectSource = useCallback(async (id: number | null) => {
    setFilterSource(id);
    await loadArticles(id ? { sourceId: id } : {});
  }, [loadArticles]);

  const switchView = useCallback(async (v: ViewFilter) => {
    setView(v);
    setDigestOpen(false);
    setPanel("none");
  }, []);

  // ---- article actions ----
  // apply an async action to an explicit set of article ids.
  const applyToIds = useCallback(async (ids: number[], fn: (a: ArticleDTO) => Promise<void>, after?: () => void) => {
    if (ids.length === 0) return;
    const targets = ids.map((id) => articles.find((a) => a.id === id)).filter((a): a is ArticleDTO => !!a);
    await Promise.all(targets.map((a) => fn(a)));
    after?.();
    void loadStatus();
  }, [articles, loadStatus]);
  const openArticle = useCallback(async (id: number) => {
    // Show the title immediately from the list while content is extracted.
    const listArt = articles.find((a) => a.id === id);
    if (listArt) { setReader(listArt); setNoteReader(null); }
    setContentLoading(true);
    try {
      const full = await api.getArticleContent(id);
      setReader(full);
      if (full.status === "unread") {
        await api.setArticleStatus(id, "read");
        setArticles((prev) => prev.map((a) => (a.id === id ? { ...a, status: "read" } : a)));
        void loadStatus();
      }
    } catch (e) { notify(String(e)); }
    finally { setContentLoading(false); }
  }, [articles, notify, loadStatus]);

  const openNote = useCallback(async (id: number) => {
    try {
      const n = await api.getNote(id);
      setNoteReader(n);
      setReader(null);
    } catch (e) { notify(String(e)); }
  }, [notify]);

  const selected = articles[selectedIndex] ?? null;

  // apply an async action to count consecutive articles from the selection.
  const applyRange = useCallback(async (count: number, fn: (a: ArticleDTO) => Promise<void>, after?: () => void) => {
    const n = Math.max(1, count);
    const batch = articles.slice(selectedIndex, selectedIndex + n);
    if (batch.length === 0) return;
    await Promise.all(batch.map((a) => fn(a)));
    after?.();
    void loadStatus();
  }, [articles, selectedIndex, loadStatus]);

  const exitBulk = useCallback(() => { setBulk(false); setBulkAnchor(null); }, []);

  const archiveIds = useCallback((ids: number[]) => {
    if (ids.length === 0) return;
    const batch = ids.map((id) => articles.find((a) => a.id === id)).filter((a): a is ArticleDTO => !!a);
    if (batch.length === 0) return;
    const restoring = batch[0].status === "archived";
    const undo: Record<number, string> = {};
    for (const a of batch) undo[a.id] = a.status;
    void applyToIds(ids, async (a) => {
      await api.setArticleStatus(a.id, restoring ? "unread" : "archived");
    }, () => {
      setArticles((prev) => prev.filter((a) => !batch.some((b) => b.id === a.id)));
      setSelectedIndex((i) => Math.max(0, i - 1));
      notify(restoring ? `${batch.length} restaurado(s)` : `${batch.length} archivado(s)`, () => {
        void Promise.all(batch.map((a) => api.setArticleStatus(a.id, undo[a.id])));
        void loadArticles();
      });
    });
  }, [articles, applyToIds, notify, loadArticles]);

  const archiveRange = useCallback((count: number) => {
    archiveIds(articles.slice(selectedIndex, selectedIndex + Math.max(1, count)).map((a) => a.id));
  }, [articles, selectedIndex, archiveIds]);

  const toggleReadRange = useCallback((count: number) => {
    void applyRange(count, async (a) => {
      if (a.status === "archived") return;
      await api.setArticleStatus(a.id, a.status === "read" ? "unread" : "read");
    }, () => void loadArticles());
  }, [applyRange, loadArticles]);

  // action entry point that honors bulk mode.
  const bulkAction = useCallback((verb: "archive" | "read") => {
    if (bulk && bulkIds.length > 0) {
      if (verb === "archive") archiveIds(bulkIds);
      else {
        void applyToIds(bulkIds, async (a) => {
          if (a.status === "archived") return;
          await api.setArticleStatus(a.id, a.status === "read" ? "unread" : "read");
        }, () => void loadArticles());
      }
      exitBulk();
      return true;
    }
    return false;
  }, [bulk, bulkIds, archiveIds, applyToIds, exitBulk, loadArticles]);

  const toggleStar = useCallback(async () => {
    if (!selected) return;
    const next = selected.status === "starred" ? "unread" : "starred";
    await api.setArticleStatus(selected.id, next);
    setArticles((prev) => prev.map((a) => (a.id === selected.id ? { ...a, status: next } : a)));
    notify(next === "starred" ? "Destacado" : "Quitado de destacados");
  }, [selected, notify]);

  const summarize = useCallback(async () => {
    if (!selected || busyRef.current) return;
    busyRef.current = true; setSummarizing(true);
    try {
      const updated = await api.summarizeArticle(selected.id);
      setReader(updated);
      setArticles((prev) => prev.map((a) => (a.id === updated.id ? { ...a, summary: updated.summary } : a)));
      notify("Resumen generado");
    } catch (e) { notify(String(e)); }
    finally { busyRef.current = false; setSummarizing(false); }
  }, [selected, notify]);

  const openExternal = useCallback(() => {
    const url = reader?.url || selected?.url;
    if (url) void api.openURL(url);
  }, [reader, selected]);

  const scrollReader = useCallback((dir: number) => {
    const el = document.querySelector<HTMLElement>(".reader-scroll");
    if (el) el.scrollBy({ top: dir * el.clientHeight * 0.9 });
  }, []);

  const openAdjacent = useCallback((delta: number) => {
    const n = Math.max(0, articles.length - 1);
    const ni = Math.max(0, Math.min(selectedIndex + delta, n));
    setSelectedIndex(ni);
    const a = articles[ni];
    if (a) void openArticle(a.id);
  }, [articles, selectedIndex, openArticle]);

  // ---- panels ----
  const generateDigest = useCallback(async () => {
    if (busyRef.current) return;
    busyRef.current = true; setDigestLoading(true);
    try {
      const d = await api.digest();
      setDigest(d);
      setDigestFocusId(d.themes[0]?.articles[0]?.id ?? null);
      setDigestOpen(true);
    } catch (e) { notify(String(e)); }
    finally { busyRef.current = false; setDigestLoading(false); }
  }, [notify]);

  const digestIds = useMemo(() => {
    const ids: number[] = [];
    if (digest) for (const th of digest.themes) for (const a of th.articles) ids.push(a.id);
    return ids;
  }, [digest]);

  const moveDigestFocus = useCallback((delta: number) => {
    if (digestIds.length === 0) return;
    const cur = digestFocusId != null ? digestIds.indexOf(digestFocusId) : 0;
    const next = Math.max(0, Math.min(cur + delta, digestIds.length - 1));
    setDigestFocusId(digestIds[next]);
  }, [digestIds, digestFocusId]);

  const openDigestFocus = useCallback(() => {
    if (digestFocusId == null) return;
    setDigestOpen(false);
    const idx = articles.findIndex((a) => a.id === digestFocusId);
    if (idx >= 0) setSelectedIndex(idx);
    void openArticle(digestFocusId);
  }, [digestFocusId, articles, openArticle]);

  const openGraph = useCallback(async () => {
    setPanel("graph");
    setGraphNoteId(null);
    if (selected) {
      try {
        const notes = await api.listNotes("atom");
        const match = notes.find((n) => n.title.toLowerCase() === selected.title.toLowerCase());
        setGraphNoteId(match?.id ?? null);
      } catch { setGraphNoteId(null); }
    }
  }, [selected]);

  const buildAndOpenGraph = useCallback(async () => {
    try {
      await api.kbuild();
      notify("knowledge graph actualizado");
      await openGraph();
    } catch (e) { notify(String(e)); }
  }, [openGraph, notify]);

  const runSearch = useCallback(async (q: string) => {
    setSearching(true);
    try {
      setSearchResults(q.trim() ? await api.search(q, 20) : []);
      setSearchFocus(0);
    } catch (e) { notify(String(e)); }
    finally { setSearching(false); }
  }, [notify]);

  // ---- commands ----
  const runCmd = useCallback(async (fn: () => Promise<unknown>, msg: string) => {
    try { await fn(); notify(msg); } catch (e) { notify(String(e)); }
  }, [notify]);

  const commands = useMemo<PaletteCommand[]>(() => [
    { name: "fetch", hint: "Traer nuevos artículos (+ extraer cuerpos)", run: () => void runCmd(async () => {
      const r = await api.fetch(); await reloadAll();
      notify(`${r.newArticles} nuevos${r.extracted ? ` · ${r.extracted} extraídos` : ""}`);
    }, "fetch") },
    { name: "classify", hint: "Clasificar (reglas + LLM)", run: () => void runCmd(() => api.classify(200), "clasificación completa") },
    { name: "kb build", hint: "Generar atoms/electrons", run: () => void runCmd(api.kbuild, "knowledge graph actualizado") },
    { name: "kb synth <categoría>", hint: "Molecule de una categoría", run: () => {
      const cat = window.prompt("Categoría (models, research, industry…)");
      if (cat) void runCmd(() => api.ksynthesize(cat), `molecule de ${cat}`);
    } },
    { name: "search index", hint: "Indexar embeddings", run: () => void runCmd(api.searchIndex, "índice actualizado") },
    { name: "digest", hint: "Digest diario", run: () => void generateDigest() },
    { name: "auto-refresh", hint: autoRefresh ? "Desactivar refresco automático" : "Activar refresco cada 15 min", run: () => setAutoRefresh((v) => !v) },
    { name: "add-source", hint: "Añadir una fuente RSS/HN/arXiv/gmail", run: () => setSourceForm({ initial: null }) },
    { name: "theme", hint: `Cambiar tema (${THEMES.map((t) => t.name).join(" | ")})`, run: () => {
      const next = THEMES[(THEMES.findIndex((t) => t.name === theme) + 1) % THEMES.length];
      applyTheme(next.name); setTheme(next.name);
    } },
    { name: "open vault", hint: "Abrir vault en Obsidian", run: () => void api.openVault() },
    { name: "quit", hint: "Salir", run: () => void api.quit() },
  ], [runCmd, generateDigest, reloadAll, theme, autoRefresh]);

  // ---- keyboard (vim grammar) ----
  useEffect(() => {
    const clearGraphTimer = () => {
      if (graphTimer.current != null) { window.clearTimeout(graphTimer.current); graphTimer.current = null; }
    };

    const handler = (e: KeyboardEvent) => {
      const target = e.target as HTMLElement;
      const typing = target && (target.tagName === "INPUT" || target.tagName === "TEXTAREA" || target.isContentEditable);
      const k = e.key;
      const now = Date.now();

      if (k === "Escape") {
        if (bulk) { exitBulk(); return; }
        if (paletteOpen) { setPaletteOpen(false); return; }
        if (helpOpen) { setHelpOpen(false); return; }
        if (sourceForm) { setSourceForm(null); return; }
        if (deleteSource) { setDeleteSource(null); return; }
        if (panel !== "none") { setPanel("none"); return; }
        if (digestOpen) { setDigestOpen(false); return; }
        if (noteReader) { setNoteReader(null); return; }
        return;
      }
      if (typing) return; // inputs handle their own keys
      if (paletteOpen || helpOpen || sourceForm || deleteSource) return;

      // digest mode: j/k/Enter navigate its articles
      if (digestOpen) {
        if (k === "j" || k === "ArrowDown") { moveDigestFocus(1); return; }
        if (k === "k" || k === "ArrowUp") { moveDigestFocus(-1); return; }
        if (k === "Enter") { openDigestFocus(); return; }
        if (k === "1") { setDigestOpen(false); return; }
      }

      // bulk (vim visual) mode
      if (k === "v") {
        if (bulk) exitBulk();
        else { setBulk(true); setBulkAnchor(selectedIndex); }
        return;
      }
      if (bulk) {
        const bn = Math.max(0, articles.length - 1);
        if (k === "j" || k === "ArrowDown") { setSelectedIndex((i) => Math.min(i + 1, bn)); return; }
        if (k === "k" || k === "ArrowUp") { setSelectedIndex((i) => Math.max(i - 1, 0)); return; }
        if (k === "a") { bulkAction("archive"); return; }
        if (k === "t") { bulkAction("read"); return; }
        if (k === "y") { void summarize(); return; }
        if (k === "m") { void toggleStar(); return; }
        return; // consume everything else while in bulk
      }

      // count prefix
      if (k >= "0" && k <= "9") {
        if (countBuf.length < 2) setCountBuf((c) => c + k);
        return;
      }
      const count = countBuf ? parseInt(countBuf, 10) || 1 : 1;
      if (countBuf) setCountBuf("");
      clearGraphTimer();
      e.preventDefault();

      const n = Math.max(0, articles.length - 1);

      // reader paging (space / Ctrl+d / Ctrl+u) when an article is open
      if (k === " ") {
        scrollReader(e.shiftKey ? -0.9 : 0.9);
        return;
      }
      if (k === "d" && e.ctrlKey) {
        if (reader) scrollReader(0.9); else setSelectedIndex((i) => Math.min(i + 8, n));
        return;
      }
      if (k === "u" && e.ctrlKey) {
        if (reader) scrollReader(-0.9); else setSelectedIndex((i) => Math.max(i - 8, 0));
        return;
      }

      if (k === "J") { openAdjacent(count); return; }
      if (k === "K") { openAdjacent(-count); return; }
      if (k === "q") { void api.quit(); return; }
      if (k === "j" || k === "ArrowDown") { setSelectedIndex((i) => Math.min(i + count, n)); return; }
      if (k === "k" || k === "ArrowUp") { setSelectedIndex((i) => Math.max(i - count, 0)); return; }
      if (k === "Home") { setSelectedIndex(0); return; }
      if (k === "End") { setSelectedIndex(n); return; }
      if (k === "g") {
        if (now - lastGRef.current < 300) {
          lastGRef.current = 0;
          setSelectedIndex(0); // gg → top
        } else {
          lastGRef.current = now;
          graphTimer.current = window.setTimeout(() => { openGraph(); graphTimer.current = null; }, 300);
        }
        return;
      }
      if (k === "G") { setSelectedIndex(n); return; }
      if (k === "Enter") { if (selected) void openArticle(selected.id); return; }
      if (k === "y") { void summarize(); return; }
      if (k === "a") { archiveRange(count); return; }
      if (k === "t") { toggleReadRange(count); return; }
      if (k === "m") { void toggleStar(); return; }
      if (k === "O" || k === "o") { openExternal(); return; }
      if (k === "s") { setPanel((p) => (p === "search" ? "none" : "search")); return; }
      if (k === "d") { void generateDigest(); return; }
      if (k === ":") { setPaletteOpen(true); return; }
      if (k === "?") { setHelpOpen(true); return; }
      if (k === "u" && !e.ctrlKey) { void switchView("unread"); return; }
      if (k === "r") { void switchView("read"); return; }
      if (k === "x") { void switchView("archived"); return; }
      if (k === "*") { void switchView("starred"); return; }
    };
    window.addEventListener("keydown", handler);
    return () => { window.removeEventListener("keydown", handler); clearGraphTimer(); };
  }, [paletteOpen, helpOpen, sourceForm, deleteSource, panel, digestOpen, noteReader, countBuf, articles.length, selected, selectedIndex, openArticle, summarize, archiveRange, toggleReadRange, toggleStar, openExternal, openGraph, switchView, moveDigestFocus, openDigestFocus, scrollReader, openAdjacent, reader, bulk, exitBulk, bulkAction]);

  // ---- source CRUD ----
  const saveSource = useCallback(async (data: { name: string; type: string; url: string; group: string }) => {
    try {
      await api.addSource(data.name, data.type, data.url, data.group);
      setSourceForm(null);
      await loadSources();
      notify(`Fuente añadida: ${data.name}`);
    } catch (e) { notify(String(e)); }
  }, [loadSources, notify]);

  const confirmDeleteSource = useCallback(async () => {
    if (!deleteSource) return;
    try {
      // logical delete: remove from registry, articles are preserved.
      await api.setSourceEnabled(deleteSource.id, false);
      await api.deleteSource(deleteSource.id);
      if (filterSource === deleteSource.id) await selectSource(null);
      await loadSources();
      notify(`Fuente eliminada de la lista (artículos conservados)`);
    } catch (e) { notify(String(e)); }
    finally { setDeleteSource(null); }
  }, [deleteSource, filterSource, selectSource, loadSources, notify]);

  const uiCtx: "list" | "reader" | "search" | "graph" | "digest" =
    panel === "search" ? "search"
    : panel === "graph" ? "graph"
    : digestOpen ? "digest"
    : reader ? "reader"
    : "list";

  const modeLabel = panel === "search" ? "BUSCAR"
    : panel === "graph" ? "GRAFO"
    : digestOpen ? "DIGEST"
    : reader ? "LECTOR"
    : "LISTA";

  const filterLabel = filterSource
    ? `fuente: ${sources.find((s) => s.id === filterSource)?.name ?? "?"}`
    : undefined;

  return (
    <div className="app">
      <header className="topbar">
        <div className="brand">
          <span className="brand-mark">G</span>
          <span className="brand-name">Giz<em>News</em></span>
        </div>
        {status && (
          <div className="status">
            <span className="pill">{status.unreadArticles} no leídos</span>
            <span className="pill">🧠 {status.totalNotes} notas</span>
            {filterSource && (
              <button className="pill filter" onClick={() => void selectSource(null)}>
                ✕ {filterLabel}
              </button>
            )}
          </div>
        )}
        <div className="topbar-actions">
          <select
            className="theme-select"
            value={theme}
            onChange={(e) => { applyTheme(e.target.value as never); setTheme(e.target.value as never); }}
            title="Tema"
          >
            {THEMES.map((t) => <option key={t.name} value={t.name}>{t.label}</option>)}
          </select>
          <button className="icon-btn" onClick={() => void reloadAll()} title="Recargar">
            <RefreshCw size={15} />
          </button>
          <button className="icon-btn" onClick={() => setHelpOpen(true)} title="Ayuda (?)">
            <CircleHelp size={15} />
          </button>
          <button className="icon-btn" onClick={() => setPaletteOpen(true)} title="Comandos (:)">
            <Command size={15} />
          </button>
        </div>
      </header>

      <main className="layout">
        <aside className="col sources-col">
          <SourceList
            sources={sources}
            activeId={filterSource}
            onSelect={(id) => void selectSource(id)}
            onToggle={async (id, enabled) => {
              await api.setSourceEnabled(id, enabled);
              await loadSources();
              if (filterSource === id && !enabled) await selectSource(null);
            }}
            onAdd={() => setSourceForm({ initial: null })}
            onEdit={(s) => setSourceForm({ initial: s })}
            onDelete={(s) => setDeleteSource(s)}
          />
        </aside>

        <section className="col list-col">
          {digestOpen ? (
            <DigestView
              digest={digest}
              loading={digestLoading}
              unreadCount={status?.unreadArticles ?? 0}
              focusId={digestFocusId}
              onFocus={setDigestFocusId}
              onGenerate={generateDigest}
              onOpenArticle={(id) => {
                setDigestOpen(false);
                const idx = articles.findIndex((a) => a.id === id);
                if (idx >= 0) setSelectedIndex(idx);
                void openArticle(id);
              }}
            />
          ) : (
            <ArticleList
              articles={articles}
              selectedIndex={selectedIndex}
              loading={loadingList}
              view={view}
              bulkRange={bulkRange}
              onView={(v) => void switchView(v)}
              onSelect={(i) => { setSelectedIndex(i); if (reader) setReader(null); }}
            />
          )}
        </section>

        <section className="col reader-col">
          {panel === "search" ? (
            <SearchPanel
              query={searchQuery}
              onQuery={(q) => { setSearchQuery(q); void runSearch(q); }}
              searching={searching}
              results={searchResults}
              focus={searchFocus}
              onFocus={setSearchFocus}
              onClose={() => setPanel("none")}
              onOpen={(r) => {
                setPanel("none");
                if (r.kind === "article") void openArticle(r.id);
                else void openNote(r.id);
              }}
            />
          ) : panel === "graph" ? (
            <GraphPanel
              key={graphNoteId ?? "none"}
              noteId={graphNoteId}
              onOpenNote={openNote}
              onBuild={() => void buildAndOpenGraph()}
              notify={notify}
            />
          ) : noteReader ? (
            <div className="reader">
              <div className="reader-head">
                <span className="note-type">{noteReader.type}</span>
                <h1>{noteReader.title}</h1>
              </div>
              <div className="reader-scroll">
                <Markdown content={noteReader.content} />
              </div>
            </div>
          ) : (
            <Reader
              article={reader}
              summarizing={summarizing}
              contentLoading={contentLoading}
              onSummarize={summarize}
              onArchive={() => archiveRange(1)}
              onStar={() => void toggleStar()}
              onOpenLink={openExternal}
            />
          )}
        </section>
      </main>

      <StatusBar
        context={uiCtx}
        modeLabel={modeLabel}
        filter={filterLabel}
        count={countBuf ? parseInt(countBuf, 10) : undefined}
        bulk={bulk}
        bulkCount={bulkIds.length}
        autoRefresh={autoRefresh}
        llmOn={!!status?.llmEnabled}
        llmReachable={!!status?.llmReachable}
        llmProvider={status?.llmProvider ?? "llm"}
        onToggleAuto={() => setAutoRefresh((v) => !v)}
      />

      {paletteOpen && <CommandPalette commands={commands} onClose={() => setPaletteOpen(false)} />}
      {helpOpen && <HelpOverlay onClose={() => setHelpOpen(false)} />}
      {welcome && (
        <WelcomeOverlay
          onDone={() => {
            try { localStorage.setItem("giznews-welcomed", "1"); } catch { /* ignore */ }
            setWelcome(false);
          }}
        />
      )}
      {sourceForm && (
        <SourceForm initial={sourceForm.initial} onSave={saveSource} onCancel={() => setSourceForm(null)} />
      )}
      {deleteSource && (
        <div className="modal-overlay" onClick={() => setDeleteSource(null)}>
          <div className="modal" onClick={(e) => e.stopPropagation()}>
            <div className="modal-head"><h2>Eliminar fuente</h2></div>
            <div className="modal-body">
              <p style={{ margin: 0 }}>
                ¿Eliminar <strong>{deleteSource.name}</strong> de la lista?
                <br /><span className="muted">Los artículos ya guardados se conservan. Es reversible: puedes volver a añadirla.</span>
              </p>
            </div>
            <div className="modal-foot">
              <button onClick={() => setDeleteSource(null)}>Cancelar</button>
              <button onClick={() => void confirmDeleteSource()} style={{ background: "var(--accent-dim)", color: "#fff" }}>Eliminar</button>
            </div>
          </div>
        </div>
      )}
      {toast && (
        <div className="toast">
          <span>{toast.msg}</span>
          {toast.undo && <button className="toast-undo" onClick={() => { toast.undo?.(); setToast(null); }}>Deshacer</button>}
        </div>
      )}
    </div>
  );
}
