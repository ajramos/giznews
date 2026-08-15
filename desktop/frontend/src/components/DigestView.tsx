import { type ReactNode, useEffect, useRef } from "react";
import { Newspaper, Loader2, Boxes, Bot, FlaskConical, Building2, Coins, Scale, Wrench, Code2, MessageSquare, FileText } from "lucide-react";
import type { DigestDTO, DigestMeta } from "../types";
import { stars } from "./Markdown";

interface Props {
  digest: DigestDTO | null;
  loading: boolean;
  unreadCount: number;
  focusId: number | null;
  history: DigestMeta[];
  selectedDate: string | null;
  onGenerate: () => void;
  onSelectDate: (date: string | null) => void;
  onFocus: (id: number | null) => void;
  onOpenArticle: (id: number) => void;
}

const CAT_ICONS: Record<string, ReactNode> = {
  models: <Bot size={15} />,
  research: <FlaskConical size={15} />,
  industry: <Building2 size={15} />,
  funding: <Coins size={15} />,
  regulation: <Scale size={15} />,
  tools: <Wrench size={15} />,
  "open-source": <Code2 size={15} />,
  opinion: <MessageSquare size={15} />,
  general: <FileText size={15} />,
};

export function DigestView({ digest, loading, unreadCount, focusId, history, selectedDate, onGenerate, onSelectDate, onFocus, onOpenArticle }: Props) {
  const focusRef = useRef<HTMLLIElement>(null);
  useEffect(() => {
    focusRef.current?.scrollIntoView({ block: "nearest" });
  }, [focusId]);

  return (
    <div className="digest-view">
      <div className="digest-head">
        <h1><Newspaper size={19} /> Digest</h1>
        {digest && <span className="digest-date-pill">{digest.date}</span>}
        <select value={selectedDate ?? ""} onChange={(e) => onSelectDate(e.target.value || null)} title="Saved digests">
          <option value="">Today (live)</option>
          {history.map((h) => <option key={h.date} value={h.date}>{h.date}</option>)}
        </select>
        <span className="pill">{unreadCount} unread</span>
        <button onClick={onGenerate} disabled={loading}>
          {loading ? <Loader2 size={13} className="spin" /> : <Newspaper size={13} />}
          {loading ? "Generating…" : "Generate today (d)"}
        </button>
      </div>

      {!digest && !loading && (
        <div className="empty with-icon">
          <Newspaper size={32} />
          <span>Press to generate the daily digest with AI.</span>
          <button onClick={onGenerate}>Generate digest</button>
        </div>
      )}
      {loading && <div className="empty with-icon"><Loader2 size={28} className="spin" /> Summarizing with AI…</div>}

      {digest && (
        <>
          {digest.themes.length > 0 && digest.themes.every((t) => t.theme === "general") && (
            <div className="digest-warn">
              These articles are not classified yet. Run <code>:process</code> to categorize them and group the digest by theme.
            </div>
          )}
          {digest.overview && <div className="digest-overview">{digest.overview}</div>}
          {digest.themes.length === 0 && <div className="empty">No themes yet. Run <code>:fetch</code> + <code>:classify</code>.</div>}
          {digest.themes.map((th) => (
            <section key={th.theme} className="digest-theme">
              <div className="digest-theme-header">
                <h2>{CAT_ICONS[th.theme] ?? <Boxes size={15} />} {th.theme}</h2>
                <span className="theme-count">{th.articles.length}</span>
              </div>
              {th.summary && <p className="theme-summary">{th.summary}</p>}
              <ul className="digest-articles">
                {th.articles.map((a) => (
                  <li
                    key={a.id}
                    ref={a.id === focusId ? focusRef : undefined}
                    className={a.id === focusId ? "selected" : ""}
                    onClick={() => onOpenArticle(a.id)}
                    onMouseEnter={() => onFocus(a.id)}
                  >
                    <span className="imp" data-level={a.importance}>{stars(a.importance)}</span>
                    <span className="dart-body">
                      <span className="dart-title">{a.title}</span>
                      {a.summary && <span className="dart-why">{a.summary}</span>}
                    </span>
                    <span className="muted">— {a.sourceName}</span>
                  </li>
                ))}
              </ul>
            </section>
          ))}
        </>
      )}
    </div>
  );
}
