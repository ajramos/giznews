import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { api } from "./api";
import { applyTheme, currentTheme } from "./theme";
import type {
  ArticleDTO,
  DigestDTO,
  DigestMeta,
  ListArticlesOptions,
  NoteDTO,
  RuleActionDTO,
  RuleDTO,
  SearchResultDTO,
  SourceDTO,
  StatusDTO,
} from "./types";
import { SourceForm } from "./components/SourceForm";
import { ArticleList } from "./components/ArticleList";
import { Reader } from "./components/Reader";
import { DigestView } from "./components/DigestView";
import { SearchPanel } from "./components/SearchPanel";
import { GraphPanel } from "./components/GraphPanel";
import { CommandPalette, type PaletteCommand } from "./components/CommandPalette";
import { HelpOverlay } from "./components/HelpOverlay";
import { StatusBar } from "./components/StatusBar";
import { Markdown, stars, catClass } from "./components/Markdown";
import { noteBody } from "./frontmatter";
import { WelcomeOverlay } from "./components/WelcomeOverlay";
import { ThemePicker } from "./components/ThemePicker";
import { ThemeModal } from "./components/ThemeModal";
import { SourcePicker } from "./components/SourcePicker";
import { JobsPanel } from "./components/JobsPanel";
import { CategoryPicker, CATEGORIES } from "./components/CategoryPicker";
import { FlowPanel } from "./components/FlowPanel";
import { LogsPanel } from "./components/LogsPanel";
import { ConceptPicker } from "./components/ConceptPicker";
import { RulesPicker } from "./components/RulesPicker";
import { RuleForm } from "./components/RuleForm";
import { PromptModal } from "./components/PromptModal";
import { StatusModal } from "./components/StatusModal";
import { VaultBrowser, type StageKey } from "./components/VaultBrowser";
import { ContextPanel } from "./components/ContextPanel";
import { LinksPicker, type LinkItem } from "./components/LinksPicker";
import { buildNoteLinks, buildArticleLinks } from "./noteLinks";
import { CircleHelp, Command, RefreshCw, Loader2 } from "lucide-react";

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
  const [archived, setArchived] = useState(false); // false = Active (unread+read)
  const [starredFilter, setStarredFilter] = useState<boolean | null>(null); // null = any
  const [readFilter, setReadFilter] = useState<"all" | "unread" | "read">("all"); // sub-filter of Active
  const [filterSource, setFilterSource] = useState<number | null>(null);
  const [filterCategory, setFilterCategory] = useState<string | null>(null);
  const [importanceExact, setImportanceExact] = useState<number | null>(null); // null = any, 0-3
  const [filterUnclassified, setFilterUnclassified] = useState(false);
  const [filterQuery, setFilterQuery] = useState("");
  const [categoryPickerOpen, setCategoryPickerOpen] = useState(false);
  const [flowOpen, setFlowOpen] = useState(false);
  const [logsOpen, setLogsOpen] = useState(false);
  const [rulesPickerOpen, setRulesPickerOpen] = useState(false);
  const [conceptPickerOpen, setConceptPickerOpen] = useState(false);
  const [ruleForm, setRuleForm] = useState<{ initial: RuleDTO | null } | null>(null);
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
  const [jobsOpen, setJobsOpen] = useState(false);
  const [synthPrompt, setSynthPrompt] = useState(false);
  const [urlPrompt, setUrlPrompt] = useState(false);
  const [statusOpen, setStatusOpen] = useState(false);
  const [mode, setMode] = useState<"news" | "vault">("news");
  const [vaultStage, setVaultStage] = useState<StageKey>("atom");
  const [contextOpen, setContextOpen] = useState(false);
  const [noteRevision, setNoteRevision] = useState(0); // bumps when a note is materialized
  const [listWidth, setListWidth] = useState(340);
  const [zoomLevel, setZoomLevel] = useState(1);
  const [noteLinks, setNoteLinks] = useState<LinkItem[] | null>(null);
  const [toast, setToast] = useState<Toast | null>(null);
  const [countBuf, setCountBuf] = useState("");
  const [rangeStatus, setRangeStatus] = useState("");
  const [welcome, setWelcome] = useState(() => {
    try { return !localStorage.getItem("giznews-welcomed"); } catch { return false; }
  });

  // ---- reader / panels ----
  const [reader, setReader] = useState<ArticleDTO | null>(null);
  const [noteReader, setNoteReader] = useState<NoteDTO | null>(null);
  const [paneFocus, setPaneFocus] = useState<"list" | "reader" | "context">("list");
  const [summarizing, setSummarizing] = useState(false);
  const [contentLoading, setContentLoading] = useState(false);
  const [digest, setDigest] = useState<DigestDTO | null>(null);
  const [digestLoading, setDigestLoading] = useState(false);
  const [digestFocusId, setDigestFocusId] = useState<number | null>(null);
  const [digestHistory, setDigestHistory] = useState<DigestMeta[]>([]);
  const [digestDate, setDigestDate] = useState<string | null>(null);
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
  const [runningJobs, setRunningJobs] = useState(0);

  const bulkIds = useMemo(() => [...bulkSel], [bulkSel]);

  const lastGRef = useRef(0);
  const graphTimer = useRef<number | null>(null);
  const busyRef = useRef(false);
  const loadingIdRef = useRef<number | null>(null);
  const searchIndexedRef = useRef(false);
  const articlesRef = useRef(articles);
  articlesRef.current = articles;

  // giztui-style range prefix: press a verb (y/a/t/m), then digits, then the
  // verb again ({op}{digits}{op}); a short timeout applies it as a single op.
  const rangeState = useRef<{ op: string; count: number } | null>(null);
  const rangeTimer = useRef<number | null>(null);

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
      const list = await api.listArticles({
        limit: 400,
        ...(starredFilter === true
          ? { starred: true }
          : archived
            ? { status: "archived" }
            : readFilter === "unread"
              ? { status: "unread" }
              : readFilter === "read"
                ? { status: "read" }
                : { unarchived: true }),
        ...(filterCategory ? { category: filterCategory } : {}),
        ...(importanceExact != null ? { importanceExact } : {}),
        ...(filterUnclassified ? { unclassified: true } : {}),
        ...(filterQuery ? { query: filterQuery } : {}),
        ...opts,
      });
      setArticles(list);
      setSelectedIndex((i) => Math.min(i, Math.max(0, list.length - 1)));
    } catch (e) {
      notify(String(e));
    } finally {
      setLoadingList(false);
    }
  }, [notify, archived, starredFilter, readFilter, filterCategory, importanceExact, filterUnclassified, filterQuery]);

  const reloadAll = useCallback(() => {
    void loadSources();
    void loadArticles();
    void loadStatus();
  }, [loadSources, loadArticles, loadStatus]);

  // load once on mount; the list reloads whenever its filter state changes.
  useEffect(() => {
    void loadSources();
    void loadArticles();
    void loadStatus();
    void loadDigestHistory();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);
  useEffect(() => { void loadArticles(); }, [loadArticles]);

  // Poll the jobs registry to drive the topbar "running" indicator.
  useEffect(() => {
    let alive = true;
    const load = () => api.listJobs().then((j) => {
      if (alive) setRunningJobs(j.filter((x) => x.status === "running").length);
    }).catch(() => {});
    load();
    const iv = window.setInterval(load, 2500);
    return () => { alive = false; window.clearInterval(iv); };
  }, []);

  // auto-refresh: quietly fetch new articles every 15 minutes.
  useEffect(() => {
    if (!autoRefresh) return;
    const iv = window.setInterval(async () => {
      try {
        const r = await api.fetch();
        if (r.newArticles > 0) notify(`${r.newArticles} new articles`);
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

  const goNews = useCallback(() => {
    setDigestOpen(false);
    setPanel("none");
    setMode("news");
    setNoteReader(null);
    setPaneFocus("list");
  }, []);

  // Active (unread+read) vs Archived, with a unread/read sub-filter on Active.
  const goActive = useCallback(() => { setArchived(false); setStarredFilter(null); setReadFilter("all"); goNews(); }, [goNews]);
  const goUnread = useCallback(() => { setArchived(false); setStarredFilter(null); setReadFilter("unread"); goNews(); }, [goNews]);
  const goRead = useCallback(() => { setArchived(false); setStarredFilter(null); setReadFilter("read"); goNews(); }, [goNews]);
  const goArchived = useCallback(() => { setArchived(true); setStarredFilter(null); goNews(); }, [goNews]);
  // Starred view shows starred articles regardless of read/archived status.
  const toggleStarred = useCallback(() => { setArchived(false); setStarredFilter((v) => (v === true ? null : true)); }, []);

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
      // Reading a note means being in the knowledge world: the vault browser
      // becomes the master list (articles are no longer shown).
      setMode("vault");
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

  // Open the links picker for an ARTICLE being read: links embedded in its
  // body, its external URL, plus its Atom note's connections when one exists.
  const openArticleLinks = useCallback(async (articleId: number) => {
    try {
      const art = reader; // the full article (with content_md)
      if (!art || art.id !== articleId) return;
      const note = await api.getArticleNote(articleId);
      const notes = await api.listNotes("");
      setNoteLinks(buildArticleLinks(art.url, art.contentMD ?? "", note, notes));
    } catch (e) { notify(String(e)); }
  }, [reader, notify]);

  const selected = articles[selectedIndex] ?? null;

  // Re-fetch the open article so the reader reflects classification/summary.
  const refreshReader = useCallback(async () => {
    if (!reader) return;
    try {
      const full = await api.getArticleContent(reader.id);
      setReader(full);
    } catch { /* ignore */ }
  }, [reader]);

  const exitBulk = useCallback(() => { setBulk(false); setBulkSel(new Set()); }, []);

  // toggle one article in the bulk selection (space key or checkbox click).
  const toggleBulkId = useCallback((id: number) => {
    setBulkSel((prev) => {
      const next = new Set(prev);
      if (next.has(id)) next.delete(id);
      else next.add(id);
      return next;
    });
  }, []);

  const archiveIds = useCallback((ids: number[]) => {
    if (ids.length === 0) return;
    const batch = ids.map((id) => articles.find((a) => a.id === id)).filter((a): a is ArticleDTO => !!a);
    if (batch.length === 0) return;
    const restoring = batch[0].status === "archived";
    const undo: Record<number, string> = {};
    for (const a of batch) undo[a.id] = a.status;
    const target = restoring ? "unread" : "archived";
    void (async () => {
      try {
        await api.bulkSetStatus(batch.map((a) => a.id), target);
        setArticles((prev) => restoring ? prev : prev.filter((a) => !batch.some((b) => b.id === a.id)));
        if (!restoring) setSelectedIndex((i) => Math.max(0, i - 1));
        void loadStatus();
        notify(`${batch.length} archived`, () => {
          void Promise.all(batch.map((a) => api.setArticleStatus(a.id, undo[a.id])));
          void loadArticles();
        });
      } catch (e) { notify(String(e)); }
    })();
  }, [articles, notify, loadArticles, loadStatus]);

  const archiveRange = useCallback((count: number) => {
    archiveIds(articles.slice(selectedIndex, selectedIndex + Math.max(1, count)).map((a) => a.id));
  }, [articles, selectedIndex, archiveIds]);

  const toggleReadRange = useCallback((count: number) => {
    const batch = articles.slice(selectedIndex, selectedIndex + Math.max(1, count));
    const targets = batch.filter((a) => a.status !== "archived");
    if (targets.length === 0) return;
    const toRead = targets[0].status !== "read";
    void (async () => {
      try {
        await Promise.all(targets.map((a) => api.setArticleStatus(a.id, toRead ? "read" : "unread")));
        notify(toRead ? `Marked read (${targets.length})` : `Marked unread (${targets.length})`);
        void loadArticles();
      } catch (e) { notify(String(e)); }
    })();
  }, [articles, selectedIndex, notify, loadArticles]);

  const toggleStarRange = useCallback((count: number) => {
    const batch = articles.slice(selectedIndex, selectedIndex + Math.max(1, count));
    const targets = batch.filter((a) => a.status !== "archived");
    if (targets.length === 0) return;
    const starring = !targets[0].starred;
    void (async () => {
      try {
        await Promise.all(targets.map((a) => api.setArticleStarred(a.id, starring)));
        notify(starring ? `Starred (${targets.length})` : `Unstarred (${targets.length})`);
        void loadArticles();
      } catch (e) { notify(String(e)); }
    })();
  }, [articles, selectedIndex, notify, loadArticles]);

  const summarizeRange = useCallback((count: number) => {
    const batch = articles.slice(selectedIndex, selectedIndex + Math.max(1, count));
    if (batch.length === 0) return;
    void (async () => {
      try {
        await Promise.all(batch.map((a) => api.summarizeArticle(a.id)));
        notify(`Summarized ${batch.length}`);
        void loadArticles();
        void refreshReader();
      } catch (e) { notify(String(e)); }
    })();
  }, [articles, selectedIndex, notify, loadArticles, refreshReader]);

  // ---- giztui-style range prefix ({op}{digits}{op}) ----
  const RANGE_VERBS = ["y", "a", "t", "m"];
  const executeRange = useCallback((op: string, count: number) => {
    const n = Math.max(1, count);
    if (op === "t") toggleReadRange(n);
    else if (op === "a") archiveRange(n);
    else if (op === "m") toggleStarRange(n);
    else if (op === "y") summarizeRange(n);
  }, [toggleReadRange, archiveRange, toggleStarRange, summarizeRange]);

  const startRange = useCallback((op: string) => {
    rangeState.current = { op, count: 0 };
    setRangeStatus(`${op}…`);
    setCountBuf("");
    if (rangeTimer.current != null) window.clearTimeout(rangeTimer.current);
    rangeTimer.current = window.setTimeout(() => {
      const st = rangeState.current;
      rangeState.current = null;
      setRangeStatus("");
      if (st) executeRange(st.op, st.count > 0 ? st.count : 1);
    }, 800);
  }, [executeRange]);

  const completeRange = useCallback((op: string) => {
    const st = rangeState.current;
    rangeState.current = null;
    if (rangeTimer.current != null) window.clearTimeout(rangeTimer.current);
    setRangeStatus("");
    executeRange(op, st && st.count > 0 ? st.count : 1);
  }, [executeRange]);

  const appendRangeDigit = useCallback((d: number) => {
    if (!rangeState.current) return;
    rangeState.current.count = rangeState.current.count * 10 + d;
    setRangeStatus(`${rangeState.current.op}${rangeState.current.count}…`);
  }, []);

  const cancelRange = useCallback(() => {
    if (rangeState.current == null) return;
    rangeState.current = null;
    if (rangeTimer.current != null) window.clearTimeout(rangeTimer.current);
    setRangeStatus("");
  }, []);

  // classify an explicit set of articles as a background job.
  const classifyIds = useCallback((ids: number[]) => {
    if (ids.length === 0) return;
    setJobsOpen(true);
    void (async () => {
      try {
        const r = await api.classifyArticles(ids);
        notify(`${r.classified} of ${ids.length} classified (${r.byRules} rules · ${r.byLLM} LLM)`);
        // Drop the "unclassified" filter so the now-classified articles stay
        // visible with their category/importance chips.
        setFilterUnclassified(false);
        await loadArticles();
        await loadStatus();
        void refreshReader();
      } catch (e) { notify(String(e)); }
    })();
  }, [notify, loadArticles, loadStatus, refreshReader]);

  // classify the bulk selection (priority over the queue).
  const classifySelected = useCallback(() => {
    if (bulkIds.length === 0) return;
    const ids = bulkIds;
    exitBulk();
    classifyIds(ids);
  }, [bulkIds, exitBulk, classifyIds]);

  // materialize knowledge notes for a set of articles (atom note each).
  const materializeArticles = useCallback((ids: number[]) => {
    if (ids.length === 0) return;
    void (async () => {
      try {
        await Promise.all(ids.map((id) => api.ensureArticleNote(id)));
        notify(ids.length === 1 ? "Note created — run :kb build for concepts" : `${ids.length} note(s) created — run :kb build for concepts`);
        setNoteRevision((r) => r + 1);
        setGraphRefresh((r) => r + 1);
        void loadStatus();
      } catch (e) { notify(String(e)); }
    })();
  }, [notify, loadStatus]);

  // materialize knowledge notes for the bulk selection (kb build for these).
  const processSelected = useCallback(() => {
    if (bulkIds.length === 0) return;
    const ids = bulkIds;
    exitBulk();
    materializeArticles(ids);
  }, [bulkIds, exitBulk, materializeArticles]);

  // summarize the bulk selection (each article gets its own background job).
  const summarizeSelected = useCallback(() => {
    if (bulkIds.length === 0) return;
    const ids = bulkIds;
    exitBulk();
    void (async () => {
      try {
        await Promise.all(ids.map((id) => api.summarizeArticle(id)));
        notify(`${ids.length} summarized`);
        void loadArticles();
      } catch (e) { notify(String(e)); }
    })();
  }, [bulkIds, exitBulk, notify, loadArticles]);

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
          await api.setArticleStarred(a.id, !a.starred);
        }, () => void loadArticles());
      }
      exitBulk();
      return true;
    }
    return false;
  }, [bulk, bulkIds, archiveIds, applyToIds, exitBulk, loadArticles]);

  const starReader = useCallback(async () => {
    if (!reader) return;
    const next = !reader.starred;
    await api.setArticleStarred(reader.id, next);
    setReader((r) => (r && r.id === reader.id ? { ...r, starred: next } : r));
    setArticles((prev) => prev.map((a) => (a.id === reader.id ? { ...a, starred: next } : a)));
    notify(next ? "Starred" : "Unstarred");
  }, [reader, notify]);

  const archiveReader = useCallback(async () => {
    if (!reader) return;
    const toArchived = reader.status !== "archived";
    await api.setArticleStatus(reader.id, toArchived ? "archived" : "unread");
    const status = toArchived ? "archived" : "unread";
    setReader((r) => (r && r.id === reader.id ? { ...r, status } : r));
    setArticles((prev) => prev.map((a) => (a.id === reader.id ? { ...a, status } : a)));
    notify(toArchived ? "Archived" : "Unarchived");
    void loadStatus();
  }, [reader, notify, loadStatus]);

  const summarize = useCallback(async () => {
    if (!reader || busyRef.current) return;
    busyRef.current = true; setSummarizing(true);
    try {
      const updated = await api.summarizeArticle(reader.id);
      setReader(updated);
      setArticles((prev) => prev.map((a) => (a.id === updated.id ? { ...a, summary: updated.summary } : a)));
      notify("Summary generated");
    } catch (e) { notify(String(e)); }
    finally { busyRef.current = false; setSummarizing(false); }
  }, [reader, notify]);

  const openExternal = useCallback(() => {
    const url = reader?.url || selected?.url;
    if (url) void api.openURL(url);
  }, [reader, selected]);

  const scrollReader = useCallback((dir: number) => {
    const el = document.querySelector<HTMLElement>(".reader-scroll");
    if (el) el.scrollBy({ top: dir * el.clientHeight * 0.9 });
  }, []);

  // Font zoom (cmd +/- / cmd 0): scales the whole app via CSS zoom (WebKit).
  const zoomBy = useCallback((delta: number) => {
    setZoomLevel((z) => Math.max(0.8, Math.min(1.6, Math.round((z + delta) * 10) / 10)));
  }, []);
  const zoomReset = useCallback(() => setZoomLevel(1), []);
  useEffect(() => {
    document.documentElement.style.zoom = String(zoomLevel);
  }, [zoomLevel]);

  const openAdjacent = useCallback((delta: number) => {
    const n = Math.max(0, articles.length - 1);
    setSelectedIndex((i) => Math.max(0, Math.min(i + delta, n)));
  }, [articles.length]);

  // ---- lazy loading: the selected article loads automatically (debounced),
  // and the adjacent ones are prefetched so navigation is instant. ----
  // Paused while a modal/panel is open (checked via ref inside the timer, so
  // closing a modal does NOT re-trigger a load that would clobber a note).
  const modalOpen =
    paletteOpen || helpOpen || sourcePickerOpen || themeModalOpen ||
    jobsOpen || categoryPickerOpen || flowOpen || sourceForm != null || deleteSource != null ||
    digestOpen || panel !== "none" || synthPrompt || urlPrompt || statusOpen || noteLinks != null;
  const modalOpenRef = useRef(modalOpen);
  modalOpenRef.current = modalOpen;
  const noteReaderRef = useRef<NoteDTO | null>(null);
  noteReaderRef.current = noteReader;

  useEffect(() => {
    if (mode !== "news") return;
    const art = articlesRef.current[selectedIndex];
    if (!art) return;
    const t = window.setTimeout(() => {
      // Don't auto-load an article while a modal is open or a note is being
      // read — it would clobber the note the user just opened.
      if (modalOpenRef.current || noteReaderRef.current) return;
      void openArticle(art.id, true); // silent: never clobber an open note
    }, 120);
    return () => window.clearTimeout(t);
  }, [selectedIndex, articles.length, openArticle, mode]);

  useEffect(() => {
    const next = articlesRef.current[selectedIndex + 1];
    const prev = articlesRef.current[selectedIndex - 1];
    if (next) void api.getArticleContent(next.id).catch(() => {});
    if (prev) void api.getArticleContent(prev.id).catch(() => {});
  }, [selectedIndex, articles.length]);

  // ---- panels ----
  const loadDigestHistory = useCallback(() => {
    api.listDigests().then(setDigestHistory).catch(() => {});
  }, []);

  const generateDigest = useCallback(async () => {
    if (busyRef.current) return;
    busyRef.current = true; setDigestLoading(true);
    try {
      const d = await api.digest();
      setDigest(d);
      setDigestFocusId(d.themes[0]?.articles[0]?.id ?? null);
      setDigestDate(null);
      setDigestOpen(true);
      void loadDigestHistory();
    } catch (e) { notify(String(e)); }
    finally { busyRef.current = false; setDigestLoading(false); }
  }, [notify, loadDigestHistory]);

  const selectDigest = useCallback(async (date: string | null) => {
    if (!date) { void generateDigest(); return; }
    setDigestLoading(true);
    try {
      const d = await api.getDigest(date);
      if (d) {
        setDigest(d);
        setDigestFocusId(d.themes[0]?.articles[0]?.id ?? null);
        setDigestDate(date);
        setDigestOpen(true);
      }
    } catch (e) { notify(String(e)); }
    finally { setDigestLoading(false); }
  }, [generateDigest, notify]);

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
      notify("Note generated for this article");
      setGraphRefresh((r) => r + 1);
      await openGraph();
    } catch (e) { notify(String(e)); }
  }, [selected, openGraph, notify]);

  // create a note for a specific article (from the context pane).
  const createNoteForArticle = useCallback((articleId: number) => {
    materializeArticles([articleId]);
  }, [materializeArticles]);

  // open the graph focused on a specific note (from the context pane).
  const openGraphForNote = useCallback((noteId: number | null) => {
    setGraphFocusId(noteId);
    setPanel("graph");
  }, []);

  const runSearch = useCallback(async (q: string) => {
    setSearching(true);
    try {
      setSearchResults(q.trim() ? await api.search(q, 20) : []);
      setSearchFocus(0);
    } catch (e) { notify(String(e)); }
    finally { setSearching(false); }
  }, [notify]);

  // ---- commands ----
  const runCmd = useCallback(async (fn: () => Promise<unknown>) => {
    try { await fn(); } catch (e) { notify(String(e)); }
  }, [notify]);

  // Run the full pipeline (fetch → classify → kb → index) as sequential
  // background jobs; the jobs panel stays open so progress is visible.
  const runProcess = useCallback(async () => {
    setJobsOpen(true);
    try {
      await api.fetch();
      await reloadAll();
      await api.classify(500);
      await loadArticles();
      void refreshReader();
      await api.kbuild();
      await loadStatus();
      await api.searchIndex();
      notify("Process completed");
    } catch (e) { notify(String(e)); }
  }, [reloadAll, loadArticles, loadStatus, notify, refreshReader]);

  const addByURL = useCallback(async (url: string) => {
    try {
      const art = await api.ingestURL(url);
      notify(`Added: ${art.title}`);
      await reloadAll();
      if (art.id) void openArticle(art.id);
    } catch (e) { notify(String(e)); }
  }, [reloadAll, openArticle, notify]);

  const commands = useMemo<PaletteCommand[]>(() => [
    { name: "process", hint: "Full pipeline: fetch → classify → kb → index", run: () => void runProcess() },
    { name: "fetch", hint: "Fetch new articles (+ extract bodies)", run: () => {
      setJobsOpen(true);
      void runCmd(async () => {
        const r = await api.fetch();
        await reloadAll();
        notify(`${r.newArticles} new${r.extracted ? ` · ${r.extracted} extracted` : ""}`);
      });
    } },
    { name: "classify", hint: "Classify (rules + LLM)", run: () => {
      setJobsOpen(true);
      void runCmd(async () => {
        const c = await api.classify(200);
        await loadArticles();
        void refreshReader();
        notify(`${c.classified} classified (${c.byRules} rules · ${c.byLLM} LLM)`);
      });
    } },
    { name: "kb build", hint: "Generate atoms/electrons", run: () => {
      setJobsOpen(true);
      void runCmd(async () => {
        const k = await api.kbuild();
        await loadStatus();
        notify(`${k.atomsCreated} atoms · ${k.electronsCreated} electrons`);
      });
    } },
    { name: "kb synth <category>", hint: "Synthesize a category into a molecule", run: () => setSynthPrompt(true) },
    { name: "search index", hint: "Rebuild search index + embeddings", run: () => {
      setJobsOpen(true);
      void runCmd(async () => { await api.searchIndex(); notify("Search index updated"); });
    } },
    { name: "digest", hint: "Daily digest", run: () => void generateDigest() },
    { name: "url", hint: "Add an article by URL", run: () => setUrlPrompt(true) },
    { name: "jobs", hint: "Background jobs", run: () => setJobsOpen(true) },
    { name: "flow", hint: "Pipeline flow (live counts)", run: () => setFlowOpen(true) },
    { name: "logs", hint: "Pipeline log (what the app decided)", run: () => setLogsOpen(true) },
    { name: "rules", hint: "Deterministic classification rules", run: () => setRulesPickerOpen(true) },
    { name: "concepts", hint: "Concepts by mentions (promote, merge)", run: () => setConceptPickerOpen(true) },
    { name: "auto-refresh", hint: autoRefresh ? "Disable auto-refresh" : "Enable auto-refresh (15 min)", run: () => setAutoRefresh((v) => !v) },
    { name: "sources", hint: "Manage sources", run: () => setSourcePickerOpen(true) },
    { name: "vault", hint: "Knowledge vault (electrons → atoms → molecules)", run: () => setMode("vault") },
    { name: "news", hint: "Back to the news feed", run: () => { setMode("news"); setNoteReader(null); setPaneFocus("list"); } },
    { name: "status", hint: "Status (articles, notes, LLM)", run: () => setStatusOpen(true) },
    { name: "add-source", hint: "Add a source (RSS/HN/arXiv/gmail)", run: () => { setSourcePickerOpen(false); setSourceForm({ initial: null }); } },
    { name: "theme", hint: "Choose theme", run: () => setThemeModalOpen(true) },
    { name: "open vault", hint: "Open vault in Obsidian", run: () => void api.openVault() },
    { name: "quit", hint: "Quit GizNews", run: () => void api.quit() },
  ], [runCmd, runProcess, generateDigest, reloadAll, loadArticles, loadStatus, refreshReader, theme, autoRefresh]);

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

      // Native shortcuts (copy/paste/quit) pass through untouched. Only zoom
      // and page-up/down combos are handled here.
      if (e.metaKey) {
        if (k === "=" || k === "+") { e.preventDefault(); zoomBy(0.1); return; }
        if (k === "-") { e.preventDefault(); zoomBy(-0.1); return; }
        if (k === "0") { e.preventDefault(); zoomReset(); return; }
        return;
      }
      if (e.ctrlKey) {
        if (k === "d") { e.preventDefault(); if (reader) scrollReader(0.9); else setSelectedIndex((i) => Math.min(i + 8, Math.max(0, articles.length - 1))); return; }
        if (k === "u") { e.preventDefault(); if (reader) scrollReader(-0.9); else setSelectedIndex((i) => Math.max(i - 8, 0)); return; }
        return;
      }
      if (e.altKey) return;

      // A pending range op is cancelled by any key that doesn't continue it.
      if (rangeState.current && !(k >= "0" && k <= "9") && !RANGE_VERBS.includes(k)) {
        cancelRange();
      }

      if (k === "Escape") {
        // Close the topmost in-app layer first. Only when something is actually
        // closed do we consume the key (preventDefault) — otherwise a bare
        // Escape (nothing open) falls through to the native macOS fullscreen
        // exit, so the second Escape "un-maximizes".
        const closed =
          noteLinks ? (setNoteLinks(null), true) :
          bulk ? (exitBulk(), true) :
          paletteOpen ? (setPaletteOpen(false), true) :
          helpOpen ? (setHelpOpen(false), true) :
          themeModalOpen ? (setThemeModalOpen(false), true) :
          sourcePickerOpen ? (setSourcePickerOpen(false), true) :
          jobsOpen ? (setJobsOpen(false), true) :
          categoryPickerOpen ? (setCategoryPickerOpen(false), true) :
          flowOpen ? (setFlowOpen(false), true) :
          logsOpen ? (setLogsOpen(false), true) :
          rulesPickerOpen ? (setRulesPickerOpen(false), true) :
          conceptPickerOpen ? (setConceptPickerOpen(false), true) :
          ruleForm ? (setRuleForm(null), true) :
          synthPrompt ? (setSynthPrompt(false), true) :
          urlPrompt ? (setUrlPrompt(false), true) :
          statusOpen ? (setStatusOpen(false), true) :
          sourceForm ? (setSourceForm(null), true) :
          deleteSource ? (setDeleteSource(null), true) :
          panel !== "none" ? (setPanel("none"), true) :
          digestOpen ? (setDigestOpen(false), true) :
          noteReader ? (setNoteReader(null), true) :
          paneFocus === "reader" ? (setPaneFocus("list"), true) :
          paneFocus === "context" ? (setPaneFocus("list"), true) :
          false;
        if (closed) {
          e.preventDefault();
          e.stopPropagation();
        }
        return;
      }
      if (typing) return; // inputs handle their own keys
      if (noteLinks || paletteOpen || helpOpen || sourceForm || deleteSource || themeModalOpen || sourcePickerOpen || jobsOpen || categoryPickerOpen || flowOpen || logsOpen || rulesPickerOpen || ruleForm || synthPrompt || urlPrompt || statusOpen) return;

      // Tab cycles pane focus: list → reader → context (skips a collapsed context).
      if (k === "Tab") {
        e.preventDefault();
        const order: ("list" | "reader" | "context")[] = contextOpen ? ["list", "reader", "context"] : ["list", "reader"];
        const i = order.indexOf(paneFocus);
        const next = e.shiftKey ? (i - 1 + order.length) % order.length : (i + 1) % order.length;
        setPaneFocus(order[next]);
        return;
      }

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
        e.preventDefault(); // consume bulk keys (space must not scroll the list)
        if (k === "j" || k === "ArrowDown") { setSelectedIndex((i) => Math.min(i + 1, bn)); return; }
        if (k === "k" || k === "ArrowUp") { setSelectedIndex((i) => Math.max(i - 1, 0)); return; }
        if (k === " ") {
          const id = selected?.id;
          if (id != null) toggleBulkId(id);
          return;
        }
        if (k === "a") { bulkAction("archive"); return; }
        if (k === "t") { bulkAction("read"); return; }
        if (k === "m") { bulkAction("star"); return; }
        if (k === "c") { classifySelected(); return; }
        if (k === "p") { processSelected(); return; }
        if (k === "y") { summarizeSelected(); return; }
        return; // consume everything else while in bulk
      }

      // count prefix
      if (k >= "0" && k <= "9") {
        if (rangeState.current) { appendRangeDigit(Number(k)); return; }
        if (countBuf.length < 2) setCountBuf((c) => c + k);
        return;
      }
      const count = countBuf ? parseInt(countBuf, 10) || 1 : 1;
      if (countBuf) setCountBuf("");
      clearGraphTimer();
      e.preventDefault();

      const n = Math.max(0, articles.length - 1);

      // Reader focus: j/k/arrows/space scroll the detail; L opens links; Esc
      // returns focus to the list.
      if (paneFocus === "reader") {
        if (k === "j" || k === "ArrowDown") { scrollReader(0.35); return; }
        if (k === "k" || k === "ArrowUp") { scrollReader(-0.35); return; }
        if (k === " ") { scrollReader(e.shiftKey ? -0.9 : 0.9); return; }
        if (k === "L") {
          if (noteReader) void openNoteLinks(noteReader.id);
          else if (reader) void openArticleLinks(reader.id);
          return;
        }
      }

      // Article reader (list focus): L shows the article's knowledge-graph connections.
      if (reader && k === "L") { void openArticleLinks(reader.id); return; }

      // reader paging (space) when an article is open
      if (k === " ") {
        scrollReader(e.shiftKey ? -0.9 : 0.9);
        return;
      }

      if (k === "q") { void api.quit(); return; }

      // News-world list navigation + actions (inactive in the vault world).
      if (mode === "news") {
        if (k === "J") { openAdjacent(count); return; }
        if (k === "K") { openAdjacent(-count); return; }
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
        if (k === "Enter") { if (selected) { void openArticle(selected.id); setPaneFocus("reader"); } return; }
        // range verbs: {op} starts a count sequence, {op}{digits}{op} applies it.
        if (RANGE_VERBS.includes(k)) {
          if (rangeState.current?.op === k) completeRange(k);
          else startRange(k);
          return;
        }
        if (k === "O" || k === "o") { openExternal(); return; }
        if (k === "c") { if (selected) classifyIds([selected.id]); return; }
        if (k === "p") { if (selected) materializeArticles([selected.id]); return; }
        // ←/→ cycle the category filter through All → models → … → general.
        if (k === "ArrowRight") { setFilterCategory((c) => { const seq = [null, ...CATEGORIES]; const i = seq.indexOf(c); return seq[(i + 1) % seq.length]; }); return; }
        if (k === "ArrowLeft") { setFilterCategory((c) => { const seq = [null, ...CATEGORIES]; const i = seq.indexOf(c); return seq[(i - 1 + seq.length) % seq.length]; }); return; }
      }
      if (k === "s") {
        setPanel((p) => (p === "search" ? "none" : "search"));
        if (!searchIndexedRef.current) {
          searchIndexedRef.current = true;
          void api.searchIndex().catch(() => {});
        }
        return;
      }
      if (k === "n") { setVaultStage("atom"); setMode("vault"); setPaneFocus("list"); return; }
      if (k === "f") { setMode((m) => (m === "vault" ? "news" : "vault")); setPaneFocus("list"); return; }
      if (k === "z") { setJobsOpen(true); return; }
      if (k === ";") { setCategoryPickerOpen(true); return; }
      if (k === "/") { document.querySelector<HTMLInputElement>(".list-search input")?.focus(); return; }
      if (k === "C") { setContextOpen((v) => !v); return; }
      if (k === "[") { setImportanceExact((v) => { const seq = [null, 0, 1, 2, 3]; const i = seq.indexOf(v); return seq[(i - 1 + seq.length) % seq.length]; }); return; }
      if (k === "]") { setImportanceExact((v) => { const seq = [null, 0, 1, 2, 3]; const i = seq.indexOf(v); return seq[(i + 1) % seq.length]; }); return; }
      if (k === "d") { void generateDigest(); return; }
      if (k === ":") { setPaletteOpen(true); return; }
      if (k === "?") { setHelpOpen(true); return; }
      if (k === "u" && !e.ctrlKey) { goUnread(); return; }
      if (k === "r") { goRead(); return; }
      if (k === "x") { goArchived(); return; }
      if (k === "*") { toggleStarred(); return; }
    };
    window.addEventListener("keydown", handler);
    return () => window.removeEventListener("keydown", handler);
  }, [paletteOpen, helpOpen, sourceForm, deleteSource, themeModalOpen, sourcePickerOpen, jobsOpen, categoryPickerOpen, flowOpen, logsOpen, rulesPickerOpen, conceptPickerOpen, ruleForm, synthPrompt, urlPrompt, statusOpen, noteLinks, panel, digestOpen, noteReader, paneFocus, contextOpen, mode, countBuf, articles.length, selected, selectedIndex, openArticle, openExternal, openGraph, goUnread, goRead, goArchived, toggleStarred, moveDigestFocus, openDigestFocus, scrollReader, openAdjacent, openNoteLinks, openArticleLinks, reader, bulk, exitBulk, toggleBulkId, bulkAction, classifyIds, classifySelected, materializeArticles, processSelected, summarizeSelected, executeRange, startRange, completeRange, appendRangeDigit, cancelRange, zoomBy, zoomReset]);

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
      notify(`Source added: ${data.name}`);
    } catch (e) { notify(String(e)); }
  }, [loadSources, notify]);

  const saveRule = useCallback(async (data: { name: string; query: string; actions: RuleActionDTO[]; enabled: boolean }) => {
    try {
      if (ruleForm?.initial) {
        await api.updateRule(ruleForm.initial.id, data.name, data.query, data.actions, data.enabled);
      } else {
        await api.addRule(data.name, data.query, data.actions, data.enabled);
      }
      setRuleForm(null);
      notify(`Rule saved: ${data.name}`);
    } catch (e) { notify(String(e)); }
  }, [ruleForm, notify]);

  const confirmDeleteSource = useCallback(async () => {
    if (!deleteSource) return;
    try {
      // logical delete: remove from registry, articles are preserved.
      await api.setSourceEnabled(deleteSource.id, false);
      await api.deleteSource(deleteSource.id);
      if (filterSource === deleteSource.id) await selectSource(null);
      await loadSources();
      notify(`Source removed (articles kept)`);
    } catch (e) { notify(String(e)); }
    finally { setDeleteSource(null); }
  }, [deleteSource, filterSource, selectSource, loadSources, notify]);

  const uiCtx: "list" | "reader" | "search" | "graph" | "digest" | "vault" =
    mode === "vault" ? "vault"
    : panel === "search" ? "search"
    : panel === "graph" ? "graph"
    : digestOpen ? "digest"
    : reader ? "reader"
    : "list";

  const modeLabel = mode === "vault" ? "VAULT"
    : panel === "search" ? "SEARCH"
    : panel === "graph" ? "GRAPH"
    : digestOpen ? "DIGEST"
    : reader ? "READER"
    : "LIST";

  const filterLabel = filterSource
    ? `source: ${sources.find((s) => s.id === filterSource)?.name ?? "?"}`
    : undefined;

  const onSplitterDown = useCallback((e: React.MouseEvent) => {
    e.preventDefault();
    const startX = e.clientX;
    const startW = listWidth;
    const move = (ev: MouseEvent) => setListWidth(Math.max(240, Math.min(560, startW + ev.clientX - startX)));
    const up = () => {
      window.removeEventListener("mousemove", move);
      window.removeEventListener("mouseup", up);
      document.body.style.cursor = "";
    };
    document.body.style.cursor = "col-resize";
    window.addEventListener("mousemove", move);
    window.addEventListener("mouseup", up);
  }, [listWidth]);

  return (
    <div className="app">
      <header className="topbar">
        <div className="brand">
          <span className="brand-mark">G</span>
          <span className="brand-name">Giz<em>News</em></span>
        </div>
        {status && (
          <div className="status">
            <button
              className="pill mode-toggle"
              onClick={() => { setMode((m) => (m === "vault" ? "news" : "vault")); setPaneFocus("list"); }}
              title="Switch world"
            >
              {mode === "vault" ? "Vault" : "News"}
            </button>
            {runningJobs > 0 && (
              <button className="pill jobs" onClick={() => setJobsOpen(true)} title="Background jobs">
                <Loader2 size={13} className="spin" /> {runningJobs} running
              </button>
            )}
            {filterSource && (
              <button className="pill filter" onClick={() => void selectSource(null)}>
                ✕ {filterLabel}
              </button>
            )}
            {filterCategory && (
              <button className="pill filter" onClick={() => setFilterCategory(null)}>
                ✕ {filterCategory}
              </button>
            )}
            {importanceExact != null && (
              <button className="pill filter" onClick={() => setImportanceExact(null)}>
                ✕ {importanceExact === 0 ? "0★" : `=${importanceExact}★`}
              </button>
            )}
            {filterUnclassified && (
              <button className="pill filter" onClick={() => setFilterUnclassified(false)}>
                ✕ unclassified
              </button>
            )}
          </div>
        )}
        <div className="topbar-actions">
          <ThemePicker value={theme} onChange={(t) => { applyTheme(t); setTheme(t); }} />
          <button className="icon-btn" onClick={() => void reloadAll()} title="Reload"><RefreshCw size={15} /></button>
          <button className="icon-btn" onClick={() => setHelpOpen(true)} title="Help (?)"><CircleHelp size={15} /></button>
          <button className="icon-btn" onClick={() => setPaletteOpen(true)} title="Commands (:)"><Command size={15} /></button>
        </div>
      </header>

      <main className="layout" style={{ gridTemplateColumns: `${listWidth}px 4px minmax(0, 1fr) auto` }}>
        <section className="col list-col">
          {mode === "vault" ? (
            <VaultBrowser
              stage={vaultStage}
              onStage={setVaultStage}
              onOpenNote={(id) => void openNote(id)}
              onFocus={() => setPaneFocus("reader")}
              onClose={() => setMode("news")}
              active={paneFocus === "list"}
              notify={notify}
            />
          ) : (
            <ArticleList
              articles={articles}
              selectedIndex={selectedIndex}
              loading={loadingList}
              archived={archived}
              starredFilter={starredFilter}
              hasSources={sources.length > 0}
              bulk={bulk}
              bulkSel={bulkSel}
              unreadCount={status?.unreadArticles ?? 0}
              filterCategory={filterCategory}
              importanceExact={importanceExact}
              filterUnclassified={filterUnclassified}
              filterQuery={filterQuery}
              onActive={goActive}
              onArchived={goArchived}
              onStarred={toggleStarred}
              readFilter={readFilter}
              onReadFilter={setReadFilter}
              onCategory={(c) => setFilterCategory(c)}
              onImportance={(n) => setImportanceExact(n)}
              onUnclassified={(v) => setFilterUnclassified(v)}
              onQuery={(q) => setFilterQuery(q)}
              onToggleBulk={toggleBulkId}
              onSelect={(i) => { setSelectedIndex(i); setPaneFocus("list"); if (reader) setReader(null); }}
            />
          )}
        </section>

        <div className="splitter" onMouseDown={onSplitterDown} title="Drag to resize" />

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
          ) : digestOpen ? (
            <DigestView
              digest={digest}
              loading={digestLoading}
              unreadCount={status?.unreadArticles ?? 0}
              focusId={digestFocusId}
              history={digestHistory}
              selectedDate={digestDate}
              onFocus={setDigestFocusId}
              onGenerate={generateDigest}
              onSelectDate={selectDigest}
              onOpenArticle={(id) => {
                setDigestOpen(false);
                const idx = articles.findIndex((a) => a.id === id);
                if (idx >= 0) setSelectedIndex(idx);
                void openArticle(id);
              }}
            />
          ) : noteReader ? (
            <div className="reader">
              <div className="reader-head">
                <div className="note-meta">
                  <span className="note-type">{noteReader.type}</span>
                  {noteReader.category && <span className={`cat-chip ${catClass(noteReader.category)}`}>{noteReader.category}</span>}
                  {noteReader.rating != null && <span className="note-rating">{stars(noteReader.rating)}</span>}
                  {noteReader.source && <span className="note-source">{noteReader.source}</span>}
                  <span className="note-created">{noteReader.createdAt ? noteReader.createdAt.slice(0, 10) : ""}</span>
                </div>
                <h1>{noteReader.title}</h1>
                {(noteReader.tags?.length ?? 0) > 0 && (
                  <div className="ctx-tags">{(noteReader.tags ?? []).map((t) => <span key={t} className="tag">#{t}</span>)}</div>
                )}
              </div>
              <div className="reader-scroll">
                <Markdown content={noteBody(noteReader.content)} />
              </div>
            </div>
          ) : (
            <Reader
              article={reader}
              summarizing={summarizing}
              contentLoading={contentLoading}
              llmAvailable={!!status?.llmEnabled && !!status?.llmReachable}
              onSummarize={summarize}
              onArchive={() => void archiveReader()}
              onStar={() => void starReader()}
              onOpenLink={openExternal}
            />
          )}
        </section>

        {contextOpen ? (
          <section className="col context-col">
            <ContextPanel
              article={reader}
              note={noteReader}
              active={paneFocus === "context"}
              revision={noteRevision}
              onOpenNote={(id) => void openNote(id)}
              onCreateNote={createNoteForArticle}
              onOpenGraph={openGraphForNote}
            />
          </section>
        ) : (
          <div className="context-tab" onClick={() => setContextOpen(true)} title="Context (C)">ctx</div>
        )}
      </main>

      <StatusBar
        context={uiCtx}
        modeLabel={modeLabel}
        filter={filterLabel}
        count={countBuf ? parseInt(countBuf, 10) : undefined}
        rangeStatus={rangeStatus}
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
      {conceptPickerOpen && (
        <ConceptPicker
          onOpenNote={(id) => { setConceptPickerOpen(false); void openNote(id); }}
          onClose={() => setConceptPickerOpen(false)}
          notify={notify}
        />
      )}
      {rulesPickerOpen && (
        <RulesPicker
          onAdd={() => { setRulesPickerOpen(false); setRuleForm({ initial: null }); }}
          onEdit={(r) => { setRulesPickerOpen(false); setRuleForm({ initial: r }); }}
          onClose={() => setRulesPickerOpen(false)}
          notify={notify}
        />
      )}
      {synthPrompt && (
        <PromptModal
          title="Category to synthesize"
          placeholder="models, research, industry…"
          onSubmit={(cat) => { setSynthPrompt(false); void runCmd(async () => { const k = await api.ksynthesize(cat); notify(`Molecule: ${k.moleculesCreated}`); }); }}
          onClose={() => setSynthPrompt(false)}
        />
      )}
      {urlPrompt && (
        <PromptModal
          title="Add article by URL"
          placeholder="https://example.com/blog/post"
          onSubmit={(url) => { setUrlPrompt(false); void addByURL(url); }}
          onClose={() => setUrlPrompt(false)}
        />
      )}
      {jobsOpen && <JobsPanel onClose={() => setJobsOpen(false)} notify={notify} />}
      {categoryPickerOpen && (
        <CategoryPicker
          current={filterCategory}
          unclassified={filterUnclassified}
          onPick={(c) => setFilterCategory(c)}
          onUnclassified={(v) => setFilterUnclassified(v)}
          onClose={() => setCategoryPickerOpen(false)}
        />
      )}
      {flowOpen && <FlowPanel onClose={() => setFlowOpen(false)} />}
      {logsOpen && <LogsPanel onClose={() => setLogsOpen(false)} />}
      {statusOpen && <StatusModal onClose={() => setStatusOpen(false)} />}
      {noteLinks && (
        <LinksPicker
          links={noteLinks}
          onPick={(item) => {
            setNoteLinks(null);
            if (item.url) void api.openURL(item.url);
            else void openNote(item.id);
          }}
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
      {ruleForm && (
        <RuleForm initial={ruleForm.initial} onSave={saveRule} onCancel={() => setRuleForm(null)} />
      )}
      {deleteSource && (
        <div className="modal-overlay" onClick={() => setDeleteSource(null)}>
          <div className="modal" onClick={(e) => e.stopPropagation()}>
            <div className="modal-head"><h2>Delete source</h2></div>
            <div className="modal-body">
              <p style={{ margin: 0 }}>
                Delete <strong>{deleteSource.name}</strong> from the list?
                <br /><span className="muted">Saved articles are kept. Reversible: you can re-add it.</span>
              </p>
            </div>
            <div className="modal-foot">
              <button onClick={() => setDeleteSource(null)}>Cancel</button>
              <button onClick={() => void confirmDeleteSource()} style={{ background: "var(--accent-dim)", color: "#fff" }}>Delete</button>
            </div>
          </div>
        </div>
      )}
      {toast && (
        <div className="toast">
          <span>{toast.msg}</span>
          {toast.undo && <button className="toast-undo" onClick={() => { toast.undo?.(); setToast(null); }}>Undo</button>}
        </div>
      )}
    </div>
  );
}
