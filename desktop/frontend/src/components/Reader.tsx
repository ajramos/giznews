import { useEffect } from "react";
import { Sparkles, ExternalLink, Archive, Star, Loader2 } from "lucide-react";
import type { ArticleDTO } from "../types";
import { Markdown, stars, catClass } from "./Markdown";

interface Props {
  article: ArticleDTO | null;
  summarizing: boolean;
  onSummarize: () => void;
  onArchive: () => void;
  onStar: () => void;
  onOpenLink: () => void;
}

export function Reader({ article, summarizing, onSummarize, onArchive, onStar, onOpenLink }: Props) {
  useEffect(() => {
    document.querySelector(".reader-scroll")?.scrollTo({ top: 0 });
  }, [article?.id]);

  if (!article) {
    return (
      <div className="empty with-icon reader-hint">
        <Sparkles size={34} />
        <span>Selecciona un artículo con <kbd>j</kbd>/<kbd>k</kbd> y pulsa <kbd>Enter</kbd></span>
      </div>
    );
  }

  return (
    <div className="reader">
      <div className="reader-head">
        <h1>{article.title}</h1>
        <div className="reader-meta">
          {article.sourceName && <span className="src">{article.sourceName}</span>}
          {article.author && <span>{article.author}</span>}
          <span title={`Importancia: ${article.importance}/3`}>{stars(article.importance)}</span>
          {article.category && <span className={`cat-chip ${catClass(article.category)}`}>{article.category}</span>}
          {article.tags.length > 0 && (
            <span className="tags">{article.tags.map((t) => <span key={t} className="tag">#{t}</span>)}</span>
          )}
        </div>
        <div className="reader-actions">
          <button onClick={onSummarize} disabled={summarizing}>
            {summarizing ? <Loader2 size={13} className="spin" /> : <Sparkles size={13} />}
            {summarizing ? "Resumiendo…" : "Resumen (y)"}
          </button>
          <button onClick={onArchive} title="Archivar (a) — reversible">
            <Archive size={13} /> Archivar
          </button>
          <button onClick={onStar} title="Destacar (m)">
            <Star size={13} /> Destacar
          </button>
          <button onClick={onOpenLink} title="Abrir en el navegador (O)">
            <ExternalLink size={13} /> Abrir (O)
          </button>
        </div>
      </div>

      {article.summary && (
        <div className="ai-summary">
          <span className="ai-label"><Sparkles size={13} /> Resumen IA</span>
          {article.summary}
        </div>
      )}

      <div className="reader-scroll">
        <Markdown content={article.contentMD || ""} />
      </div>
    </div>
  );
}
