import { Search, Loader2, FileText, Brain, X } from "lucide-react";
import type { SearchResultDTO } from "../types";

interface Props {
  query: string;
  onQuery: (q: string) => void;
  searching: boolean;
  results: SearchResultDTO[];
  focus: number;
  onFocus: (i: number) => void;
  onOpen: (r: SearchResultDTO) => void;
  onClose: () => void;
}

export function SearchPanel({ query, onQuery, searching, results, focus, onFocus, onOpen, onClose }: Props) {
  return (
    <div className="panel search-panel">
      <div className="panel-title"><Search size={13} /> Búsqueda semántica</div>
      <div className="search-box">
        <Search size={15} />
        <input
          autoFocus
          value={query}
          onChange={(e) => onQuery(e.target.value)}
          onKeyDown={(e) => {
            if (e.key === "ArrowDown") { e.preventDefault(); onFocus(Math.min(focus + 1, results.length - 1)); }
            else if (e.key === "ArrowUp") { e.preventDefault(); onFocus(Math.max(focus - 1, 0)); }
            else if (e.key === "Enter") { if (results[focus]) onOpen(results[focus]); }
            else if (e.key === "Escape") onClose();
          }}
          placeholder="p. ej. watermarking, agentic workflows, LoRA…"
        />
        {query && (
          <button className="search-clear" title="Limpiar" onClick={() => onQuery("")}>
            <X size={14} />
          </button>
        )}
      </div>
      {searching && <div className="empty with-icon"><Loader2 size={22} className="spin" /> Buscando…</div>}
      {!searching && query && results.length === 0 && <div className="empty">Sin resultados.</div>}
      <div className="results">
        {results.map((r, i) => (
          <div
            key={r.kind + r.id}
            className={`result-row ${i === focus ? "selected" : ""}`}
            onMouseEnter={() => onFocus(i)}
            onClick={() => onOpen(r)}
          >
            <span className="result-icon">{r.kind === "note" ? <Brain size={15} /> : <FileText size={15} />}</span>
            <div className="result-body">
              <div className="result-title">{r.title}</div>
              <div className="result-snippet">{r.snippet}</div>
            </div>
            <span className="kind-badge">{r.kind}</span>
          </div>
        ))}
      </div>
    </div>
  );
}
