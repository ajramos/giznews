import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { api } from "./api";
import { applyTheme, currentTheme } from "./theme";
import type {
  ArticleDTO,
  DigestDTO,
  ListArticlesOptions,
  NoteDTO,
  SearchResultDTO,
  SourceDTO,
  StatusDTO,
} from "./types";
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
import { ThemePicker } from "./components/ThemePicker";
import { ThemeModal } from "./components/ThemeModal";
import { SourcePicker } from "./components/SourcePicker";
import { NotesPicker } from "./components/NotesPicker";
import { PipelineModal, type PipelineStep } from "./components/PipelineModal";
import { PromptModal } from "./components/PromptModal";
import { StatusModal } from "./components/StatusModal";
import { VaultPanel } from "./components/VaultPanel";
import { LinksPicker, type LinkItem } from "./components/LinksPicker";
import { buildNoteLinks } from "./noteLinks";
import { CircleHelp, Command, RefreshCw, Tag, Network, Search } from "lucide-react";

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
  const [themeModalOpen, setThemeModalOpen] = useState(false);
  const [sourcePickerOpen, setSourcePickerOpen] = useState(false);
  const [notesPickerOpen, setNotesPickerOpen] = useState(false);
  const [pipelineOpen, setPipelineOpen] = useState(false);
  const [pipelineSteps, setPipelineSteps] = useState<PipelineStep[]>([]);
  const [synthPrompt, setSynthPrompt] = useState(false);
  const [statusOpen, setStatusOpen] = useState(false);
  const [vaultOpen, setVaultOpen] = useState(false);
  const [noteLinks, setNoteLinks] = useState<LinkItem[] | null>(null);
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
  const [graphFocusId, setGraphFocusId] = useState<number | null>(null);
  const [graphRefresh, setGraphRefresh] = useState(0);

  // bulk mode (giztui-style): v to enter, space to toggle individual items,
  // then an action key applies to the selected set.
  const [bulk, setBulk] = useState(false);
  const [bulkSel, setBulkSel] = useState<Set<number>>(new Set());
  const [autoRefresh, setAutoRefresh] = useState(true);

  const bulkIds = useMemo(() => [...bulkSel], [bulkSel]);

  const lastGRef = useRef(0);
  const graphTimer = useRef<number | null>(null);
  const busyRef = useRef(false);
  const loadingIdRef = useRef<number | null>(null);
  const searchIndexedRef = useRef(false);
  const articlesRef = useRef(articles);
  articlesRef.current = articles;

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
  const openArticle = useCallback(async (id: number, silent = false) => {
    // Guard: skip if this article is already the one loading (dedupes the
    // auto-load effect vs Enter vs adjacent-nav all racing on the same id).
    if (loadingIdRef.current === id) return;
    loadingIdRef.current = id;
    // Show the title immediately from the list while content is extracted.
    const listArt = articlesRef.current.find((a) => a.id === id);
    if (listArt) {
      setReader(listArt);
      // The background auto-load must never clobber an open note; only an
      // explicit user action switches from a note back to an article.
      if (!silent) setNoteReader(null);
    }
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
    finally { setContentLoading(false); loadingIdRef.current = null; }
  }, [notify, loadStatus]);

  const openNote = useCallback(async (id: number) => {
    try {
      const n = await api.getNote(id);
      setNoteReader(n);
      setReader(null);
      setPanel("none"); // a note takes over the reader, closing any panel
    } catch (e) { notify(String(e)); }
  }, [notify]);

  // Open the giztui-style links picker for a note (in the reader view).
  const openNoteLinks = useCallback(async (id: number) => {
    try {
      const notes = await api.listNotes("");
      const n = notes.find((x) => x.id === id);
      if (n) setNoteLinks(buildNoteLinks(n, notes));
    } catch (e) { notify(String(e)); }
  }, [notify]);

  // Open the links picker for an ARTICLE being read: find its Atom note (by
  // title) and show that note's connections.
  const openArticleLinks = useCallback(async (articleId: number) => {
    try {
      const notes = await api.listNotes("");
      const art = articlesRef.current.find((a) => a.id === articleId);
      if (!art) return;
      const note = notes.find((n) => n.title.toLowerCase() === art.title.toLowerCase());
      if (note) {
        setNoteLinks(buildNoteLinks(note, notes));
      } else {
        notify("Este artículo aún no tiene nota — genera una con el grafo (g) o :procesar");
      }
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

  const exitBulk = useCallback(() => { setBulk(false); setBulkSel(new Set()); }, []);

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
  const bulkAction = useCallback((verb: "archive" | "read" | "star") => {
    if (bulk && bulkIds.length > 0) {
      if (verb === "archive") {
        archiveIds(bulkIds);
      } else if (verb === "read") {
        void applyToIds(bulkIds, async (a) => {
          if (a.status === "archived") return;
          await api.setArticleStatus(a.id, a.status === "read" ? "unread" : "read");
        }, () => void loadArticles());
      } else {
        void applyToIds(bulkIds, async (a) => {
          if (a.status === "archived") return;
          await api.setArticleStatus(a.id, a.status === "starred" ? "unread" : "starred");
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
    setSelectedIndex((i) => Math.max(0, Math.min(i + delta, n)));
  }, [articles.length]);

  // ---- lazy loading: the selected article loads automatically (debounced),
  // and the adjacent ones are prefetched so navigation is instant. ----
  // Paused while a modal/panel is open (checked via ref inside the timer, so
  // closing a modal does NOT re-trigger a load that would clobber a note).
  const modalOpen =
    paletteOpen || helpOpen || notesPickerOpen || sourcePickerOpen || themeModalOpen ||
    pipelineOpen || sourceForm != null || deleteSource != null || vaultOpen ||
    digestOpen || panel !== "none" || synthPrompt || statusOpen || noteLinks != null;
  const modalOpenRef = useRef(modalOpen);
  modalOpenRef.current = modalOpen;
  const noteReaderRef = useRef<NoteDTO | null>(null);
  noteReaderRef.current = noteReader;

  useEffect(() => {
    const art = articlesRef.current[selectedIndex];
    if (!art) return;
    const t = window.setTimeout(() => {
      // Don't auto-load an article while a modal is open or a note is being
      // read — it would clobber the note the user just opened.
      if (modalOpenRef.current || noteReaderRef.current) return;
      void openArticle(art.id, true); // silent: never clobber an open note
    }, 120);
    return () => window.clearTimeout(t);
  }, [selectedIndex, articles.length, openArticle]);

  useEffect(() => {
    const next = articlesRef.current[selectedIndex + 1];
    const prev = articlesRef.current[selectedIndex - 1];
    if (next) void api.getArticleContent(next.id).catch(() => {});
    if (prev) void api.getArticleContent(prev.id).catch(() => {});
  }, [selectedIndex, articles.length]);

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
    // Resolve the article's note first, then open the panel so it never shows
    // the "no note" empty state while the lookup is still in flight.
    let id: number | null = null;
    if (selected) {
      try {
        const notes = await api.listNotes("atom");
        const match = notes.find((n) => n.title.toLowerCase() === selected.title.toLowerCase());
        id = match?.id ?? null;
      } catch { id = null; }
    }
    setGraphFocusId(id);
    setPanel("graph");
  }, [selected]);

  const buildAndOpenGraph = useCallback(async () => {
    // Generate the note for the CURRENT article (regardless of importance
    // threshold), then reload the graph so it appears immediately.
    const id = selected?.id;
    if (id == null) return;
    try {
      await api.ensureArticleNote(id);
      notify("Nota generada para este artículo");
      setGraphRefresh((r) => r + 1);
      await openGraph();
    } catch (e) { notify(String(e)); }
  }, [selected, openGraph, notify]);

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

  const runPipeline = useCallback(async () => {
    setPipelineOpen(true);
    const steps: PipelineStep[] = [
      { key: "fetch", label: "Traer artículos", icon: <RefreshCw size={14} />, status: "pending" },
      { key: "classify", label: "Clasificar (reglas + LLM)", icon: <Tag size={14} />, status: "pending" },
      { key: "kb", label: "Construir knowledge graph", icon: <Network size={14} />, status: "pending" },
      { key: "index", label: "Indexar búsqueda semántica", icon: <Search size={14} />, status: "pending" },
    ];
    const update = (key: string, patch: Partial<PipelineStep>) =>
      setPipelineSteps((prev) => prev.map((s) => (s.key === key ? { ...s, ...patch } : s)));
    setPipelineSteps(steps);

    try {
      update("fetch", { status: "running" });
      const f = await api.fetch();
      update("fetch", { status: "done", summary: `${f.newArticles} nuevos · ${f.extracted} extraídos` });
      await loadArticles();
      await loadSources();
      await loadStatus();

      update("classify", { status: "running" });
      const c = await api.classify(500);
      update("classify", { status: "done", summary: `${c.classified} clasificados (⚡ ${c.byRules} · ${c.byLLM} LLM)` });
      await loadArticles();

      update("kb", { status: "running" });
      const k = await api.kbuild();
      update("kb", { status: "done", summary: `${k.atomsCreated} atoms · ${k.electronsCreated} electrons` });
      await loadStatus();

      update("index", { status: "running" });
      const idx = await api.searchIndex();
      update("index", { status: "done", summary: `${idx.notesEmbedded} notas · ${idx.articlesEmbedded} artículos` });

      notify("Procesado completado");
    } catch (e) {
      setPipelineSteps((prev) => prev.map((s) => (s.status === "running" ? { ...s, status: "error", summary: String(e) } : s)));
    }
  }, [loadArticles, loadSources, loadStatus, notify]);

  const commands = useMemo<PaletteCommand[]>(() => [
    { name: "procesar", hint: "Pipeline completo: fetch → clasificar → kb", run: () => void runPipeline() },
    { name: "fetch", hint: "Traer nuevos artículos (+ extraer cuerpos)", run: () => void runCmd(async () => {
      const r = await api.fetch(); await reloadAll();
      notify(`${r.newArticles} nuevos${r.extracted ? ` · ${r.extracted} extraídos` : ""}`);
    }, "fetch") },
    { name: "classify", hint: "Clasificar (reglas + LLM)", run: () => void runCmd(async () => {
      const c = await api.classify(200);
      notify(`${c.classified} clasificados (⚡ ${c.byRules} reglas · ${c.byLLM} LLM)`);
    }, "clasificación") },
    { name: "kb build", hint: "Generar atoms/electrons", run: () => void runCmd(async () => {
      const k = await api.kbuild();
      notify(`${k.atomsCreated} atoms · ${k.electronsCreated} electrons`);
      await loadStatus();
    }, "kb build") },
    { name: "kb synth <categoría>", hint: "Molecule de una categoría", run: () => setSynthPrompt(true) },
    { name: "search index", hint: "Indexar embeddings", run: () => void runCmd(api.searchIndex, "índice actualizado") },
    { name: "digest", hint: "Digest diario", run: () => void generateDigest() },
    { name: "auto-refresh", hint: autoRefresh ? "Desactivar refresco automático" : "Activar refresco cada 15 min", run: () => setAutoRefresh((v) => !v) },
    { name: "sources", hint: "Gestionar fuentes (picker)", run: () => setSourcePickerOpen(true) },
    { name: "notes", hint: "Ver notas del knowledge graph", run: () => setNotesPickerOpen(true) },
    { name: "vault", hint: "Flujo del vault (inbox → electrons → atoms → molecules)", run: () => setVaultOpen(true) },
    { name: "status", hint: "Resumen del estado (artículos, notas, LLM)", run: () => setStatusOpen(true) },
    { name: "add-source", hint: "Añadir una fuente RSS/HN/arXiv/gmail", run: () => { setSourcePickerOpen(false); setSourceForm({ initial: null }); } },
    { name: "theme", hint: "Elegir tema (picker)", run: () => setThemeModalOpen(true) },
    { name: "open vault", hint: "Abrir vault en Obsidian", run: () => void api.openVault() },
    { name: "quit", hint: "Salir", run: () => void api.quit() },
  ], [runCmd, generateDigest, reloadAll, theme, autoRefresh, runPipeline]);

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
        if (noteLinks) { setNoteLinks(null); return; }
        if (bulk) { exitBulk(); return; }
        if (paletteOpen) { setPaletteOpen(false); return; }
        if (helpOpen) { setHelpOpen(false); return; }
        if (themeModalOpen) { setThemeModalOpen(false); return; }
        if (sourcePickerOpen) { setSourcePickerOpen(false); return; }
        if (notesPickerOpen) { setNotesPickerOpen(false); return; }
        if (pipelineOpen) { setPipelineOpen(false); return; }
        if (synthPrompt) { setSynthPrompt(false); return; }
        if (statusOpen) { setStatusOpen(false); return; }
        if (vaultOpen) { setVaultOpen(false); return; }
        if (sourceForm) { setSourceForm(null); return; }
        if (deleteSource) { setDeleteSource(null); return; }
        if (panel !== "none") { setPanel("none"); return; }
        if (digestOpen) { setDigestOpen(false); return; }
        if (noteReader) { setNoteReader(null); return; }
        return;
      }
      if (typing) return; // inputs handle their own keys
      if (noteLinks || paletteOpen || helpOpen || sourceForm || deleteSource || themeModalOpen || sourcePickerOpen || notesPickerOpen || pipelineOpen || synthPrompt || statusOpen || vaultOpen) return;

      // digest mode: j/k/Enter navigate its articles
      if (digestOpen) {
        if (k === "j" || k === "ArrowDown") { moveDigestFocus(1); return; }
        if (k === "k" || k === "ArrowUp") { moveDigestFocus(-1); return; }
        if (k === "Enter") { openDigestFocus(); return; }
        if (k === "1") { setDigestOpen(false); return; }
      }

      // bulk mode (giztui-style): v enter, space toggle, action key applies
      if (k === "v") {
        if (bulk) exitBulk();
        else {
          setBulk(true);
          const id = selected?.id;
          setBulkSel(id != null ? new Set([id]) : new Set());
        }
        return;
      }
      if (bulk) {
        const bn = Math.max(0, articles.length - 1);
        if (k === "j" || k === "ArrowDown") { setSelectedIndex((i) => Math.min(i + 1, bn)); return; }
        if (k === "k" || k === "ArrowUp") { setSelectedIndex((i) => Math.max(i - 1, 0)); return; }
        if (k === " ") {
          const id = selected?.id;
          if (id != null) {
            setBulkSel((prev) => {
              const next = new Set(prev);
              if (next.has(id)) next.delete(id);
              else next.add(id);
              return next;
            });
          }
          return;
        }
        if (k === "a") { bulkAction("archive"); return; }
        if (k === "t") { bulkAction("read"); return; }
        if (k === "m") { bulkAction("star"); return; }
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

      // Note reader mode: j/k/arrows/space scroll the note; L opens its links.
      if (noteReader) {
        if (k === "j" || k === "ArrowDown") { scrollReader(0.35); return; }
        if (k === "k" || k === "ArrowUp") { scrollReader(-0.35); return; }
        if (k === " ") { scrollReader(e.shiftKey ? -0.9 : 0.9); return; }
        if (k === "L") { void openNoteLinks(noteReader.id); return; }
      }

      // Article reader: L shows the article's knowledge-graph connections.
      if (reader && k === "L") { void openArticleLinks(reader.id); return; }

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
      if (k === "s") {
        setPanel((p) => (p === "search" ? "none" : "search"));
        if (!searchIndexedRef.current) {
          searchIndexedRef.current = true;
          void api.searchIndex().catch(() => {});
        }
        return;
      }
      if (k === "n") { setNotesPickerOpen(true); return; }
      if (k === "f") { setVaultOpen(true); return; }
      if (k === "d") { void generateDigest(); return; }
      if (k === ":") { setPaletteOpen(true); return; }
      if (k === "?") { setHelpOpen(true); return; }
      if (k === "u" && !e.ctrlKey) { void switchView("unread"); return; }
      if (k === "r") { void switchView("read"); return; }
      if (k === "x") { void switchView("archived"); return; }
      if (k === "*") { void switchView("starred"); return; }
    };
    window.addEventListener("keydown", handler);
    return () => window.removeEventListener("keydown", handler);
  }, [paletteOpen, helpOpen, sourceForm, deleteSource, themeModalOpen, sourcePickerOpen, notesPickerOpen, pipelineOpen, synthPrompt, statusOpen, vaultOpen, noteLinks, panel, digestOpen, noteReader, countBuf, articles.length, selected, selectedIndex, openArticle, summarize, archiveRange, toggleReadRange, toggleStar, openExternal, openGraph, switchView, moveDigestFocus, openDigestFocus, scrollReader, openAdjacent, openNoteLinks, openArticleLinks, reader, bulk, exitBulk, bulkAction]);

  // clear any pending graph-open timer only on unmount (the keyboard effect
  // re-subscribes often, so its cleanup must NOT cancel the pending `g`).
  useEffect(() => () => {
    if (graphTimer.current != null) window.clearTimeout(graphTimer.current);
  }, []);

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
            <span className="pill" title="Artículos sin leer">{status.unreadArticles} no leídos</span>
            <span className="pill" title="Notas del knowledge graph en Obsidian (atoms + electrons + molecules)">🧠 {status.totalNotes} notas</span>
            {filterSource && (
              <button className="pill filter" onClick={() => void selectSource(null)}>
                ✕ {filterLabel}
              </button>
            )}
          </div>
        )}
        <div className="topbar-actions">
          <ThemePicker value={theme} onChange={(t) => { applyTheme(t); setTheme(t); }} />
          <button className="icon-btn" onClick={() => void reloadAll()} title="Recargar"><RefreshCw size={15} /></button>
          <button className="icon-btn" onClick={() => setHelpOpen(true)} title="Ayuda (?)"><CircleHelp size={15} /></button>
          <button className="icon-btn" onClick={() => setPaletteOpen(true)} title="Comandos (:)"><Command size={15} /></button>
        </div>
      </header>

      <main className="layout">
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
              hasSources={sources.length > 0}
              bulkSel={bulkSel}
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
              focusNoteId={graphFocusId}
              refresh={graphRefresh}
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
              llmAvailable={!!status?.llmEnabled && !!status?.llmReachable}
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
      {themeModalOpen && (
        <ThemeModal
          value={theme}
          onChange={(t) => { applyTheme(t); setTheme(t); }}
          onClose={() => setThemeModalOpen(false)}
        />
      )}
      {sourcePickerOpen && (
        <SourcePicker
          sources={sources}
          onToggle={(id, enabled) => { void api.setSourceEnabled(id, enabled).then(() => loadSources()); }}
          onAdd={() => { setSourcePickerOpen(false); setSourceForm({ initial: null }); }}
          onEdit={(s) => { setSourcePickerOpen(false); setSourceForm({ initial: s }); }}
          onDelete={(s) => { setSourcePickerOpen(false); setDeleteSource(s); }}
          onFilter={(s) => { setSourcePickerOpen(false); void selectSource(s.id); }}
          onClose={() => setSourcePickerOpen(false)}
        />
      )}
      {notesPickerOpen && (
        <NotesPicker
          onOpen={(id) => { setNotesPickerOpen(false); void openNote(id); }}
          onClose={() => setNotesPickerOpen(false)}
          notify={notify}
        />
      )}
      {pipelineOpen && <PipelineModal steps={pipelineSteps} onClose={() => setPipelineOpen(false)} />}
      {synthPrompt && (
        <PromptModal
          title="Categoría para la síntesis"
          placeholder="models, research, industry…"
          onSubmit={(cat) => { setSynthPrompt(false); void runCmd(() => api.ksynthesize(cat), `molecule de ${cat}`); }}
          onClose={() => setSynthPrompt(false)}
        />
      )}
      {statusOpen && <StatusModal onClose={() => setStatusOpen(false)} />}
      {vaultOpen && (
        <VaultPanel
          onOpenNote={(id) => { setVaultOpen(false); void openNote(id); }}
          onOpenArticle={(id) => { setVaultOpen(false); void openArticle(id); }}
          onClose={() => setVaultOpen(false)}
          notify={notify}
        />
      )}
      {noteLinks && (
        <LinksPicker
          links={noteLinks}
          onPick={(id) => { setNoteLinks(null); void openNote(id); }}
          onClose={() => setNoteLinks(null)}
        />
      )}
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
