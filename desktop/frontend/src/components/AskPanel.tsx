import { Loader2, Brain, FileText, Sparkles, X } from "lucide-react";
import type { AnswerDTO, SearchResultDTO } from "../types";

interface Props {
  question: string;
  onQuestion: (q: string) => void;
  asking: boolean;
  answer: AnswerDTO | null;
  onAsk: () => void;
  onOpen: (r: SearchResultDTO) => void;
  onClose: () => void;
}

const CITATION = /\[\[([^\]|#]+)\]\]/g;

// AskPanel: the knowledge base answering in its own words, with every claim
// traceable back to the note it came from — a citation you cannot click is
// barely better than no citation, so they are all buttons.
export function AskPanel({ question, onQuestion, asking, answer, onAsk, onOpen, onClose }: Props) {
  const bySlug = new Map<string, SearchResultDTO>();
  for (const s of answer?.sources ?? []) if (s.slug) bySlug.set(s.slug, s);

  // Render the prose, turning [[slug]] into something that opens that note.
  const prose = (text: string) => {
    const out: React.ReactNode[] = [];
    let last = 0;
    for (const m of text.matchAll(CITATION)) {
      const at = m.index ?? 0;
      if (at > last) out.push(text.slice(last, at));
      const slug = m[1].trim();
      const source = bySlug.get(slug);
      out.push(
        source ? (
          <button key={`${at}-${slug}`} className="citation" onClick={() => onOpen(source)} title={source.title}>
            {slug}
          </button>
        ) : (
          <span key={`${at}-${slug}`} className="citation muted" title="Not among the notes read for this answer">
            {slug}
          </span>
        ),
      );
      last = at + m[0].length;
    }
    if (last < text.length) out.push(text.slice(last));
    return out;
  };

  return (
    <div className="panel ask-panel">
      <div className="panel-title"><Sparkles size={13} /> Ask your notes</div>
      <div className="search-box">
        <Brain size={15} />
        <input
          autoFocus
          value={question}
          onChange={(e) => onQuestion(e.target.value)}
          onKeyDown={(e) => {
            if (e.key === "Enter") { e.preventDefault(); onAsk(); }
            else if (e.key === "Escape") { e.preventDefault(); onClose(); }
          }}
          placeholder="what do I know about sparse attention?"
        />
        {question && (
          <button className="search-clear" title="Clear" onClick={() => onQuestion("")}>
            <X size={14} />
          </button>
        )}
      </div>

      {asking && <div className="empty with-icon"><Loader2 size={22} className="spin" /> Reading your notes…</div>}

      {!asking && answer && (
        <div className="results">
          {answer.grounded ? (
            <div className="ask-answer" data-testid="ask-answer">{prose(answer.text)}</div>
          ) : (
            <div className="ask-answer ungrounded" data-testid="ask-ungrounded">
              {answer.sources.length === 0
                ? "Nothing in your vault touches that."
                : "No answer written — this is what the vault has on it."}
            </div>
          )}
          {(answer.dropped?.length ?? 0) > 0 && (
            <div className="ask-dropped" data-testid="ask-dropped">
              Dropped {answer.dropped!.length} invented citation(s): {answer.dropped!.join(", ")}
            </div>
          )}
          {answer.sources.map((r) => (
            <div key={r.kind + r.id} className="result-row" onClick={() => onOpen(r)}>
              <span className="result-icon">{r.kind === "note" ? <Brain size={15} /> : <FileText size={15} />}</span>
              <div className="result-body">
                <div className="result-title">{r.title}</div>
                <div className="result-snippet">{r.snippet}</div>
              </div>
              <span className="kind-badge">{r.slug ? r.slug : r.kind}</span>
            </div>
          ))}
        </div>
      )}
    </div>
  );
}
