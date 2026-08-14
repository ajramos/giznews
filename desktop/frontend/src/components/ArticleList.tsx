import { useEffect, useRef, type ReactNode } from "react";
import { Inbox, Check, Archive, Star } from "lucide-react";
import type { ArticleDTO } from "../types";
import { stars, timeAgo, catClass } from "./Markdown";

export type ViewFilter = "unread" | "read" | "archived" | "starred";

const VIEWS: { key: ViewFilter; label: string; icon: ReactNode }[] = [
  { key: "unread", label: "No leídos", icon: <Inbox size={12} /> },
  { key: "read", label: "Leídos", icon: <Check size={12} /> },
  { key: "archived", label: "Archivados", icon: <Archive size={12} /> },
  { key: "starred", label: "Destacados", icon: <Star size={12} /> },
];

interface Props {
  articles: ArticleDTO[];
  selectedIndex: number;
  loading: boolean;
  view: ViewFilter;
  bulkRange: readonly [number, number] | null;
  onView: (v: ViewFilter) => void;
  onSelect: (index: number) => void;
}

export function ArticleList({ articles, selectedIndex, loading, view, bulkRange, onView, onSelect }: Props) {
  const containerRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    containerRef.current?.querySelector(".article-row.selected")?.scrollIntoView({ block: "nearest" });
  }, [selectedIndex, articles]);

  return (
    <div className="list-pane">
      <div className="list-head">
        <div className="view-tabs">
          {VIEWS.map((v) => (
            <button
              key={v.key}
              className={`view-tab ${view === v.key ? "active" : ""}`}
              onClick={() => onView(v.key)}
            >
              {v.icon} {v.label}
            </button>
          ))}
        </div>
      </div>

      {loading ? (
        <div className="skeleton">
          {[...Array(8)].map((_, i) => <div key={i} className="bar" style={{ width: `${70 + (i % 3) * 12}%` }} />)}
        </div>
      ) : articles.length === 0 ? (
        <div className="empty with-icon">
          <Inbox size={30} />
          <span>No hay artículos en esta vista.</span>
          <span className="muted">Prueba <kbd>u</kbd>/<kbd>r</kbd>/<kbd>x</kbd>/<kbd>*</kbd> o ejecuta <code>:fetch</code></span>
        </div>
      ) : (
        <>
          <div className="col-header">
            <span className="c-flag" title="Estado">●</span>
            <span className="c-imp" title="Importancia (0-3)">Imp.</span>
            <span className="c-title">Título</span>
            <span className="c-src">Fuente</span>
            <span className="c-time">Fecha</span>
          </div>
          <div className="article-list" ref={containerRef}>
            {articles.map((a, i) => {
              const inBulk = bulkRange ? i >= bulkRange[0] && i <= bulkRange[1] : false;
              return (
              <div
                key={a.id}
                className={`article-row ${i === selectedIndex ? "selected" : ""} ${inBulk ? "bulk" : ""} ${a.status === "read" ? "read" : ""} ${a.status === "archived" ? "archived" : ""}`}
                onClick={() => onSelect(i)}
                onDoubleClick={() => onSelect(i)}
              >
                <span className="article-flag" title={statusTitle(a.status)}>
                  {a.status === "unread" ? <span className="unread-badge" /> : a.status === "starred" ? <span className="star-badge">★</span> : null}
                </span>
                <span className="article-imp imp" data-level={a.importance} title={`Importancia: ${a.importance}/3`}>
                  {stars(a.importance)}
                </span>
                <span className="article-main">
                  <span className="article-title">{a.title}</span>
                  <span className="article-meta">
                    {a.category && <span className={`cat-chip ${catClass(a.category)}`}>{a.category}</span>}
                    <span className="src">{a.sourceName}</span>
                    <span className="time" title={a.published || a.fetchedAt}>{timeAgo(a.published || a.fetchedAt)}</span>
                  </span>
                </span>
              </div>
              );
            })}
          </div>
        </>
      )}
    </div>
  );
}

function statusTitle(s: string): string {
  switch (s) {
    case "unread": return "No leído";
    case "read": return "Leído";
    case "archived": return "Archivado (lógico, recuperable)";
    case "starred": return "Destacado";
    default: return s;
  }
}
