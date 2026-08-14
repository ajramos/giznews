import { useState } from "react";
import type { SearchResultDTO } from "../types";

interface Props {
  query: string;
  onQuery: (q: string) => void;
  searching: boolean;
  results: SearchResultDTO[];
  onOpen: (r: SearchResultDTO) => void;
}

export function SearchPanel({ query, onQuery, searching, results, onOpen }: Props) {
  const [focus, setFocus] = useState(0);

  return (
    <div className="panel search-panel">
      <div className="panel-title">s · Búsqueda semántica</div>
      <input
        autoFocus
        value={query}
        onChange={(e) => onQuery(e.target.value)}
        placeholder="p. ej. watermarking, agentic workflows, LoRA…"
      />
      {searching && <div className="empty">Buscando…</div>}
      {!searching && results.length === 0 && query && <div className="empty">Sin resultados</div>}
      <div className="results">
        {results.map((r, i) => (
          <div
            key={r.kind + r.id}
            className={`result-row ${i === focus ? "selected" : ""}`}
            onMouseEnter={() => setFocus(i)}
            onClick={() => onOpen(r)}
          >
            <span className="result-icon">{r.kind === "note" ? "🧠" : "📄"}</span>
            <div className="result-body">
              <div className="result-title">{r.title}</div>
              <div className="result-snippet">{r.snippet}</div>
            </div>
          </div>
        ))}
      </div>
    </div>
  );
}
