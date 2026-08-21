import { useEffect, useState } from "react";
import { Sparkles, ExternalLink, Archive, Star, Loader2 } from "lucide-react";
import type { ArticleDTO } from "../types";
import { Markdown, stars, catClass } from "./Markdown";

// MIN_BODY is the length below which a body is almost certainly a feed summary,
// not the extracted article. Matches the extract package's MinLength.
const MIN_BODY = 200;

// formatDate renders a timestamp as the readable date of the article.
function formatDate(iso?: string): string {
  if (!iso) return "";
  const d = new Date(iso);
  if (isNaN(d.getTime())) return "";
  return d.toLocaleDateString("es-ES", { day: "numeric", month: "short", year: "numeric" });
}

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
        <span>Navigate with <kbd>j</kbd>/<kbd>k</kbd> — the article loads automatically</span>
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
          {(article.published || article.fetchedAt) && (
            <span className="time" title={article.published || article.fetchedAt}>{formatDate(article.published || article.fetchedAt)}</span>
          )}
          <span title={`Importance: ${article.importance}/3`}>{stars(article.importance)}</span>
          {article.category && <span className={`cat-chip ${catClass(article.category)}`}>{article.category}</span>}
          {(article.tags?.length ?? 0) > 0 && (
            <span className="tags">{(article.tags ?? []).map((t) => <span key={t} className="tag">#{t}</span>)}</span>
          )}
        </div>
        <div className="reader-actions">
          <button onClick={onSummarize} disabled={summarizing || !llmAvailable} title={llmAvailable ? "AI summary (y)" : "LLM unavailable"}>
            {summarizing ? <Loader2 size={16} className="spin" /> : <Sparkles size={16} />}
          </button>
          <button
            className={article.status === "archived" ? "active" : ""}
            onClick={onArchive}
            title={article.status === "archived" ? "Unarchive (a)" : "Archive (a) — reversible"}
          >
            <Archive size={16} fill={article.status === "archived" ? "currentColor" : "none"} />
          </button>
          <button
            className={article.starred === true ? "active" : ""}
            onClick={onStar}
            title={article.starred === true ? "Unstar (m)" : "Star (m)"}
          >
            <Star size={16} fill={article.starred === true ? "currentColor" : "none"} />
          </button>
          <button onClick={onOpenLink} title="Open in browser (O)">
            <ExternalLink size={16} />
          </button>
        </div>
      </div>

      {article.summary && (
        <div className="ai-summary">
          <span className="ai-label"><Sparkles size={13} /> AI summary</span>
          {article.summary}
        </div>
      )}

      <div className="reader-scroll" onScroll={onScroll}>
        {contentLoading && !article.contentMD ? (
          <div className="empty with-icon">
            <Loader2 size={24} className="spin" /> Extracting the article…
          </div>
        ) : article.contentMD ? (
          <>
            <Markdown content={article.contentMD} />
            {/* A short body is a feed summary, not the article. Some sites (e.g.
                OpenAI) sit behind Cloudflare + JS, so a plain fetch can never get
                the real text — point the reader at the source instead. */}
            {!contentLoading && article.contentMD.length < MIN_BODY && (
              <div className="empty with-icon" style={{ marginTop: 10 }}>
                <ExternalLink size={22} />
                <span>This looks like a summary, not the full article (the page may not be extractable).</span>
                <button onClick={onOpenLink}>Open in browser (O)</button>
              </div>
            )}
          </>
        ) : (
          <div className="empty with-icon">
            <ExternalLink size={26} />
            <span>Could not extract content from this source (may be JavaScript or blocked).</span>
            <button onClick={onOpenLink}>Open in browser (O)</button>
          </div>
        )}
      </div>
    </div>
  );
}
