import { useEffect, useMemo, useState, type ReactNode } from "react";
import { FileText, GitBranch, FlaskConical, Plus, Link2, Boxes } from "lucide-react";
import { api } from "../api";
import type { ArticleDTO, NoteDTO } from "../types";
import { stars, catClass } from "./Markdown";

interface Props {
  article: ArticleDTO | null;
  note: NoteDTO | null;
  active: boolean;
  revision: number;
  onOpenNote: (id: number) => void;
  onCreateNote: (articleId: number) => void;
  onOpenGraph: (noteId: number | null) => void;
}

const TYPE_ICON: Record<string, typeof FileText> = {
  atom: FileText,
  electron: GitBranch,
  molecule: FlaskConical,
};

interface CtxAction {
  key: string;
  label: string;
  icon: ReactNode;
  section: string;
  run: () => void;
}

function noteIcon(n: NoteDTO): ReactNode {
  const I = TYPE_ICON[n.type] ?? FileText;
  return <I size={13} />;
}

// ContextPanel is the third pane: the bridge between an item and the knowledge
// graph. When focused (Tab), j/k navigate its actions and Enter runs them.
export function ContextPanel({ article, note, active, revision, onOpenNote, onCreateNote, onOpenGraph }: Props) {
  const [notes, setNotes] = useState<NoteDTO[]>([]);
  const [articleNote, setArticleNote] = useState<NoteDTO | null>(null);
  const [focus, setFocus] = useState(0);

  useEffect(() => {
    api.listNotes("").then(setNotes).catch(() => {});
  }, []);

  useEffect(() => {
    if (article) api.getArticleNote(article.id).then((n) => setArticleNote(n ?? null)).catch(() => setArticleNote(null));
    else setArticleNote(null);
  }, [article, revision]);

  const bySlug = useMemo(() => new Map(notes.map((n) => [n.slug, n])), [notes]);

  const related = useMemo(() => {
    if (!article) return [];
    return notes.filter((n) => n.type === "atom" && n.tags?.length && n.tags.some((t) => (article.tags ?? []).includes(t)));
  }, [article, notes]);

  const outgoing = useMemo(() => {
    if (!note) return [];
    return (note.wikilinks ?? []).map((s) => bySlug.get(s)).filter((n): n is NoteDTO => !!n);
  }, [note, bySlug]);

  const incoming = useMemo(() => {
    if (!note) return [];
    return notes.filter((n) => (n.wikilinks ?? []).includes(note.slug));
  }, [note, notes]);

  const actions: CtxAction[] = useMemo(() => {
    const out: CtxAction[] = [];
    if (article) {
      if (articleNote) {
        out.push({ key: "note", label: articleNote.title, icon: <FileText size={13} />, section: "Knowledge", run: () => onOpenNote(articleNote.id) });
      } else {
        out.push({ key: "create", label: "Create note", icon: <Plus size={13} />, section: "Knowledge", run: () => onCreateNote(article.id) });
      }
      for (const n of related) out.push({ key: `rel-${n.id}`, label: n.title, icon: noteIcon(n), section: "Related notes", run: () => onOpenNote(n.id) });
    } else if (note) {
      for (const n of outgoing) out.push({ key: `out-${n.id}`, label: n.title, icon: noteIcon(n), section: "Links out", run: () => onOpenNote(n.id) });
      for (const n of incoming) out.push({ key: `in-${n.id}`, label: n.title, icon: noteIcon(n), section: "Backlinks", run: () => onOpenNote(n.id) });
      out.push({ key: "graph", label: "Open in graph", icon: <GitBranch size={13} />, section: "Graph", run: () => onOpenGraph(note.id) });
    }
    return out;
  }, [article, note, articleNote, related, outgoing, incoming, onOpenNote, onCreateNote, onOpenGraph]);

  useEffect(() => { setFocus(0); }, [article?.id, note?.id]);

  useEffect(() => {
    if (!active) return;
    const onKey = (e: KeyboardEvent) => {
      const n = actions.length;
      if (e.key === "j" || e.key === "ArrowDown") { e.preventDefault(); e.stopPropagation(); setFocus((f) => Math.min(f + 1, n - 1)); }
      else if (e.key === "k" || e.key === "ArrowUp") { e.preventDefault(); e.stopPropagation(); setFocus((f) => Math.max(f - 1, 0)); }
      else if (e.key === "Enter") { e.preventDefault(); e.stopPropagation(); const a = actions[focus]; if (a) a.run(); }
    };
    window.addEventListener("keydown", onKey, true);
    return () => window.removeEventListener("keydown", onKey, true);
  }, [active, actions, focus]);

  const meta = article
    ? (
      <>
        <div className="ctx-row">
          {article.category && <span className={`cat-chip ${catClass(article.category)}`}>{article.category}</span>}
          <span className="ctx-imp">{stars(article.importance)}</span>
          {article.sourceName && <span className="ctx-src">{article.sourceName}</span>}
        </div>
        {(article.tags?.length ?? 0) > 0 && (
          <div className="ctx-tags">{(article.tags ?? []).map((t) => <span key={t} className="tag">#{t}</span>)}</div>
        )}
      </>
    )
    : note
    ? (
      <>
        <div className="ctx-row"><span className={`note-type ${note.type}`}>{note.type}</span></div>
        {(note.tags?.length ?? 0) > 0 && (
          <div className="ctx-tags">{(note.tags ?? []).map((t) => <span key={t} className="tag">#{t}</span>)}</div>
        )}
      </>
    )
    : null;

  return (
    <div className={`ctx-pane ${active ? "active" : ""}`}>
      <div className="ctx-head"><Boxes size={14} /> Context</div>
      <div className="ctx-body">
        {meta}
        {actions.length === 0 ? (
          <div className="ctx-empty">Open an article or note to see its connections.</div>
        ) : (
          actions.map((a, i) => {
            const showHeader = i === 0 || actions[i - 1].section !== a.section;
            return (
              <div key={a.key} style={{ display: "contents" }}>
                {showHeader && <div className="ctx-label"><Link2 size={12} /> {a.section}</div>}
                <button className={`ctx-item ${i === focus ? "focused" : ""}`} onClick={a.run} onMouseEnter={() => setFocus(i)}>
                  {a.icon}
                  <span className="ctx-item-title">{a.label}</span>
                </button>
              </div>
            );
          })
        )}
      </div>
    </div>
  );
}
