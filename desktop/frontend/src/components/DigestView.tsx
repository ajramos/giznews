import type { DigestDTO } from "../types";
import { stars } from "./Markdown";

interface Props {
  digest: DigestDTO | null;
  loading: boolean;
  onGenerate: () => void;
  onOpenArticle: (id: number) => void;
}

export function DigestView({ digest, loading, onGenerate, onOpenArticle }: Props) {
  return (
    <div className="digest-view">
      <div className="digest-head">
        <h1>Digest de IA</h1>
        {digest && <span className="muted">{digest.date}</span>}
        <button onClick={onGenerate} disabled={loading}>
          {loading ? "Generando…" : "d · Generar digest"}
        </button>
      </div>

      {!digest && !loading && <div className="empty">Pulsa para generar el digest diario.</div>}
      {loading && <div className="empty">Resumiendo con IA…</div>}

      {digest && (
        <>
          {digest.overview && <div className="digest-overview">{digest.overview}</div>}
          {digest.themes.map((th) => (
            <section key={th.theme} className="digest-theme">
              <h2>{th.theme}</h2>
              {th.summary && <p className="theme-summary">{th.summary}</p>}
              <ul className="digest-articles">
                {th.articles.map((a) => (
                  <li key={a.id} onClick={() => onOpenArticle(a.id)}>
                    <span className="imp" data-level={a.importance}>{stars(a.importance)}</span>
                    <span>{a.title}</span>
                    <span className="muted"> — {a.sourceName}</span>
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
