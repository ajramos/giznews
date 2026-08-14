import { useEffect, useState } from "react";
import { Sparkles, ExternalLink, Archive, Star, Loader2 } from "lucide-react";
import type { ArticleDTO } from "../types";
import { Markdown, stars, catClass } from "./Markdown";

interface Props {
  article: ArticleDTO | null;
  summarizing: boolean;
  contentLoading: boolean;
  llmAvailable: boolean;
  onSummarize: () => void;
  onArchive: () => void;
  onStar: () => void;
  onOpenLink: () => void;
}

export function Reader({ article, summarizing, contentLoading, llmAvailable, onSummarize, onArchive, onStar, onOpenLink }: Props) {
  const [progress, setProgress] = useState(0);

  useEffect(() => {
    const el = document.querySelector(".reader-scroll");
    if (el) el.scrollTo({ top: 0 });
    setProgress(0);
  }, [article?.id]);

  const onScroll = () => {
    const el = document.querySelector(".reader-scroll");
    if (!el) return;
    const max = el.scrollHeight - el.clientHeight;
    setProgress(max > 0 ? Math.min(1, el.scrollTop / max) : 0);
  };

  if (!article) {
    return (
      <div className="empty with-icon reader-hint">
        <Sparkles size={34} />
        <span>Navega con <kbd>j</kbd>/<kbd>k</kbd> — el artículo se carga automáticamente</span>
      </div>
    );
  }

  return (
    <div className="reader">
      <div className="read-progress"><span style={{ width: `${progress * 100}%` }} /></div>
      <div className="reader-head">
        <h1>{article.title}</h1>
        <div className="reader-meta">
          {article.sourceName && <span className="src">{article.sourceName}</span>}
          {article.author && <span>{article.author}</span>}
          <span title={`Importancia: ${article.importance}/3`}>{stars(article.importance)}</span>
          {article.category && <span className={`cat-chip ${catClass(article.category)}`}>{article.category}</span>}
          {(article.tags?.length ?? 0) > 0 && (
            <span className="tags">{(article.tags ?? []).map((t) => <span key={t} className="tag">#{t}</span>)}</span>
          )}
        </div>
        <div className="reader-actions">
          <button onClick={onSummarize} disabled={summarizing || !llmAvailable} title={llmAvailable ? "Resumen IA (y)" : "LLM no disponible (sin resumen)"}>
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

      <div className="reader-scroll" onScroll={onScroll}>
        {contentLoading && !article.contentMD ? (
          <div className="empty with-icon">
            <Loader2 size={24} className="spin" /> Extrayendo el artículo…
          </div>
        ) : article.contentMD ? (
          <Markdown content={article.contentMD} />
        ) : (
          <div className="empty with-icon">
            <ExternalLink size={26} />
            <span>No se pudo extraer el contenido de esta fuente (puede ser JavaScript o estar bloqueada).</span>
            <button onClick={onOpenLink}>Abrir en el navegador (O)</button>
          </div>
        )}
      </div>
    </div>
  );
}
