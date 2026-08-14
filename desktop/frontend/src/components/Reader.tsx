import { useEffect } from "react";
import type { ArticleDTO } from "../types";
import { Markdown, stars } from "./Markdown";

interface Props {
  article: ArticleDTO | null;
  summarizing: boolean;
  onSummarize: () => void;
  onOpenLink: () => void;
}

export function Reader({ article, summarizing, onSummarize, onOpenLink }: Props) {
  useEffect(() => {
    const el = document.querySelector(".reader-scroll");
    if (el) el.scrollTop = 0;
  }, [article?.id]);

  if (!article) {
    return <div className="empty reader-hint">Selecciona un artículo y pulsa Enter</div>;
  }

  return (
    <div className="reader">
      <div className="reader-head">
        <h1>{article.title}</h1>
        <div className="reader-meta">
          {article.sourceName} · {stars(article.importance)}
          {article.author ? ` · ${article.author}` : ""}
          {article.tags.length > 0 && (
            <span className="tags">
              {article.tags.map((t) => (
                <span key={t} className="tag">#{t}</span>
              ))}
            </span>
          )}
        </div>
        <div className="reader-actions">
          <button onClick={onSummarize} disabled={summarizing}>
            {summarizing ? "Resumiendo…" : "y · Resumen IA"}
          </button>
          <button onClick={onOpenLink} title="Abrir en navegador">Abrir ↗</button>
        </div>
      </div>

      {article.summary && (
        <div className="ai-summary">
          <strong>Resumen:</strong> {article.summary}
        </div>
      )}

      <div className="reader-scroll">
        <Markdown content={article.contentMD || ""} />
      </div>
    </div>
  );
}
