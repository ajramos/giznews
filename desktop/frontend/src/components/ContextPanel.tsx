import { useEffect, useMemo, useState } from "react";
import { FileText, GitBranch, FlaskConical, Plus, Link2, GitFork, Boxes } from "lucide-react";
import { api } from "../api";
import type { ArticleDTO, NoteDTO } from "../types";
import { stars, catClass } from "./Markdown";

interface Props {
  article: ArticleDTO | null;
  note: NoteDTO | null;
  onOpenNote: (id: number) => void;
  onCreateNote: (articleId: number) => void;
  onOpenGraph: (noteId: number | null) => void;
}

const TYPE_ICON: Record<string, typeof FileText> = {
  atom: FileText,
  electron: GitBranch,
  molecule: FlaskConical,
};

// ContextPanel is the third pane: the bridge between an item and the knowledge
// graph. For an article it shows its note + related notes; for a note it shows
// its connections and tags.
export function ContextPanel({ article, note, onOpenNote, onCreateNote, onOpenGraph }: Props) {
  const [notes, setNotes] = useState<NoteDTO[]>([]);
  const [articleNote, setArticleNote] = useState<NoteDTO | null>(null);

  useEffect(() => {
    api.listNotes("").then(setNotes).catch(() => {});
  }, []);

  useEffect(() => {
    if (article) api.getArticleNote(article.id).then((n) => setArticleNote(n ?? null)).catch(() => setArticleNote(null));
    else setArticleNote(null);
  }, [article]);

  const bySlug = useMemo(() => new Map(notes.map((n) => [n.slug, n])), [notes]);

  const related = useMemo(() => {
    if (!article) return [];
    return notes.filter((n) => {
      if (n.type !== "atom" || !n.tags?.length) return false;
      return n.tags.some((t) => (article.tags ?? []).includes(t));
    });
  }, [article, notes]);

  const outgoing = useMemo(() => {
    if (!note) return [];
    return (note.wikilinks ?? []).map((s) => bySlug.get(s)).filter((n): n is NoteDTO => !!n);
  }, [note, bySlug]);

  const incoming = useMemo(() => {
    if (!note) return [];
    return notes.filter((n) => (n.wikilinks ?? []).includes(note.slug));
  }, [note, notes]);

  if (article) {
    return (
      <div className="ctx-pane">
        <div className="ctx-head"><Boxes size={14} /> Context</div>
        <div className="ctx-body">
          <div className="ctx-row">
            {article.category && <span className={`cat-chip ${catClass(article.category)}`}>{article.category}</span>}
            <span className="ctx-imp">{stars(article.importance)}</span>
            {article.sourceName && <span className="ctx-src">{article.sourceName}</span>}
          </div>
          {(article.tags?.length ?? 0) > 0 && (
            <div className="ctx-tags">{(article.tags ?? []).map((t) => <span key={t} className="tag">#{t}</span>)}</div>
          )}

          <div className="ctx-section">
            <div className="ctx-label"><GitFork size={12} /> Knowledge</div>
            {articleNote ? (
              <button className="ctx-item" onClick={() => onOpenNote(articleNote.id)}>
                <FileText size={13} />
                <span className="ctx-item-title">{articleNote.title}</span>
                <span className="ctx-item-hint">open</span>
              </button>
            ) : (
              <button className="ctx-item" onClick={() => onCreateNote(article.id)}>
                <Plus size={13} />
                <span className="ctx-item-title">Create note</span>
              </button>
            )}
          </div>

          {related.length > 0 && (
            <div className="ctx-section">
              <div className="ctx-label"><Link2 size={12} /> Related notes</div>
              {related.map((n) => (
                <button key={n.id} className="ctx-item" onClick={() => onOpenNote(n.id)}>
                  {(() => { const I = TYPE_ICON[n.type] ?? FileText; return <I size={13} />; })()}
                  <span className="ctx-item-title">{n.title}</span>
                </button>
              ))}
            </div>
          )}
        </div>
      </div>
    );
  }

  if (note) {
    return (
      <div className="ctx-pane">
        <div className="ctx-head"><Boxes size={14} /> Context</div>
        <div className="ctx-body">
          <div className="ctx-row">
            <span className={`note-type ${note.type}`}>{note.type}</span>
          </div>
          {(note.tags?.length ?? 0) > 0 && (
            <div className="ctx-tags">{(note.tags ?? []).map((t) => <span key={t} className="tag">#{t}</span>)}</div>
          )}

          <div className="ctx-section">
            <div className="ctx-label"><Link2 size={12} /> Links out</div>
            {outgoing.length === 0 && <div className="ctx-empty">No outgoing links.</div>}
            {outgoing.map((n) => (
              <button key={n.id} className="ctx-item" onClick={() => onOpenNote(n.id)}>
                {(() => { const I = TYPE_ICON[n.type] ?? FileText; return <I size={13} />; })()}
                <span className="ctx-item-title">{n.title}</span>
              </button>
            ))}
          </div>

          <div className="ctx-section">
            <div className="ctx-label">Backlinks</div>
            {incoming.length === 0 && <div className="ctx-empty">No backlinks.</div>}
            {incoming.map((n) => (
              <button key={n.id} className="ctx-item" onClick={() => onOpenNote(n.id)}>
                {(() => { const I = TYPE_ICON[n.type] ?? FileText; return <I size={13} />; })()}
                <span className="ctx-item-title">{n.title}</span>
              </button>
            ))}
          </div>

          <button className="ctx-graph" onClick={() => onOpenGraph(note.id)}>
            <GitBranch size={13} /> Open in graph
          </button>
        </div>
      </div>
    );
  }

  return (
    <div className="ctx-pane">
      <div className="ctx-head"><Boxes size={14} /> Context</div>
      <div className="ctx-body"><div className="ctx-empty">Open an article or note to see its connections.</div></div>
    </div>
  );
}
