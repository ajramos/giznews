import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { api } from "./api";
import type {
  ArticleDTO,
  DigestDTO,
  ListArticlesOptions,
  NoteDTO,
  SearchResultDTO,
  SourceDTO,
  StatusDTO,
  ViewMode,
  PanelMode,
} from "./types";
import { SourceList } from "./components/SourceList";
import { ArticleList } from "./components/ArticleList";
import { Reader } from "./components/Reader";
import { DigestView } from "./components/DigestView";
import { SearchPanel } from "./components/SearchPanel";
import { GraphPanel } from "./components/GraphPanel";
import { CommandPalette, type PaletteCommand } from "./components/CommandPalette";
import { HelpOverlay } from "./components/HelpOverlay";
import { Markdown } from "./components/Markdown";

export default function App() {
  const [sources, setSources] = useState<SourceDTO[]>([]);
  const [articles, setArticles] = useState<ArticleDTO[]>([]);
  const [selectedIndex, setSelectedIndex] = useState(0);
  const [view, setView] = useState<ViewMode>("articles");
  const [panel, setPanel] = useState<PanelMode>("none");
  const [paletteOpen, setPaletteOpen] = useState(false);
  const [helpOpen, setHelpOpen] = useState(false);

  const [reader, setReader] = useState<ArticleDTO | null>(null);
  const [noteReader, setNoteReader] = useState<NoteDTO | null>(null);
  const [summarizing, setSummarizing] = useState(false);

  const [digest, setDigest] = useState<DigestDTO | null>(null);
  const [digestLoading, setDigestLoading] = useState(false);

  const [searchQuery, setSearchQuery] = useState("");
  const [searchResults, setSearchResults] = useState<SearchResultDTO[]>([]);
  const [searching, setSearching] = useState(false);

  const [graphNote, setGraphNote] = useState<NoteDTO | null>(null);
  const [graphNeighbors, setGraphNeighbors] = useState<NoteDTO[]>([]);
  const [graphLoading, setGraphLoading] = useState(false);

  const [status, setStatus] = useState<StatusDTO | null>(null);
  const [toast, setToast] = useState<string | null>(null);
  const lastGRef = useRef(0);
  const busyRef = useRef(false);

  const notify = useCallback((msg: string) => {
    setToast(msg);
    window.setTimeout(() => setToast(null), 3500);
  }, []);

  const loadStatus = useCallback(async () => {
    try {
      setStatus(await api.status());
    } catch {
      /* backend still starting */
    }
  }, []);

  const loadSources = useCallback(async () => {
    try {
      setSources(await api.listSources());
    } catch (e) {
      notify(String(e));
    }
  }, [notify]);

  const loadArticles = useCallback(async (opts: ListArticlesOptions = {}) => {
    try {
      const list = await api.listArticles({ status: "unread", limit: 300, ...opts });
      setArticles(list);
      setSelectedIndex((i) => Math.min(i, Math.max(0, list.length - 1)));
    } catch (e) {
      notify(String(e));
    }
  }, [notify]);

  useEffect(() => {
    void loadSources();
    void loadArticles();
    void loadStatus();
  }, [loadSources, loadArticles, loadStatus]);

  const selectedArticle = articles[selectedIndex] ?? null;

  const openArticle = useCallback(
    async (id: number) => {
      try {
        const full = await api.getArticle(id);
        setReader(full);
        setNoteReader(null);
        if (full.status === "unread") {
          await api.setArticleStatus(id, "read");
          setArticles((prev) =>
            prev.map((a) => (a.id === id ? { ...a, status: "read" } : a))
          );
          void loadStatus();
        }
      } catch (e) {
        notify(String(e));
      }
    },
    [notify, loadStatus]
  );

  const openNote = useCallback(
    async (id: number) => {
      try {
        const n = await api.getNote(id);
        setNoteReader(n);
        setReader(null);
      } catch (e) {
        notify(String(e));
      }
    },
    [notify]
  );

  const openSelected = useCallback(async () => {
    if (selectedArticle) await openArticle(selectedArticle.id);
  }, [selectedArticle, openArticle]);

  const summarize = useCallback(async () => {
    if (!selectedArticle || busyRef.current) return;
    busyRef.current = true;
    setSummarizing(true);
    try {
      const updated = await api.summarizeArticle(selectedArticle.id);
      setReader(updated);
      setArticles((prev) => prev.map((a) => (a.id === updated.id ? { ...a, summary: updated.summary } : a)));
      notify("Resumen generado");
    } catch (e) {
      notify(String(e));
    } finally {
      busyRef.current = false;
      setSummarizing(false);
    }
  }, [selectedArticle, notify]);

  const archive = useCallback(async () => {
    if (!selectedArticle) return;
    await api.setArticleStatus(selectedArticle.id, "archived");
    setArticles((prev) => prev.filter((a) => a.id !== selectedArticle.id));
    setSelectedIndex((i) => Math.max(0, i - 1));
    void loadStatus();
  }, [selectedArticle, loadStatus]);

  const toggleRead = useCallback(async () => {
    if (!selectedArticle) return;
    const next = selectedArticle.status === "read" ? "unread" : "read";
    await api.setArticleStatus(selectedArticle.id, next);
    setArticles((prev) => prev.map((a) => (a.id === selectedArticle.id ? { ...a, status: next } : a)));
  }, [selectedArticle]);

  const generateDigest = useCallback(async () => {
    if (busyRef.current) return;
    busyRef.current = true;
    setDigestLoading(true);
    try {
      const d = await api.digest();
      setDigest(d);
      setView("digest");
    } catch (e) {
      notify(String(e));
    } finally {
      busyRef.current = false;
      setDigestLoading(false);
    }
  }, [notify]);

  const openGraph = useCallback(async () => {
    setPanel("graph");
    setGraphLoading(true);
    setGraphNote(null);
    setGraphNeighbors([]);
    try {
      const notes = await api.listNotes("atom");
      const match = selectedArticle
        ? notes.find((n) => n.title.toLowerCase() === selectedArticle.title.toLowerCase())
        : null;
      if (match) {
        setGraphNote(match);
        setGraphNeighbors(await api.graphNeighbors(match.id));
      }
    } catch (e) {
      notify(String(e));
    } finally {
      setGraphLoading(false);
    }
  }, [selectedArticle, notify]);

  const runSearch = useCallback(async (q: string) => {
    setSearching(true);
    try {
      if (!q.trim()) {
        setSearchResults([]);
      } else {
        setSearchResults(await api.search(q, 20));
      }
    } catch (e) {
      notify(String(e));
    } finally {
      setSearching(false);
    }
  }, [notify]);

  const commands = useMemo<PaletteCommand[]>(() => {
    const run = async (fn: () => Promise<unknown>, msg: string) => {
      try {
        await fn();
        notify(msg);
      } catch (e) {
        notify(String(e));
      }
    };
    return [
      {
        name: "fetch",
        hint: "Traer nuevos artículos",
        run: () => void run(async () => {
          const r = await api.fetch();
          await loadArticles();
          notify(`${r.newArticles} nuevos, ${r.sourcesFailed} fuentes con error`);
        }, "fetch"),
      },
      {
        name: "classify",
        hint: "Clasificar artículos (reglas + LLM)",
        run: () => void run(api.classify.bind(null, 100), "clasificación completa"),
      },
      {
        name: "kb build",
        hint: "Construir knowledge graph (atoms/electrons)",
        run: () => void run(api.kbuild, "knowledge graph actualizado"),
      },
      {
        name: "kb synth <cat>",
        hint: "Sintetizar molecule de una categoría",
        run: () => {
          const cat = window.prompt("Categoría (models, research, industry…)");
          if (cat) void run(() => api.ksynthesize(cat), `molecule de ${cat}`);
        },
      },
      {
        name: "search index",
        hint: "Indexar embeddings semánticos",
        run: () => void run(api.searchIndex, "índice actualizado"),
      },
      {
        name: "digest",
        hint: "Generar digest diario",
        run: () => void generateDigest(),
      },
      {
        name: "open vault",
        hint: "Abrir vault en Obsidian",
        run: () => {
          if (window.go && window.go.main && window.go.main.App && (window.go.main.App as Record<string, unknown>)["OpenVault"]) {
            void (window.go.main.App as unknown as { OpenVault: () => Promise<void> }).OpenVault();
          }
        },
      },
      {
        name: "quit",
        hint: "Salir de la aplicación",
        run: () => {
          if (window.go && window.go.main && window.go.main.App && (window.go.main.App as Record<string, unknown>)["Quit"]) {
            void (window.go.main.App as unknown as { Quit: () => Promise<void> }).Quit();
          }
        },
      },
    ];
  }, [loadArticles, notify, generateDigest]);

  // ---- keyboard ----
  useEffect(() => {
    const handler = (e: KeyboardEvent) => {
      const target = e.target as HTMLElement;
      const typing = target && (target.tagName === "INPUT" || target.tagName === "TEXTAREA" || target.isContentEditable);
      if (typing) return;

      if (paletteOpen || helpOpen) return;

      const k = e.key;
      const now = Date.now();

      if (k === "Escape") {
        if (panel !== "none") setPanel("none");
        else if (view !== "articles") setView("articles");
        return;
      }
      if (k === "j" || k === "ArrowDown") {
        e.preventDefault();
        setSelectedIndex((i) => Math.min(i + 1, Math.max(0, articles.length - 1)));
        return;
      }
      if (k === "k" || k === "ArrowUp") {
        e.preventDefault();
        setSelectedIndex((i) => Math.max(i - 1, 0));
        return;
      }
      if (k === "Enter") {
        void openSelected();
        return;
      }
      if (k === "g") {
        if (now - lastGRef.current < 400) {
          lastGRef.current = 0;
          void openGraph();
        } else {
          lastGRef.current = now;
          setSelectedIndex(0);
        }
        return;
      }
      if (k === "G") {
        setSelectedIndex(Math.max(0, articles.length - 1));
        return;
      }
      if (k === "s") { setPanel((p) => (p === "search" ? "none" : "search")); return; }
      if (k === "y") { void summarize(); return; }
      if (k === "a") { void archive(); return; }
      if (k === "t") { void toggleRead(); return; }
      if (k === "d") { void generateDigest(); return; }
      if (k === "1") { setView("articles"); return; }
      if (k === "2") { setView("digest"); return; }
      if (k === ":") { setPaletteOpen(true); return; }
      if (k === "?") { setHelpOpen(true); return; }
    };
    window.addEventListener("keydown", handler);
    return () => window.removeEventListener("keydown", handler);
  }, [paletteOpen, helpOpen, panel, view, articles.length, openSelected, openGraph, summarize, archive, toggleRead, generateDigest]);

  const openExternal = useCallback(() => {
    if (!reader?.url) return;
    window.open(reader.url, "_blank");
  }, [reader]);

  const filteredArticles = useMemo(() => {
    if (panel === "search") return articles;
    return articles;
  }, [articles, panel]);

  return (
    <div className="app">
      <header className="topbar">
        <div className="brand">GizNews</div>
        {status && (
          <div className="status">
            <span className="pill">{status.unreadArticles} no leídos</span>
            <span className={`pill llm ${status.llmEnabled ? "on" : "off"}`}>{status.llmProvider}</span>
            <span className="pill">🧠 {status.totalNotes} notas</span>
          </div>
        )}
        <div className="topbar-actions">
          <button onClick={() => setView(view === "articles" ? "digest" : "articles")}>
            {view === "articles" ? "Digest (2)" : "Artículos (1)"}
          </button>
          <button onClick={() => setHelpOpen(true)} title="Ayuda">?</button>
        </div>
      </header>

      <main className="layout">
        <aside className="col sources-col">
          <SourceList
            sources={sources}
            activeId={null}
            onSelect={() => {}}
            onToggle={async (id, enabled) => {
              await api.setSourceEnabled(id, enabled);
              await loadSources();
            }}
          />
        </aside>

        <section className="col list-col">
          {view === "articles" ? (
            <ArticleList articles={filteredArticles} selectedIndex={selectedIndex} onSelect={setSelectedIndex} />
          ) : (
            <DigestView digest={digest} loading={digestLoading} onGenerate={generateDigest} onOpenArticle={(id) => {
              setView("articles");
              const idx = articles.findIndex((a) => a.id === id);
              if (idx >= 0) setSelectedIndex(idx);
              void openArticle(id);
            }} />
          )}
        </section>

        <section className="col reader-col">
          {panel === "search" ? (
            <SearchPanel
              query={searchQuery}
              onQuery={(q) => { setSearchQuery(q); void runSearch(q); }}
              searching={searching}
              results={searchResults}
              onOpen={(r) => {
                setPanel("none");
                if (r.kind === "article") {
                  const idx = articles.findIndex((a) => a.id === r.id);
                  if (idx >= 0) { setSelectedIndex(idx); void openArticle(r.id); }
                  else void openArticle(r.id);
                } else {
                  void openNote(r.id);
                }
              }}
            />
          ) : panel === "graph" ? (
            <GraphPanel note={graphNote} neighbors={graphNeighbors} loading={graphLoading} onOpenNote={openNote} />
          ) : noteReader ? (
            <div className="reader">
              <div className="reader-head">
                <h1>{noteReader.title}</h1>
                <button onClick={() => setNoteReader(null)}>Cerrar (Esc)</button>
              </div>
              <div className="reader-scroll">
                <Markdown content={noteReader.content} />
              </div>
            </div>
          ) : (
            <Reader article={reader} summarizing={summarizing} onSummarize={summarize} onOpenLink={openExternal} />
          )}
        </section>
      </main>

      {paletteOpen && <CommandPalette commands={commands} onClose={() => setPaletteOpen(false)} />}
      {helpOpen && <HelpOverlay onClose={() => setHelpOpen(false)} />}
      {toast && <div className="toast">{toast}</div>}
    </div>
  );
}
