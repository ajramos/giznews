import { useEffect, useRef } from "react";
import type { ArticleDTO } from "../types";
import { stars, timeAgo } from "./Markdown";

interface Props {
  articles: ArticleDTO[];
  selectedIndex: number;
  onSelect: (index: number) => void;
}

export function ArticleList({ articles, selectedIndex, onSelect }: Props) {
  const containerRef = useRef<HTMLDivElement>(null);

  // Keep the selected row visible while navigating with j/k.
  useEffect(() => {
    const sel = containerRef.current?.querySelector(".article-row.selected");
    sel?.scrollIntoView({ block: "nearest" });
  }, [selectedIndex, articles]);

  if (articles.length === 0) {
    return <div className="empty">Sin artículos. Pulsa <code>:fetch</code> para traer novedades.</div>;
  }
  return (
    <div className="article-list" ref={containerRef}>
      {articles.map((a, i) => (
        <div
          key={a.id}
          className={`article-row ${i === selectedIndex ? "selected" : ""}`}
          onClick={() => onSelect(i)}
          onDoubleClick={() => onSelect(i)}
        >
          <span className="article-flags">
            <span className="imp" data-level={a.importance}>{stars(a.importance)}</span>
            {a.status === "unread" ? <span className="unread-dot">●</span> : null}
            {a.status === "archived" ? <span className="arch">🗄</span> : null}
          </span>
          <span className="article-main">
            <span className="article-title">{a.title}</span>
            <span className="article-meta">
              {a.sourceName} · {a.category || "sin categoría"} · {timeAgo(a.published || a.fetchedAt)}
            </span>
          </span>
        </div>
      ))}
    </div>
  );
}
